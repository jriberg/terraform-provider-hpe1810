// Port of ~/code/experiments/hpe/extractors/port_summary.py
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const portSummaryPath = "/PortSummary.html"

var (
	_ datasource.DataSource              = &portSummaryDataSource{}
	_ datasource.DataSourceWithConfigure = &portSummaryDataSource{}
)

func NewPortSummaryDataSource() datasource.DataSource {
	return &portSummaryDataSource{}
}

type portSummaryDataSource struct {
	client *client.Client
}

type portSummaryModel struct {
	Ports types.List `tfsdk:"ports"`
}

func (d *portSummaryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_port_summary"
}

func (d *portSummaryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the switch's Port Summary page: one entry per port (Interface, Physical Type, Port Status, AutoNeg Status, Link Speed, MTU).",
		Attributes: map[string]schema.Attribute{
			"ports": schema.ListAttribute{
				Computed:    true,
				ElementType: types.MapType{ElemType: types.StringType},
			},
		},
	}
}

func (d *portSummaryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *portSummaryDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tables, err := d.client.FetchTables(portSummaryPath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch Port Summary page", err.Error())
		return
	}
	table, ok := tables["Port Summary"]
	if !ok {
		resp.Diagnostics.AddError("Unexpected page layout", `table captioned "Port Summary" not found`)
		return
	}

	portsValue, diags := rowsToTF(ctx, table.Rows)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := portSummaryModel{Ports: portsValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
