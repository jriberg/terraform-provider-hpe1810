// Port of ~/code/experiments/hpe/extractors/lldp_statistics.py
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const lldpStatisticsPath = "/LLDPStats.html"

var (
	_ datasource.DataSource              = &lldpStatisticsDataSource{}
	_ datasource.DataSourceWithConfigure = &lldpStatisticsDataSource{}
)

func NewLLDPStatisticsDataSource() datasource.DataSource {
	return &lldpStatisticsDataSource{}
}

type lldpStatisticsDataSource struct {
	client *client.Client
}

type lldpStatisticsModel struct {
	Global     types.Map  `tfsdk:"global"`
	Interfaces types.List `tfsdk:"interfaces"`
}

func (d *lldpStatisticsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lldp_statistics"
}

func (d *lldpStatisticsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the switch's LLDP Statistics page: global counters plus one entry per interface.",
		Attributes: map[string]schema.Attribute{
			"global": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
			"interfaces": schema.ListAttribute{
				Computed:    true,
				ElementType: types.MapType{ElemType: types.StringType},
			},
		},
	}
}

func (d *lldpStatisticsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *lldpStatisticsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tables, err := d.client.FetchTables(lldpStatisticsPath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch LLDP Statistics page", err.Error())
		return
	}
	global, ok := tables["LLDP Global Statistics"]
	if !ok {
		resp.Diagnostics.AddError("Unexpected page layout", `table captioned "LLDP Global Statistics" not found`)
		return
	}
	interfaces, ok := tables["LLDP Interface Statistics"]
	if !ok {
		resp.Diagnostics.AddError("Unexpected page layout", `table captioned "LLDP Interface Statistics" not found`)
		return
	}

	globalValue, diags := stringMapToTF(ctx, global.Record)
	resp.Diagnostics.Append(diags...)
	interfacesValue, diags := rowsToTF(ctx, interfaces.Rows)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := lldpStatisticsModel{Global: globalValue, Interfaces: interfacesValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
