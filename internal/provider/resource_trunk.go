// Port of TrunkConfig.html + TrunkMembership.html -- see
// client/trunk.go's package doc comment for the full write-protocol
// writeup. Unlike every other resource so far (singletons, or a fixed
// for_each set of 24 ports), a trunk is a genuine named sub-entity: create,
// assign members, delete.
//
// admin_mode and static_capability are exposed read-only (Computed) only --
// only Create/set-members/Delete were verified live; toggling those two via
// the row-based "Modify" mechanism was not tested, so this resource doesn't
// claim to control them.
package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

var (
	_ resource.Resource              = &trunkResource{}
	_ resource.ResourceWithConfigure = &trunkResource{}
)

func NewTrunkResource() resource.Resource {
	return &trunkResource{}
}

type trunkResource struct {
	client *client.Client
}

type trunkResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Members          types.Set    `tfsdk:"members"`
	AdminMode        types.Bool   `tfsdk:"admin_mode"`
	StaticCapability types.Bool   `tfsdk:"static_capability"`
}

func (r *trunkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trunk"
}

func (r *trunkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates a link aggregation trunk (LAG) and sets its member ports (TrunkConfig.html + TrunkMembership.html). The provider's admin_port (default 24) can never be included in members. Delete removes the trunk entirely.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Switch-assigned identifier, e.g. \"Trunk1\".",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "1-15 alphanumeric characters. Changing this replaces the resource (trunks aren't renamed in place).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"members": schema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Port numbers to include in this trunk, e.g. [\"5\", \"6\"]. Never include the admin_port.",
			},
			"admin_mode": schema.BoolAttribute{
				Computed:    true,
				Description: "Read-only -- reflects the switch's current value, not settable by this resource (defaults to Enable on creation).",
			},
			"static_capability": schema.BoolAttribute{
				Computed:    true,
				Description: "Read-only -- reflects the switch's current value, not settable by this resource (defaults to Disable, i.e. LACP/dynamic, on creation).",
			},
		},
	}
}

func (r *trunkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected resource configure type", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *trunkResource) guardAdminPort(members []string) error {
	for _, m := range members {
		n, err := strconv.Atoi(m)
		if err != nil {
			return fmt.Errorf("member %q is not a valid port number: %w", m, err)
		}
		if int64(n) == r.client.AdminPort {
			return fmt.Errorf("refusing to include interface %q in a trunk: it's the provider's admin_port (%d), protected from all writes by this resource", m, r.client.AdminPort)
		}
	}
	return nil
}

func trunkToModel(t client.Trunk, membersSet types.Set) trunkResourceModel {
	return trunkResourceModel{
		ID:               types.StringValue(t.ID),
		Name:             types.StringValue(t.Name),
		Members:          membersSet,
		AdminMode:        types.BoolValue(t.AdminMode),
		StaticCapability: types.BoolValue(t.StaticCapability),
	}
}

func (r *trunkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan trunkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var members []string
	resp.Diagnostics.Append(plan.Members.ElementsAs(ctx, &members, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.guardAdminPort(members); err != nil {
		resp.Diagnostics.AddError("Refusing to write to admin_port", err.Error())
		return
	}

	trunk, err := r.client.CreateTrunk(plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to create trunk", err.Error())
		return
	}

	memberSet := make(map[string]bool, len(members))
	for _, m := range members {
		memberSet[m] = true
	}
	if err := r.client.SetTrunkMembers(trunk.ID, memberSet); err != nil {
		resp.Diagnostics.AddError("Unable to set trunk members", err.Error())
		return
	}

	final, _, err := r.client.FindTrunkByName(plan.Name.ValueString())
	if err != nil || final == nil {
		resp.Diagnostics.AddError("Unable to read back trunk", fmt.Sprintf("err=%v, found=%v", err, final != nil))
		return
	}

	membersValue, diags := types.SetValueFrom(ctx, types.StringType, final.Members)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := trunkToModel(*final, membersValue)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *trunkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state trunkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	trunk, _, err := r.client.FindTrunkByName(state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read trunk", err.Error())
		return
	}
	if trunk == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	membersValue, diags := types.SetValueFrom(ctx, types.StringType, trunk.Members)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState := trunkToModel(*trunk, membersValue)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *trunkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan trunkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state trunkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var members []string
	resp.Diagnostics.Append(plan.Members.ElementsAs(ctx, &members, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.guardAdminPort(members); err != nil {
		resp.Diagnostics.AddError("Refusing to write to admin_port", err.Error())
		return
	}

	trunkID := state.ID.ValueString()
	memberSet := make(map[string]bool, len(members))
	for _, m := range members {
		memberSet[m] = true
	}
	if err := r.client.SetTrunkMembers(trunkID, memberSet); err != nil {
		resp.Diagnostics.AddError("Unable to set trunk members", err.Error())
		return
	}

	final, _, err := r.client.FindTrunkByName(plan.Name.ValueString())
	if err != nil || final == nil {
		resp.Diagnostics.AddError("Unable to read back trunk", fmt.Sprintf("err=%v, found=%v", err, final != nil))
		return
	}

	membersValue, diags := types.SetValueFrom(ctx, types.StringType, final.Members)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState := trunkToModel(*final, membersValue)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// Delete removes the trunk entirely -- there's no meaningful "safe default"
// to reset a trunk to; a LAG either exists or it doesn't.
func (r *trunkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state trunkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteTrunk(state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete trunk", err.Error())
	}
}
