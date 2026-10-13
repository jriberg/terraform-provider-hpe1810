// Port of SwitchConfig.html -- single boolean field, same shape as
// resource_locator.go.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const (
	flowControlPath        = "/SwitchConfig.html"
	flowControlCaption     = "Flow Control Configuration"
	flowControlEnableLabel = "Enable Flow Control"
)

var (
	_ resource.Resource              = &flowControlResource{}
	_ resource.ResourceWithConfigure = &flowControlResource{}
)

func NewFlowControlResource() resource.Resource {
	return &flowControlResource{}
}

type flowControlResource struct {
	client *client.Client
}

type flowControlResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func (r *flowControlResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_flow_control"
}

func (r *flowControlResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Controls the switch's global Flow Control setting (SwitchConfig.html). Singleton -- there is only ever one of these per switch. Delete resets it to Enable (the switch's factory default).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always \"flow_control\" -- this is a singleton resource.",
			},
			"enabled": schema.BoolAttribute{
				Required:    true,
				Description: "Whether 802.3x flow control is enabled switch-wide.",
			},
		},
	}
}

func (r *flowControlResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *flowControlResource) readState() (bool, error) {
	tables, err := r.client.FetchTables(flowControlPath)
	if err != nil {
		return false, err
	}
	table, ok := tables[flowControlCaption]
	if !ok {
		return false, fmt.Errorf("table captioned %q not found on %s", flowControlCaption, flowControlPath)
	}
	return table.Record[flowControlEnableLabel] == "Enable", nil
}

func (r *flowControlResource) setState(enabled bool) error {
	tables, err := r.client.FetchTables(flowControlPath)
	if err != nil {
		return err
	}
	table, ok := tables[flowControlCaption]
	if !ok {
		return fmt.Errorf("table captioned %q not found on %s", flowControlCaption, flowControlPath)
	}
	field, ok := table.FieldNames[flowControlEnableLabel]
	if !ok {
		return fmt.Errorf("field %q not found in table %q", flowControlEnableLabel, flowControlCaption)
	}

	value := "Disable"
	if enabled {
		value = "Enable"
	}
	_, err = r.client.SubmitForm(flowControlPath, map[string]string{field: value})
	return err
}

func (r *flowControlResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan flowControlResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setState(plan.Enabled.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Unable to set Flow Control state", err.Error())
		return
	}

	enabled, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back Flow Control state", err.Error())
		return
	}

	state := flowControlResourceModel{ID: types.StringValue("flow_control"), Enabled: types.BoolValue(enabled)}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *flowControlResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	enabled, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Flow Control state", err.Error())
		return
	}

	state := flowControlResourceModel{ID: types.StringValue("flow_control"), Enabled: types.BoolValue(enabled)}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *flowControlResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan flowControlResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setState(plan.Enabled.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Unable to set Flow Control state", err.Error())
		return
	}

	enabled, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back Flow Control state", err.Error())
		return
	}

	state := flowControlResourceModel{ID: types.StringValue("flow_control"), Enabled: types.BoolValue(enabled)}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete resets to Enable -- the switch's factory default, harmless (flow
// control is a link-layer congestion mechanism, not a security setting).
func (r *flowControlResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	if err := r.setState(true); err != nil {
		resp.Diagnostics.AddError("Unable to reset Flow Control state on delete", err.Error())
	}
}
