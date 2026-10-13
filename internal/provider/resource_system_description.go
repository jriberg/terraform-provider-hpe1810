// Port of the writable subset of SysDescription.html. Most fields on that
// page (Description, Software Version, Object ID, Up Time, Current Time,
// Date) are pure device-reported info -- confirmed not writable by the
// page's own client-side validation script (xeValData), which only defines
// error messages for v_1_2_1/v_1_3_1/v_1_4_1 (System Name/Location/Contact).
// Those three are what this resource manages; the rest stay exclusively in
// data_source_system_description.go.
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
	sysDescriptionCaption = "System Description"
	sysNameLabel          = "System Name"
	sysLocationLabel      = "System Location"
	sysContactLabel       = "System Contact"
)

var (
	_ resource.Resource              = &systemDescriptionResource{}
	_ resource.ResourceWithConfigure = &systemDescriptionResource{}
)

func NewSystemDescriptionResource() resource.Resource {
	return &systemDescriptionResource{}
}

type systemDescriptionResource struct {
	client *client.Client
}

type systemDescriptionResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Location types.String `tfsdk:"location"`
	Contact  types.String `tfsdk:"contact"`
}

func (r *systemDescriptionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_description"
}

func (r *systemDescriptionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Controls the switch's System Name/Location/Contact (SysDescription.html). Singleton -- there is only ever one of these per switch. The other fields on that page (description, software version, uptime, ...) are read-only device info -- see the hpe1810_system_description data source for those. Delete resets all three fields to empty strings.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always \"system_description\" -- this is a singleton resource.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "System Name (sysName).",
			},
			"location": schema.StringAttribute{
				Required:    true,
				Description: "System Location (sysLocation).",
			},
			"contact": schema.StringAttribute{
				Required:    true,
				Description: "System Contact (sysContact). Can be an empty string.",
			},
		},
	}
}

func (r *systemDescriptionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type systemDescriptionState struct {
	Name     string
	Location string
	Contact  string
}

func (r *systemDescriptionResource) readState() (systemDescriptionState, error) {
	tables, err := r.client.FetchTables(systemDescriptionPath)
	if err != nil {
		return systemDescriptionState{}, err
	}
	table, ok := tables[sysDescriptionCaption]
	if !ok {
		return systemDescriptionState{}, fmt.Errorf("table captioned %q not found on %s", sysDescriptionCaption, systemDescriptionPath)
	}
	return systemDescriptionState{
		Name:     table.Record[sysNameLabel],
		Location: table.Record[sysLocationLabel],
		Contact:  table.Record[sysContactLabel],
	}, nil
}

func (r *systemDescriptionResource) setState(desired systemDescriptionState) error {
	tables, err := r.client.FetchTables(systemDescriptionPath)
	if err != nil {
		return err
	}
	table, ok := tables[sysDescriptionCaption]
	if !ok {
		return fmt.Errorf("table captioned %q not found on %s", sysDescriptionCaption, systemDescriptionPath)
	}

	nameField, ok := table.FieldNames[sysNameLabel]
	if !ok {
		return fmt.Errorf("field %q not found in table %q", sysNameLabel, sysDescriptionCaption)
	}
	locationField, ok := table.FieldNames[sysLocationLabel]
	if !ok {
		return fmt.Errorf("field %q not found in table %q", sysLocationLabel, sysDescriptionCaption)
	}
	contactField, ok := table.FieldNames[sysContactLabel]
	if !ok {
		return fmt.Errorf("field %q not found in table %q", sysContactLabel, sysDescriptionCaption)
	}

	_, err = r.client.SubmitForm(systemDescriptionPath, map[string]string{
		nameField:     desired.Name,
		locationField: desired.Location,
		contactField:  desired.Contact,
	})
	return err
}

func sysDescStateToModel(s systemDescriptionState) systemDescriptionResourceModel {
	return systemDescriptionResourceModel{
		ID:       types.StringValue("system_description"),
		Name:     types.StringValue(s.Name),
		Location: types.StringValue(s.Location),
		Contact:  types.StringValue(s.Contact),
	}
}

func (r *systemDescriptionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan systemDescriptionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired := systemDescriptionState{
		Name:     plan.Name.ValueString(),
		Location: plan.Location.ValueString(),
		Contact:  plan.Contact.ValueString(),
	}
	if err := r.setState(desired); err != nil {
		resp.Diagnostics.AddError("Unable to set System Description state", err.Error())
		return
	}

	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back System Description state", err.Error())
		return
	}

	state := sysDescStateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *systemDescriptionResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read System Description state", err.Error())
		return
	}

	state := sysDescStateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *systemDescriptionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan systemDescriptionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired := systemDescriptionState{
		Name:     plan.Name.ValueString(),
		Location: plan.Location.ValueString(),
		Contact:  plan.Contact.ValueString(),
	}
	if err := r.setState(desired); err != nil {
		resp.Diagnostics.AddError("Unable to set System Description state", err.Error())
		return
	}

	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back System Description state", err.Error())
		return
	}

	state := sysDescStateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete resets all three fields to empty strings -- harmless, and matches
// the switch's out-of-box default before any of this was configured.
func (r *systemDescriptionResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	if err := r.setState(systemDescriptionState{}); err != nil {
		resp.Diagnostics.AddError("Unable to reset System Description state on delete", err.Error())
	}
}
