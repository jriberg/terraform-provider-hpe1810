// Port of Summer_Time_Configuration.html -- see client/dst_config.go's
// package doc comment for why this page needed dedicated handling. Only
// Disable and Recurring modes are supported; NonRecurring is rejected by
// the client with a clear error rather than guessed at.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

var (
	_ resource.Resource              = &daylightSavingTimeResource{}
	_ resource.ResourceWithConfigure = &daylightSavingTimeResource{}
)

func NewDaylightSavingTimeResource() resource.Resource {
	return &daylightSavingTimeResource{}
}

type daylightSavingTimeResource struct {
	client *client.Client
}

type daylightSavingTimeResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Mode          types.String `tfsdk:"mode"`
	StartWeek     types.Int64  `tfsdk:"start_week"`
	StartDay      types.String `tfsdk:"start_day"`
	StartMonth    types.String `tfsdk:"start_month"`
	StartHour     types.Int64  `tfsdk:"start_hour"`
	StartMinute   types.Int64  `tfsdk:"start_minute"`
	EndWeek       types.Int64  `tfsdk:"end_week"`
	EndDay        types.String `tfsdk:"end_day"`
	EndMonth      types.String `tfsdk:"end_month"`
	EndHour       types.Int64  `tfsdk:"end_hour"`
	EndMinute     types.Int64  `tfsdk:"end_minute"`
	OffsetMinutes types.Int64  `tfsdk:"offset_minutes"`
	OffsetAcronym types.String `tfsdk:"offset_acronym"`
}

func (r *daylightSavingTimeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_daylight_saving_time"
}

func (r *daylightSavingTimeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Controls the switch's Daylight Saving Time Configuration page (Summer_Time_Configuration.html). Singleton -- there is only ever one of these per switch. Only \"Disable\" and \"Recurring\" modes are supported (the page's NonRecurring mode uses a different absolute-date field set and is rejected). Delete sets mode to \"Disable\", leaving the recurring rule fields as last configured (harmless once disabled).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always \"daylight_saving_time\" -- this is a singleton resource.",
			},
			"mode": schema.StringAttribute{
				Required:    true,
				Description: "\"Disable\" or \"Recurring\" only.",
			},
			"start_week": schema.Int64Attribute{
				Required:    true,
				Description: "Week of the month DST starts (1-4, or 5 for \"last\").",
			},
			"start_day": schema.StringAttribute{
				Required:    true,
				Description: "e.g. \"Sun\".",
			},
			"start_month": schema.StringAttribute{
				Required:    true,
				Description: "e.g. \"Mar\".",
			},
			"start_hour": schema.Int64Attribute{
				Required:    true,
				Description: "0-23.",
			},
			"start_minute": schema.Int64Attribute{
				Required:    true,
				Description: "0-59.",
			},
			"end_week": schema.Int64Attribute{
				Required:    true,
				Description: "Week of the month DST ends (1-4, or 5 for \"last\").",
			},
			"end_day": schema.StringAttribute{
				Required:    true,
				Description: "e.g. \"Sun\".",
			},
			"end_month": schema.StringAttribute{
				Required:    true,
				Description: "e.g. \"Oct\".",
			},
			"end_hour": schema.Int64Attribute{
				Required:    true,
				Description: "0-23.",
			},
			"end_minute": schema.Int64Attribute{
				Required:    true,
				Description: "0-59.",
			},
			"offset_minutes": schema.Int64Attribute{
				Required:    true,
				Description: "Minutes to shift the clock during DST. Range 1-1440.",
			},
			"offset_acronym": schema.StringAttribute{
				Required:    true,
				Description: "0-4 characters, e.g. \"CEST\". Can be an empty string.",
			},
		},
	}
}

func (r *daylightSavingTimeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func dstConfigToModel(c client.DSTConfig) daylightSavingTimeResourceModel {
	return daylightSavingTimeResourceModel{
		ID:            types.StringValue("daylight_saving_time"),
		Mode:          types.StringValue(c.Mode),
		StartWeek:     types.Int64Value(int64(c.StartWeek)),
		StartDay:      types.StringValue(c.StartDay),
		StartMonth:    types.StringValue(c.StartMonth),
		StartHour:     types.Int64Value(int64(c.StartHour)),
		StartMinute:   types.Int64Value(int64(c.StartMinute)),
		EndWeek:       types.Int64Value(int64(c.EndWeek)),
		EndDay:        types.StringValue(c.EndDay),
		EndMonth:      types.StringValue(c.EndMonth),
		EndHour:       types.Int64Value(int64(c.EndHour)),
		EndMinute:     types.Int64Value(int64(c.EndMinute)),
		OffsetMinutes: types.Int64Value(int64(c.OffsetMinutes)),
		OffsetAcronym: types.StringValue(c.OffsetAcronym),
	}
}

func modelToDSTConfig(m daylightSavingTimeResourceModel) client.DSTConfig {
	return client.DSTConfig{
		Mode:          m.Mode.ValueString(),
		StartWeek:     int(m.StartWeek.ValueInt64()),
		StartDay:      m.StartDay.ValueString(),
		StartMonth:    m.StartMonth.ValueString(),
		StartHour:     int(m.StartHour.ValueInt64()),
		StartMinute:   int(m.StartMinute.ValueInt64()),
		EndWeek:       int(m.EndWeek.ValueInt64()),
		EndDay:        m.EndDay.ValueString(),
		EndMonth:      m.EndMonth.ValueString(),
		EndHour:       int(m.EndHour.ValueInt64()),
		EndMinute:     int(m.EndMinute.ValueInt64()),
		OffsetMinutes: int(m.OffsetMinutes.ValueInt64()),
		OffsetAcronym: m.OffsetAcronym.ValueString(),
	}
}

func (r *daylightSavingTimeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan daylightSavingTimeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetDSTConfig(modelToDSTConfig(plan)); err != nil {
		resp.Diagnostics.AddError("Unable to set Daylight Saving Time state", err.Error())
		return
	}

	actual, err := r.client.ReadDSTConfig()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back Daylight Saving Time state", err.Error())
		return
	}

	state := dstConfigToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *daylightSavingTimeResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	actual, err := r.client.ReadDSTConfig()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Daylight Saving Time state", err.Error())
		return
	}

	state := dstConfigToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *daylightSavingTimeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan daylightSavingTimeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetDSTConfig(modelToDSTConfig(plan)); err != nil {
		resp.Diagnostics.AddError("Unable to set Daylight Saving Time state", err.Error())
		return
	}

	actual, err := r.client.ReadDSTConfig()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back Daylight Saving Time state", err.Error())
		return
	}

	state := dstConfigToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete sets mode to Disable, leaving the recurring rule fields as last
// configured -- harmless once DST itself is disabled, and avoids having to
// invent a "correct" default rule for an arbitrary switch's timezone.
func (r *daylightSavingTimeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state daylightSavingTimeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired := modelToDSTConfig(state)
	desired.Mode = client.DSTModeDisable
	if err := r.client.SetDSTConfig(desired); err != nil {
		resp.Diagnostics.AddError("Unable to reset Daylight Saving Time state on delete", err.Error())
	}
}
