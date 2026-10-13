// Port of ~/code/experiments/hpe/extractors/mac_table.py
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const macTablePath = "/FDBSearch.html"

var (
	_ datasource.DataSource              = &macTableDataSource{}
	_ datasource.DataSourceWithConfigure = &macTableDataSource{}
)

func NewMACTableDataSource() datasource.DataSource {
	return &macTableDataSource{}
}

type macTableDataSource struct {
	client *client.Client
}

type macTableModel struct {
	Summary types.Map  `tfsdk:"summary"`
	Entries types.List `tfsdk:"entries"`
}

func (d *macTableDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mac_table"
}

func (d *macTableDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the switch's MAC Address Table (FDB Search) page: summary counters plus one entry per learned MAC. The per-entry table has a blank <CAPTION>, so it's looked up by table id (\"1_1\") instead of caption.",
		Attributes: map[string]schema.Attribute{
			"summary": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
			"entries": schema.ListAttribute{
				Computed:    true,
				ElementType: types.MapType{ElemType: types.StringType},
			},
		},
	}
}

func (d *macTableDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *macTableDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tables, err := d.client.FetchAllTables(macTablePath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch MAC Table page", err.Error())
		return
	}

	var summary map[string]string
	var entries []map[string]string
	for _, t := range tables {
		if t.Caption == "MAC Table" {
			summary = t.Record
		}
		if t.ID == "1_1" {
			entries = t.Rows
		}
	}
	if summary == nil {
		resp.Diagnostics.AddError("Unexpected page layout", `table captioned "MAC Table" not found`)
		return
	}
	if entries == nil {
		resp.Diagnostics.AddError("Unexpected page layout", `table with id "1_1" not found`)
		return
	}

	summaryValue, diags := stringMapToTF(ctx, summary)
	resp.Diagnostics.Append(diags...)
	entriesValue, diags := rowsToTF(ctx, entries)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := macTableModel{Summary: summaryValue, Entries: entriesValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
