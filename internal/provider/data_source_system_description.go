// Port of ~/code/experiments/hpe/extractors/system_description.py
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const systemDescriptionPath = "/SysDescription.html"

var (
	_ datasource.DataSource              = &systemDescriptionDataSource{}
	_ datasource.DataSourceWithConfigure = &systemDescriptionDataSource{}
)

func NewSystemDescriptionDataSource() datasource.DataSource {
	return &systemDescriptionDataSource{}
}

type systemDescriptionDataSource struct {
	client *client.Client
}

type systemDescriptionModel struct {
	Description     types.String `tfsdk:"description"`
	Name            types.String `tfsdk:"name"`
	Location        types.String `tfsdk:"location"`
	Contact         types.String `tfsdk:"contact"`
	SoftwareVersion types.String `tfsdk:"software_version"`
	ObjectID        types.String `tfsdk:"object_id"`
	UpTime          types.String `tfsdk:"up_time"`
	CurrentTime     types.String `tfsdk:"current_time"`
	Date            types.String `tfsdk:"date"`
}

func (d *systemDescriptionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_description"
}

func (d *systemDescriptionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the switch's System Description page (name, location, contact, uptime, ...).",
		Attributes: map[string]schema.Attribute{
			"description":      schema.StringAttribute{Computed: true},
			"name":             schema.StringAttribute{Computed: true},
			"location":         schema.StringAttribute{Computed: true},
			"contact":          schema.StringAttribute{Computed: true},
			"software_version": schema.StringAttribute{Computed: true},
			"object_id":        schema.StringAttribute{Computed: true},
			"up_time":          schema.StringAttribute{Computed: true},
			"current_time":     schema.StringAttribute{Computed: true},
			"date":             schema.StringAttribute{Computed: true},
		},
	}
}

func (d *systemDescriptionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected data source configure type", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *systemDescriptionDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tables, err := d.client.FetchTables(systemDescriptionPath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch System Description page", err.Error())
		return
	}
	table, ok := tables["System Description"]
	if !ok {
		resp.Diagnostics.AddError("Unexpected page layout", `table captioned "System Description" not found`)
		return
	}
	record := table.Record

	state := systemDescriptionModel{
		Description:     types.StringValue(record["System Description"]),
		Name:            types.StringValue(record["System Name"]),
		Location:        types.StringValue(record["System Location"]),
		Contact:         types.StringValue(record["System Contact"]),
		SoftwareVersion: types.StringValue(record["Software Version"]),
		ObjectID:        types.StringValue(record["System Object ID"]),
		UpTime:          types.StringValue(record["System Up Time"]),
		CurrentTime:     types.StringValue(record["Current Time"]),
		Date:            types.StringValue(record["Date"]),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
