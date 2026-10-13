// Port of ~/code/experiments/hpe/extractors/clock.py
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const clockPath = "/Clock_Detail.html"

var (
	_ datasource.DataSource              = &clockDataSource{}
	_ datasource.DataSourceWithConfigure = &clockDataSource{}
)

func NewClockDataSource() datasource.DataSource {
	return &clockDataSource{}
}

type clockDataSource struct {
	client *client.Client
}

type clockModel struct {
	Tables types.Map `tfsdk:"tables"`
}

func (d *clockDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_clock"
}

func (d *clockDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the switch's Clock Detail page: Current Time, Time Zone and Daylight Saving Time, keyed by table caption.",
		Attributes: map[string]schema.Attribute{
			"tables": schema.MapAttribute{
				Computed:    true,
				ElementType: types.MapType{ElemType: types.StringType},
			},
		},
	}
}

func (d *clockDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *clockDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tables, err := d.client.FetchTables(clockPath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch Clock Detail page", err.Error())
		return
	}

	records := make(map[string]map[string]string, len(tables))
	for caption, table := range tables {
		records[caption] = table.Record
	}

	tablesValue, diags := tableOfRecordsToTF(ctx, records)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := clockModel{Tables: tablesValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
