// Port of ~/code/experiments/hpe/extractors/dual_image_status.py
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const dualImageStatusPath = "/dual_image_status.html"

var (
	_ datasource.DataSource              = &dualImageStatusDataSource{}
	_ datasource.DataSourceWithConfigure = &dualImageStatusDataSource{}
)

func NewDualImageStatusDataSource() datasource.DataSource {
	return &dualImageStatusDataSource{}
}

type dualImageStatusDataSource struct {
	client *client.Client
}

type dualImageStatusModel struct {
	Units types.List `tfsdk:"units"`
}

func (d *dualImageStatusDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dual_image_status"
}

func (d *dualImageStatusDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the switch's Dual Image Status page: one entry per unit (Active image, Next-Active image). The page's separate \"Dual Image Descriptions\" table uses a header shape the generic parser doesn't handle and is skipped, matching the Python reference implementation.",
		Attributes: map[string]schema.Attribute{
			"units": schema.ListAttribute{
				Computed:    true,
				ElementType: types.MapType{ElemType: types.StringType},
			},
		},
	}
}

func (d *dualImageStatusDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *dualImageStatusDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tables, err := d.client.FetchTables(dualImageStatusPath)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch Dual Image Status page", err.Error())
		return
	}
	table, ok := tables["Dual Image Status"]
	if !ok {
		resp.Diagnostics.AddError("Unexpected page layout", `table captioned "Dual Image Status" not found`)
		return
	}

	unitsValue, diags := rowsToTF(ctx, table.Rows)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := dualImageStatusModel{Units: unitsValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
