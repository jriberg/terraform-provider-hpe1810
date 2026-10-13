// Port of ~/code/experiments/hpe/actions/locator.py -- the first write
// resource. Singleton setting (nothing to "create" in the usual sense), so
// Create/Update both just push the desired state and Read always reports
// whatever the switch currently has.
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
	locatorPath    = "/locator.html"
	locatorCaption = "Locator Configuration"
	locatorField   = "Locate"
)

var (
	_ resource.Resource              = &locatorResource{}
	_ resource.ResourceWithConfigure = &locatorResource{}
)

func NewLocatorResource() resource.Resource {
	return &locatorResource{}
}

type locatorResource struct {
	client *client.Client
}

type locatorResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func (r *locatorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_locator"
}

func (r *locatorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Controls the switch's front-panel Locator LED (blink on/off). Singleton -- there is only ever one of these per switch. Delete resets it to Disable (a safe, harmless default), not a no-op.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always \"locator\" -- this is a singleton resource.",
			},
			"enabled": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the locator LED should be blinking.",
			},
		},
	}
}

func (r *locatorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// readLocatorState mirrors actions/locator.py's get_locator_state().
func (r *locatorResource) readLocatorState() (bool, error) {
	tables, err := r.client.FetchTables(locatorPath)
	if err != nil {
		return false, err
	}
	table, ok := tables[locatorCaption]
	if !ok {
		return false, fmt.Errorf("table captioned %q not found on %s", locatorCaption, locatorPath)
	}
	return table.Record[locatorField] == "Enable", nil
}

// setLocatorState mirrors actions/locator.py's set_locator(): it looks up
// the real (hidden) form field name via FieldNames rather than hardcoding
// it, same as the Python reference implementation does.
func (r *locatorResource) setLocatorState(enabled bool) error {
	tables, err := r.client.FetchTables(locatorPath)
	if err != nil {
		return err
	}
	table, ok := tables[locatorCaption]
	if !ok {
		return fmt.Errorf("table captioned %q not found on %s", locatorCaption, locatorPath)
	}
	fieldName, ok := table.FieldNames[locatorField]
	if !ok {
		return fmt.Errorf("field %q not found in table %q", locatorField, locatorCaption)
	}

	value := "Disable"
	if enabled {
		value = "Enable"
	}
	_, err = r.client.SubmitForm(locatorPath, map[string]string{fieldName: value})
	return err
}

func (r *locatorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan locatorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setLocatorState(plan.Enabled.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Unable to set locator state", err.Error())
		return
	}

	enabled, err := r.readLocatorState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back locator state", err.Error())
		return
	}

	state := locatorResourceModel{ID: types.StringValue("locator"), Enabled: types.BoolValue(enabled)}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *locatorResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	enabled, err := r.readLocatorState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read locator state", err.Error())
		return
	}

	state := locatorResourceModel{ID: types.StringValue("locator"), Enabled: types.BoolValue(enabled)}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *locatorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan locatorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setLocatorState(plan.Enabled.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Unable to set locator state", err.Error())
		return
	}

	enabled, err := r.readLocatorState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back locator state", err.Error())
		return
	}

	state := locatorResourceModel{ID: types.StringValue("locator"), Enabled: types.BoolValue(enabled)}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete resets the locator to Disable (its safe, harmless default) rather
// than leaving it however it was -- see AGENT.md's per-resource Delete
// semantics notes. Unlike Network Setup, there's no risk in touching this on
// delete: the locator is purely a physical LED, not a management config.
func (r *locatorResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	if err := r.setLocatorState(false); err != nil {
		resp.Diagnostics.AddError("Unable to reset locator state on delete", err.Error())
	}
}
