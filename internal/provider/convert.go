// Shared helpers for turning client.Table pieces (plain Go maps/slices) into
// the typed map/list values the Plugin Framework wants in state.
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var rowType = types.MapType{ElemType: types.StringType}

func stringMapToTF(ctx context.Context, m map[string]string) (types.Map, diag.Diagnostics) {
	return types.MapValueFrom(ctx, types.StringType, m)
}

func rowsToTF(ctx context.Context, rows []map[string]string) (types.List, diag.Diagnostics) {
	return types.ListValueFrom(ctx, rowType, rows)
}

func tableOfRecordsToTF(ctx context.Context, tables map[string]map[string]string) (types.Map, diag.Diagnostics) {
	return types.MapValueFrom(ctx, types.MapType{ElemType: types.StringType}, tables)
}
