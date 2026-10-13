// Port of SNTP.html -- see client/sntp_config.go for the page's quirks
// (three Apply buttons, one per mode; IPv4 literal only). Singleton.
//
// In Disabled mode the switch keeps the previously set server/port but the
// form doesn't send them, so for Disabled the state carries the configured
// server/port (or null) instead of what the page still shows -- otherwise
// every plan would diff a leftover server against a null config.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const sntpDefaultPort = 123

var (
	_ resource.Resource                   = &sntpResource{}
	_ resource.ResourceWithConfigure      = &sntpResource{}
	_ resource.ResourceWithValidateConfig = &sntpResource{}
)

func NewSNTPResource() resource.Resource {
	return &sntpResource{}
}

type sntpResource struct {
	client *client.Client
}

type sntpResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Mode             types.String `tfsdk:"mode"`
	Server           types.String `tfsdk:"server"`
	Port             types.Int64  `tfsdk:"port"`
	TimeFormat       types.String `tfsdk:"time_format"`
	LastUpdateStatus types.String `tfsdk:"last_update_status"`
}

func (r *sntpResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sntp"
}

func (r *sntpResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Controls the switch's SNTP Configuration page (one unicast server). Singleton -- there is only ever one of these per switch. Delete sets SNTP to Disabled; the clock keeps running from its last sync.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always \"sntp\" -- this is a singleton resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mode": schema.StringAttribute{
				Required:    true,
				Description: "\"Unicast\" or \"Disabled\" (the only modes this firmware offers).",
			},
			"server": schema.StringAttribute{
				Optional:    true,
				Description: "NTP server as an IPv4 literal -- the switch accepts no hostnames. Required when mode is \"Unicast\".",
			},
			"port": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(sntpDefaultPort),
				Description: "NTP server port, 1-65535. Default 123.",
			},
			"time_format": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("24 Hour"),
				Description: "\"24 Hour\" (default) or \"12 Hour\" -- how the web UI shows times. Lives on the same page, so it's part of this resource.",
			},
			"last_update_status": schema.StringAttribute{
				Computed:    true,
				Description: "Read-only: result of the last sync attempt, e.g. \"Success\" or \"Request timed out\".",
			},
		},
	}
}

func (r *sntpResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected resource configure type", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

// ValidateConfig catches a Unicast config without a server at plan time
// instead of halfway through an apply. The IPv4/port checks themselves
// live in client.sntpFormFields, which runs before anything is written.
func (r *sntpResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg sntpResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg.Mode.IsUnknown() || cfg.Server.IsUnknown() {
		return
	}
	switch cfg.Mode.ValueString() {
	case client.SNTPModeUnicast:
		if cfg.Server.IsNull() || cfg.Server.ValueString() == "" {
			resp.Diagnostics.AddAttributeError(path.Root("server"), "Missing server", "server is required when mode is \"Unicast\".")
		}
	case client.SNTPModeDisabled:
	default:
		resp.Diagnostics.AddAttributeError(path.Root("mode"), "Invalid mode", fmt.Sprintf("mode must be %q or %q, got %q.", client.SNTPModeUnicast, client.SNTPModeDisabled, cfg.Mode.ValueString()))
	}
}

// toModel builds state from what the switch reports. For Disabled, server
// and port come from prior (plan or state) -- see the file comment.
func sntpToModel(actual client.SNTPConfig, prior sntpResourceModel) sntpResourceModel {
	m := sntpResourceModel{
		ID:               types.StringValue("sntp"),
		Mode:             types.StringValue(actual.Mode),
		Server:           types.StringValue(actual.Server),
		Port:             types.Int64Value(int64(actual.Port)),
		TimeFormat:       types.StringValue(actual.TimeFormat),
		LastUpdateStatus: types.StringValue(actual.LastUpdateStatus),
	}
	if actual.Mode == client.SNTPModeDisabled {
		m.Server = prior.Server
		m.Port = prior.Port
	}
	return m
}

func sntpDesired(plan sntpResourceModel) client.SNTPConfig {
	return client.SNTPConfig{
		Mode:       plan.Mode.ValueString(),
		Server:     plan.Server.ValueString(),
		Port:       int(plan.Port.ValueInt64()),
		TimeFormat: plan.TimeFormat.ValueString(),
	}
}

// apply writes desired, reads it back and fails if the switch didn't take
// it (the firmware silently ignores many invalid writes, see SubmitForm).
func (r *sntpResource) apply(plan sntpResourceModel) (sntpResourceModel, error) {
	desired := sntpDesired(plan)
	if err := r.client.SetSNTPConfig(desired); err != nil {
		return sntpResourceModel{}, err
	}
	actual, err := r.client.ReadSNTPConfig()
	if err != nil {
		return sntpResourceModel{}, fmt.Errorf("reading back: %w", err)
	}
	if actual.Mode != desired.Mode || actual.TimeFormat != desired.TimeFormat ||
		(desired.Mode == client.SNTPModeUnicast && (actual.Server != desired.Server || actual.Port != desired.Port)) {
		return sntpResourceModel{}, fmt.Errorf("switch did not apply the change: wanted %+v, read back mode=%q server=%q port=%d time_format=%q",
			desired, actual.Mode, actual.Server, actual.Port, actual.TimeFormat)
	}
	return sntpToModel(actual, plan), nil
}

func (r *sntpResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sntpResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, err := r.apply(plan)
	if err != nil {
		resp.Diagnostics.AddError("Unable to set SNTP configuration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sntpResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior sntpResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	actual, err := r.client.ReadSNTPConfig()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read SNTP configuration", err.Error())
		return
	}
	state := sntpToModel(actual, prior)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sntpResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sntpResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, err := r.apply(plan)
	if err != nil {
		resp.Diagnostics.AddError("Unable to set SNTP configuration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete disables SNTP. Harmless: the clock keeps running from the last
// sync, it just drifts. Time format is left as it was.
func (r *sntpResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sntpResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.SetSNTPConfig(client.SNTPConfig{Mode: client.SNTPModeDisabled, TimeFormat: state.TimeFormat.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Unable to disable SNTP on delete", err.Error())
	}
}
