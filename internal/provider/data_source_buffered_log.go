// Port of ~/code/experiments/hpe/extractors/buffered_log.py
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const bufferedLogPath = "/BufferedLogs.html"

var (
	_ datasource.DataSource              = &bufferedLogDataSource{}
	_ datasource.DataSourceWithConfigure = &bufferedLogDataSource{}
)

func NewBufferedLogDataSource() datasource.DataSource {
	return &bufferedLogDataSource{}
}

type bufferedLogDataSource struct {
	client *client.Client
}

type bufferedLogModel struct {
	Entries types.List `tfsdk:"entries"`
}

func (d *bufferedLogDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_buffered_log"
}

func (d *bufferedLogDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the switch's Buffered Log page: one entry per log message (Log Index, Severity, Log Time, Component, Description, Message Text).",
		Attributes: map[string]schema.Attribute{
			"entries": schema.ListAttribute{
				Computed:    true,
				ElementType: types.MapType{ElemType: types.StringType},
			},
		},
	}
}

func (d *bufferedLogDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *bufferedLogDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tables, err := d.client.FetchTables(bufferedLogPath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch Buffered Log page", err.Error())
		return
	}
	table, ok := tables["Log Message"]
	if !ok {
		resp.Diagnostics.AddError("Unexpected page layout", `table captioned "Log Message" not found`)
		return
	}

	entriesValue, diags := rowsToTF(ctx, table.Rows)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := bufferedLogModel{Entries: entriesValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
