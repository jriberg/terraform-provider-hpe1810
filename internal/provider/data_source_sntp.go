// Port of ~/code/experiments/hpe/extractors/sntp.py
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const sntpPath = "/SNTP.html"

var (
	_ datasource.DataSource              = &sntpDataSource{}
	_ datasource.DataSourceWithConfigure = &sntpDataSource{}
)

func NewSNTPDataSource() datasource.DataSource {
	return &sntpDataSource{}
}

type sntpDataSource struct {
	client *client.Client
}

type sntpModel struct {
	Values types.Map `tfsdk:"values"`
}

func (d *sntpDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sntp"
}

func (d *sntpDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the switch's SNTP Configuration page.",
		Attributes: map[string]schema.Attribute{
			"values": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *sntpDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sntpDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tables, err := d.client.FetchTables(sntpPath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch SNTP page", err.Error())
		return
	}
	table, ok := tables["SNTP Configuration"]
	if !ok {
		resp.Diagnostics.AddError("Unexpected page layout", `table captioned "SNTP Configuration" not found`)
		return
	}

	valuesValue, diags := stringMapToTF(ctx, table.Record)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := sntpModel{Values: valuesValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
