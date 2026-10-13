// Port of LLDPConfig.html's "Global Mode" table only. The page also has a
// per-port "Interface Mode" table (Transmit/Receive Enable etc, one row per
// port with per-port field names like "1.0.24.v_2_1_2") -- that's a
// separate, more involved write target (needs the same per-port-field-name
// handling as resource_port.go) and isn't built yet; only requested was
// "LLDP Configuration" which maps to the global settings.
package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const (
	lldpConfigPath                = "/LLDPConfig.html"
	lldpGlobalModeCaption         = "Global Mode"
	lldpTransmitIntervalLabel     = "Transmit Interval"
	lldpTransmitHoldLabel         = "Transmit Hold"
	lldpReinitDelayLabel          = "Re-Initialization Delay"
	lldpNotificationIntervalLabel = "Notification Interval"
)

var (
	_ resource.Resource              = &lldpConfigurationResource{}
	_ resource.ResourceWithConfigure = &lldpConfigurationResource{}
)

func NewLLDPConfigurationResource() resource.Resource {
	return &lldpConfigurationResource{}
}

type lldpConfigurationResource struct {
	client *client.Client
}

type lldpConfigurationResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	TransmitInterval     types.Int64  `tfsdk:"transmit_interval"`
	TransmitHold         types.Int64  `tfsdk:"transmit_hold"`
	ReinitDelay          types.Int64  `tfsdk:"reinit_delay"`
	NotificationInterval types.Int64  `tfsdk:"notification_interval"`
}

func (r *lldpConfigurationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lldp_configuration"
}

func (r *lldpConfigurationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Controls the switch's LLDP Global Mode settings (LLDPConfig.html). Singleton -- there is only ever one of these per switch. Does NOT cover the page's per-port Interface Mode table -- see the package doc comment. Delete resets all four fields to their documented factory defaults.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always \"lldp_configuration\" -- this is a singleton resource.",
			},
			"transmit_interval": schema.Int64Attribute{
				Required:    true,
				Description: "Seconds between LLDP transmissions. Range 5-32768 (switch default: 30).",
			},
			"transmit_hold": schema.Int64Attribute{
				Required:    true,
				Description: "Multiplier for how long a receiver holds LLDP info before aging it out. Range 2-10 (switch default: 4).",
			},
			"reinit_delay": schema.Int64Attribute{
				Required:    true,
				Description: "Seconds to wait before re-initializing LLDP on a port after it's disabled. Range 1-10 (switch default: 2).",
			},
			"notification_interval": schema.Int64Attribute{
				Required:    true,
				Description: "Minimum seconds between LLDP change notifications. Range 5-3600 (switch default: 5).",
			},
		},
	}
}

func (r *lldpConfigurationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type lldpConfigurationState struct {
	TransmitInterval     int64
	TransmitHold         int64
	ReinitDelay          int64
	NotificationInterval int64
}

func (r *lldpConfigurationResource) readState() (lldpConfigurationState, error) {
	tables, err := r.client.FetchTables(lldpConfigPath)
	if err != nil {
		return lldpConfigurationState{}, err
	}
	table, ok := tables[lldpGlobalModeCaption]
	if !ok {
		return lldpConfigurationState{}, fmt.Errorf("table captioned %q not found on %s", lldpGlobalModeCaption, lldpConfigPath)
	}

	parse := func(label string) (int64, error) {
		v, err := strconv.ParseInt(table.Record[label], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing %q (%q) as int: %w", label, table.Record[label], err)
		}
		return v, nil
	}

	transmitInterval, err := parse(lldpTransmitIntervalLabel)
	if err != nil {
		return lldpConfigurationState{}, err
	}
	transmitHold, err := parse(lldpTransmitHoldLabel)
	if err != nil {
		return lldpConfigurationState{}, err
	}
	reinitDelay, err := parse(lldpReinitDelayLabel)
	if err != nil {
		return lldpConfigurationState{}, err
	}
	notificationInterval, err := parse(lldpNotificationIntervalLabel)
	if err != nil {
		return lldpConfigurationState{}, err
	}

	return lldpConfigurationState{
		TransmitInterval:     transmitInterval,
		TransmitHold:         transmitHold,
		ReinitDelay:          reinitDelay,
		NotificationInterval: notificationInterval,
	}, nil
}

func (r *lldpConfigurationResource) setState(desired lldpConfigurationState) error {
	tables, err := r.client.FetchTables(lldpConfigPath)
	if err != nil {
		return err
	}
	table, ok := tables[lldpGlobalModeCaption]
	if !ok {
		return fmt.Errorf("table captioned %q not found on %s", lldpGlobalModeCaption, lldpConfigPath)
	}

	field := func(label string) (string, error) {
		f, ok := table.FieldNames[label]
		if !ok {
			return "", fmt.Errorf("field %q not found in table %q", label, lldpGlobalModeCaption)
		}
		return f, nil
	}

	transmitIntervalField, err := field(lldpTransmitIntervalLabel)
	if err != nil {
		return err
	}
	transmitHoldField, err := field(lldpTransmitHoldLabel)
	if err != nil {
		return err
	}
	reinitDelayField, err := field(lldpReinitDelayLabel)
	if err != nil {
		return err
	}
	notificationIntervalField, err := field(lldpNotificationIntervalLabel)
	if err != nil {
		return err
	}

	_, err = r.client.SubmitForm(lldpConfigPath, map[string]string{
		transmitIntervalField:     strconv.FormatInt(desired.TransmitInterval, 10),
		transmitHoldField:         strconv.FormatInt(desired.TransmitHold, 10),
		reinitDelayField:          strconv.FormatInt(desired.ReinitDelay, 10),
		notificationIntervalField: strconv.FormatInt(desired.NotificationInterval, 10),
	})
	return err
}

func lldpConfigStateToModel(s lldpConfigurationState) lldpConfigurationResourceModel {
	return lldpConfigurationResourceModel{
		ID:                   types.StringValue("lldp_configuration"),
		TransmitInterval:     types.Int64Value(s.TransmitInterval),
		TransmitHold:         types.Int64Value(s.TransmitHold),
		ReinitDelay:          types.Int64Value(s.ReinitDelay),
		NotificationInterval: types.Int64Value(s.NotificationInterval),
	}
}

func (r *lldpConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan lldpConfigurationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired := lldpConfigurationState{
		TransmitInterval:     plan.TransmitInterval.ValueInt64(),
		TransmitHold:         plan.TransmitHold.ValueInt64(),
		ReinitDelay:          plan.ReinitDelay.ValueInt64(),
		NotificationInterval: plan.NotificationInterval.ValueInt64(),
	}
	if err := r.setState(desired); err != nil {
		resp.Diagnostics.AddError("Unable to set LLDP Configuration state", err.Error())
		return
	}

	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back LLDP Configuration state", err.Error())
		return
	}

	state := lldpConfigStateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lldpConfigurationResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read LLDP Configuration state", err.Error())
		return
	}

	state := lldpConfigStateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lldpConfigurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan lldpConfigurationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired := lldpConfigurationState{
		TransmitInterval:     plan.TransmitInterval.ValueInt64(),
		TransmitHold:         plan.TransmitHold.ValueInt64(),
		ReinitDelay:          plan.ReinitDelay.ValueInt64(),
		NotificationInterval: plan.NotificationInterval.ValueInt64(),
	}
	if err := r.setState(desired); err != nil {
		resp.Diagnostics.AddError("Unable to set LLDP Configuration state", err.Error())
		return
	}

	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back LLDP Configuration state", err.Error())
		return
	}

	state := lldpConfigStateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete resets all four fields to their documented factory defaults --
// safe, no traffic impact.
func (r *lldpConfigurationResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	defaults := lldpConfigurationState{TransmitInterval: 30, TransmitHold: 4, ReinitDelay: 2, NotificationInterval: 5}
	if err := r.setState(defaults); err != nil {
		resp.Diagnostics.AddError("Unable to reset LLDP Configuration state on delete", err.Error())
	}
}
