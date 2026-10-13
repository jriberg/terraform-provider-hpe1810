// Port of ~/code/experiments/hpe/extractors/loop_protection_status.py
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const loopProtectionStatusPath = "/LoopProtectionStatus.html"

var (
	_ datasource.DataSource              = &loopProtectionStatusDataSource{}
	_ datasource.DataSourceWithConfigure = &loopProtectionStatusDataSource{}
)

func NewLoopProtectionStatusDataSource() datasource.DataSource {
	return &loopProtectionStatusDataSource{}
}

type loopProtectionStatusDataSource struct {
	client *client.Client
}

type loopProtectionStatusModel struct {
	Ports types.List `tfsdk:"ports"`
}

func (d *loopProtectionStatusDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_loop_protection_status"
}

func (d *loopProtectionStatusDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the switch's Loop Protection Status page: one entry per port (Interface, Configured Action Taken, Protection Feature Mode, Tx Mode, counters, Loop Detected).",
		Attributes: map[string]schema.Attribute{
			"ports": schema.ListAttribute{
				Computed:    true,
				ElementType: types.MapType{ElemType: types.StringType},
			},
		},
	}
}

func (d *loopProtectionStatusDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *loopProtectionStatusDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tables, err := d.client.FetchTables(loopProtectionStatusPath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch Loop Protection Status page", err.Error())
		return
	}
	table, ok := tables["Loop Protection Status"]
	if !ok {
		resp.Diagnostics.AddError("Unexpected page layout", `table captioned "Loop Protection Status" not found`)
		return
	}

	portsValue, diags := rowsToTF(ctx, table.Rows)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := loopProtectionStatusModel{Ports: portsValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
