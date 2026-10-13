// Port of PortConfiguration.html -- see client/port_config.go's package doc
// for why this page needed dedicated (non-generic) client-side handling.
//
// This is a materially riskier write target than Locator/Green Features: a
// wrong write can disable the port a management session, or the switch's
// only uplink, depends on. See the admin-port guard below.
package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

var (
	_ resource.Resource              = &portResource{}
	_ resource.ResourceWithConfigure = &portResource{}
)

func NewPortResource() resource.Resource {
	return &portResource{}
}

type portResource struct {
	client *client.Client
}

type portResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Interface    types.String `tfsdk:"interface"`
	AdminMode    types.Bool   `tfsdk:"admin_mode"`
	LinkSpeed    types.String `tfsdk:"link_speed"`
	PhysicalType types.String `tfsdk:"physical_type"`
	LinkStatus   types.String `tfsdk:"link_status"`
}

func (r *portResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_port"
}

func (r *portResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Controls one switch port's Admin Mode and Link Speed. Intended to be driven with for_each over a map keyed by port number. The provider's admin_port (default 24) can never be written by this resource, regardless of what's in the for_each map -- see the provider schema.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Same as interface.",
			},
			"interface": schema.StringAttribute{
				Required:    true,
				Description: "Port number, \"1\"..\"24\". Changing this replaces the resource (it's the for_each key in normal usage).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"admin_mode": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the port is enabled.",
			},
			"link_speed": schema.StringAttribute{
				Required:    true,
				Description: "e.g. \"Auto\", \"100 Mbps Full Duplex\". Not enum-validated client-side -- the switch's accepted values for gigabit copper ports aren't fully confirmed, so invalid values are caught by reading the value back rather than guessed at up front.",
			},
			"physical_type": schema.StringAttribute{
				Computed:    true,
				Description: "Read-only, e.g. \"No SFP\" for RJ45 copper.",
			},
			"link_status": schema.StringAttribute{
				Computed:    true,
				Description: "Read-only, \"Link Up\" or \"Link Down\".",
			},
		},
	}
}

func (r *portResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// guardAdminPort refuses any write targeting the provider's protected
// admin_port (default 24 -- the switch's uplink), and refuses a
// non-numeric interface value outright. This is the safety net requested
// explicitly: even if the admin port ends up in a for_each map by mistake,
// the write never reaches the switch.
func (r *portResource) guardAdminPort(interfaceStr string) error {
	n, err := strconv.Atoi(interfaceStr)
	if err != nil {
		return fmt.Errorf("interface %q is not a valid port number: %w", interfaceStr, err)
	}
	if int64(n) == r.client.AdminPort {
		return fmt.Errorf("refusing to write to interface %q: it's the provider's admin_port (%d), protected from all writes by this resource", interfaceStr, r.client.AdminPort)
	}
	return nil
}

func portConfigToModel(pc client.PortConfig) portResourceModel {
	return portResourceModel{
		ID:           types.StringValue(pc.Interface),
		Interface:    types.StringValue(pc.Interface),
		AdminMode:    types.BoolValue(pc.AdminMode),
		LinkSpeed:    types.StringValue(pc.LinkSpeed),
		PhysicalType: types.StringValue(pc.PhysicalType),
		LinkStatus:   types.StringValue(pc.LinkStatus),
	}
}

func (r *portResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan portResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	iface := plan.Interface.ValueString()
	if err := r.guardAdminPort(iface); err != nil {
		resp.Diagnostics.AddError("Refusing to write to admin_port", err.Error())
		return
	}

	if err := r.client.SetPortConfig(iface, plan.AdminMode.ValueBool(), plan.LinkSpeed.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to set port config", err.Error())
		return
	}

	pc, err := r.client.ReadPortConfig(iface)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back port config", err.Error())
		return
	}

	state := portConfigToModel(pc)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *portResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state portResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pc, err := r.client.ReadPortConfig(state.Interface.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read port config", err.Error())
		return
	}

	newState := portConfigToModel(pc)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *portResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan portResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	iface := plan.Interface.ValueString()
	if err := r.guardAdminPort(iface); err != nil {
		resp.Diagnostics.AddError("Refusing to write to admin_port", err.Error())
		return
	}

	if err := r.client.SetPortConfig(iface, plan.AdminMode.ValueBool(), plan.LinkSpeed.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to set port config", err.Error())
		return
	}

	pc, err := r.client.ReadPortConfig(iface)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back port config", err.Error())
		return
	}

	state := portConfigToModel(pc)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete disables the port and resets Link Speed to Auto -- matches
// AGENT.md's Port Configuration risk-triage decision: "no longer managed by
// Terraform" should fail toward safer (closed), not toward whatever it
// happened to be. Safe to apply unconditionally here because the one port
// that must never be disabled (admin_port) is structurally excluded by the
// guard below -- it can never have been Created in the first place.
func (r *portResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state portResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	iface := state.Interface.ValueString()
	if err := r.guardAdminPort(iface); err != nil {
		resp.Diagnostics.AddError("Refusing to write to admin_port", err.Error())
		return
	}

	if err := r.client.SetPortConfig(iface, false, "Auto"); err != nil {
		resp.Diagnostics.AddError("Unable to reset port config on delete", err.Error())
	}
}
