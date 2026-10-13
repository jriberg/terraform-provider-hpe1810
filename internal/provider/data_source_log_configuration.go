// Port of ~/code/experiments/hpe/extractors/log_configuration.py
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const logConfigurationPath = "/Logging.html"

var (
	_ datasource.DataSource              = &logConfigurationDataSource{}
	_ datasource.DataSourceWithConfigure = &logConfigurationDataSource{}
)

func NewLogConfigurationDataSource() datasource.DataSource {
	return &logConfigurationDataSource{}
}

type logConfigurationDataSource struct {
	client *client.Client
}

type logConfigurationModel struct {
	Values types.Map `tfsdk:"values"`
}

func (d *logConfigurationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_log_configuration"
}

func (d *logConfigurationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the switch's Logging Configuration page.",
		Attributes: map[string]schema.Attribute{
			"values": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *logConfigurationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *logConfigurationDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tables, err := d.client.FetchTables(logConfigurationPath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch Logging page", err.Error())
		return
	}
	table, ok := tables["Logging"]
	if !ok {
		resp.Diagnostics.AddError("Unexpected page layout", `table captioned "Logging" not found`)
		return
	}

	valuesValue, diags := stringMapToTF(ctx, table.Record)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := logConfigurationModel{Values: valuesValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
