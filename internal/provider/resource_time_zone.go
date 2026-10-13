// Port of Time_Zone_Configuration.html. Simple scalar page, both fields
// (Time Zone combo, Acronym textbox) are read-write, no duplicate labels.
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
	timeZonePath         = "/Time_Zone_Configuration.html"
	timeZoneCaption      = "Time Zone Configuration"
	timeZoneNameLabel    = "Time Zone"
	timeZoneAcronymLabel = "Acronym"
)

var (
	_ resource.Resource              = &timeZoneResource{}
	_ resource.ResourceWithConfigure = &timeZoneResource{}
)

func NewTimeZoneResource() resource.Resource {
	return &timeZoneResource{}
}

type timeZoneResource struct {
	client *client.Client
}

type timeZoneResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Zone    types.String `tfsdk:"zone"`
	Acronym types.String `tfsdk:"acronym"`
}

func (r *timeZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_time_zone"
}

func (r *timeZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Controls the switch's Time Zone Configuration page. Singleton -- there is only ever one of these per switch. Delete resets to \"None\" (no offset) with an empty acronym.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always \"time_zone\" -- this is a singleton resource.",
			},
			"zone": schema.StringAttribute{
				Required:    true,
				Description: "e.g. \"(GMT+01:00) Amsterdam, Berlin, Bern, Rome, Stockholm, Vienna\", or \"None\". Not enum-validated client-side -- the exact display strings the switch accepts are numerous (see _xe_Time_Zone_Configuration.js on the switch); invalid values are caught by reading back rather than guessed at up front.",
			},
			"acronym": schema.StringAttribute{
				Required:    true,
				Description: "0-4 characters, e.g. \"CET\". Can be an empty string.",
			},
		},
	}
}

func (r *timeZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type timeZoneState struct {
	Zone    string
	Acronym string
}

func (r *timeZoneResource) readState() (timeZoneState, error) {
	tables, err := r.client.FetchTables(timeZonePath)
	if err != nil {
		return timeZoneState{}, err
	}
	table, ok := tables[timeZoneCaption]
	if !ok {
		return timeZoneState{}, fmt.Errorf("table captioned %q not found on %s", timeZoneCaption, timeZonePath)
	}
	return timeZoneState{
		Zone:    table.Record[timeZoneNameLabel],
		Acronym: table.Record[timeZoneAcronymLabel],
	}, nil
}

func (r *timeZoneResource) setState(desired timeZoneState) error {
	tables, err := r.client.FetchTables(timeZonePath)
	if err != nil {
		return err
	}
	table, ok := tables[timeZoneCaption]
	if !ok {
		return fmt.Errorf("table captioned %q not found on %s", timeZoneCaption, timeZonePath)
	}
	zoneField, ok := table.FieldNames[timeZoneNameLabel]
	if !ok {
		return fmt.Errorf("field %q not found in table %q", timeZoneNameLabel, timeZoneCaption)
	}
	acronymField, ok := table.FieldNames[timeZoneAcronymLabel]
	if !ok {
		return fmt.Errorf("field %q not found in table %q", timeZoneAcronymLabel, timeZoneCaption)
	}

	_, err = r.client.SubmitForm(timeZonePath, map[string]string{
		zoneField:    desired.Zone,
		acronymField: desired.Acronym,
	})
	return err
}

func timeZoneStateToModel(s timeZoneState) timeZoneResourceModel {
	return timeZoneResourceModel{
		ID:      types.StringValue("time_zone"),
		Zone:    types.StringValue(s.Zone),
		Acronym: types.StringValue(s.Acronym),
	}
}

func (r *timeZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan timeZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired := timeZoneState{Zone: plan.Zone.ValueString(), Acronym: plan.Acronym.ValueString()}
	if err := r.setState(desired); err != nil {
		resp.Diagnostics.AddError("Unable to set Time Zone state", err.Error())
		return
	}

	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back Time Zone state", err.Error())
		return
	}

	state := timeZoneStateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *timeZoneResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Time Zone state", err.Error())
		return
	}

	state := timeZoneStateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *timeZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan timeZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired := timeZoneState{Zone: plan.Zone.ValueString(), Acronym: plan.Acronym.ValueString()}
	if err := r.setState(desired); err != nil {
		resp.Diagnostics.AddError("Unable to set Time Zone state", err.Error())
		return
	}

	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back Time Zone state", err.Error())
		return
	}

	state := timeZoneStateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete resets to "None" (no timezone offset) with an empty acronym --
// harmless, matches the switch's out-of-box default.
func (r *timeZoneResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	if err := r.setState(timeZoneState{Zone: "None", Acronym: ""}); err != nil {
		resp.Diagnostics.AddError("Unable to reset Time Zone state on delete", err.Error())
	}
}
