// Port of ~/code/experiments/hpe/extractors/network_setup.py
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const networkSetupPath = "/Network.html"

var (
	_ datasource.DataSource              = &networkSetupDataSource{}
	_ datasource.DataSourceWithConfigure = &networkSetupDataSource{}
)

func NewNetworkSetupDataSource() datasource.DataSource {
	return &networkSetupDataSource{}
}

type networkSetupDataSource struct {
	client *client.Client
}

type networkSetupModel struct {
	Tables types.Map `tfsdk:"tables"`
}

func (d *networkSetupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_setup"
}

func (d *networkSetupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the switch's Network Setup page: Network Details, Web Parameters, Management Access and SNMP, keyed by table caption.",
		Attributes: map[string]schema.Attribute{
			"tables": schema.MapAttribute{
				Computed:    true,
				ElementType: types.MapType{ElemType: types.StringType},
			},
		},
	}
}

func (d *networkSetupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *networkSetupDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tables, err := d.client.FetchTables(networkSetupPath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch Network Setup page", err.Error())
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

	state := networkSetupModel{Tables: tablesValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
