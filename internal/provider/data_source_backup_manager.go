// Port of ~/code/experiments/hpe/extractors/backup_manager.py
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const backupManagerPath = "/UploadFile.html"

var (
	_ datasource.DataSource              = &backupManagerDataSource{}
	_ datasource.DataSourceWithConfigure = &backupManagerDataSource{}
)

func NewBackupManagerDataSource() datasource.DataSource {
	return &backupManagerDataSource{}
}

type backupManagerDataSource struct {
	client *client.Client
}

type backupManagerModel struct {
	Values types.Map `tfsdk:"values"`
}

func (d *backupManagerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_manager"
}

func (d *backupManagerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the switch's Backup/Upload File page's current form fields (method, server address, etc.). This is a config form, not a data table, so values reflect whatever was last configured or left blank.",
		Attributes: map[string]schema.Attribute{
			"values": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *backupManagerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *backupManagerDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tables, err := d.client.FetchTables(backupManagerPath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch Backup page", err.Error())
		return
	}
	table, ok := tables["Backup"]
	if !ok {
		resp.Diagnostics.AddError("Unexpected page layout", `table captioned "Backup" not found`)
		return
	}

	valuesValue, diags := stringMapToTF(ctx, table.Record)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := backupManagerModel{Values: valuesValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
