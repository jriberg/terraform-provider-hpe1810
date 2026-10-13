// Port of ~/code/experiments/hpe/extractors/trunk_status.py
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const trunkStatusPath = "/TrunkStatus.html"

var (
	_ datasource.DataSource              = &trunkStatusDataSource{}
	_ datasource.DataSourceWithConfigure = &trunkStatusDataSource{}
)

func NewTrunkStatusDataSource() datasource.DataSource {
	return &trunkStatusDataSource{}
}

type trunkStatusDataSource struct {
	client *client.Client
}

type trunkStatusModel struct {
	Trunks types.List `tfsdk:"trunks"`
}

func (d *trunkStatusDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trunk_status"
}

func (d *trunkStatusDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the switch's Trunk Status page: one entry per configured trunk. Empty list if no trunks exist.",
		Attributes: map[string]schema.Attribute{
			"trunks": schema.ListAttribute{
				Computed:    true,
				ElementType: types.MapType{ElemType: types.StringType},
			},
		},
	}
}

func (d *trunkStatusDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *trunkStatusDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tables, err := d.client.FetchTables(trunkStatusPath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch Trunk Status page", err.Error())
		return
	}
	table, ok := tables["Trunk Status"]
	if !ok {
		resp.Diagnostics.AddError("Unexpected page layout", `table captioned "Trunk Status" not found`)
		return
	}

	trunksValue, diags := rowsToTF(ctx, table.Rows)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := trunkStatusModel{Trunks: trunksValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
