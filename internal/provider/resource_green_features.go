// First resource ported straight from a saved snapshot (no Python
// extractors/actions equivalent existed yet) -- see
// ~/code/experiments/hpe/page_snapshots/greenmode.html.html. Safest write
// target after Locator per AGENT.md's risk triage: EEE power-saving, no
// traffic impact.
//
// The page has two tables in one <FORM>: "Green Features Configuration"
// (Green Mode, Mode LED Time) and "Phy Auto Power-Down" (Mode). All three
// fields are always submitted together on any change -- the firmware's form
// includes all of them at once in a real browser submission, and there's no
// evidence omitted fields are left alone rather than reset, so this plays it
// safe and always sends current-or-desired values for all three.
package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const (
	greenFeaturesPath           = "/greenmode.html"
	greenFeaturesCaption        = "Green Features Configuration"
	greenFeaturesGreenModeField = "Green Mode"
	greenFeaturesLEDTimeField   = "Mode LED Time"
	phyAutoPowerDownCaption     = "Phy Auto Power-Down"
	phyAutoPowerDownModeField   = "Mode"
)

var (
	_ resource.Resource              = &greenFeaturesResource{}
	_ resource.ResourceWithConfigure = &greenFeaturesResource{}
)

func NewGreenFeaturesResource() resource.Resource {
	return &greenFeaturesResource{}
}

type greenFeaturesResource struct {
	client *client.Client
}

type greenFeaturesResourceModel struct {
	ID               types.String `tfsdk:"id"`
	GreenMode        types.Bool   `tfsdk:"green_mode"`
	ModeLEDTime      types.Int64  `tfsdk:"mode_led_time"`
	PhyAutoPowerDown types.Bool   `tfsdk:"phy_auto_power_down"`
}

func (r *greenFeaturesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_green_features"
}

func (r *greenFeaturesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Controls the switch's Green Features (EEE power-saving) page. Singleton -- there is only ever one of these per switch. Delete resets all three fields to their documented factory defaults (Green Mode/Phy Auto Power-Down Enable, Mode LED Time 10).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always \"green_features\" -- this is a singleton resource.",
			},
			"green_mode": schema.BoolAttribute{
				Required:    true,
				Description: "Whether EEE (Energy Efficient Ethernet) is enabled.",
			},
			"mode_led_time": schema.Int64Attribute{
				Required:    true,
				Description: "Minutes of link-up activity before the port LED dims in green mode. Range 1-30 (switch default: 10).",
			},
			"phy_auto_power_down": schema.BoolAttribute{
				Required:    true,
				Description: "Whether PHY Auto Power-Down (link-partner-absent power saving) is enabled.",
			},
		},
	}
}

func (r *greenFeaturesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type greenFeaturesState struct {
	GreenMode        bool
	ModeLEDTime      int64
	PhyAutoPowerDown bool
}

func (r *greenFeaturesResource) readState() (greenFeaturesState, error) {
	tables, err := r.client.FetchTables(greenFeaturesPath)
	if err != nil {
		return greenFeaturesState{}, err
	}
	green, ok := tables[greenFeaturesCaption]
	if !ok {
		return greenFeaturesState{}, fmt.Errorf("table captioned %q not found on %s", greenFeaturesCaption, greenFeaturesPath)
	}
	phy, ok := tables[phyAutoPowerDownCaption]
	if !ok {
		return greenFeaturesState{}, fmt.Errorf("table captioned %q not found on %s", phyAutoPowerDownCaption, greenFeaturesPath)
	}

	ledTime, err := strconv.ParseInt(green.Record[greenFeaturesLEDTimeField], 10, 64)
	if err != nil {
		return greenFeaturesState{}, fmt.Errorf("parsing %q as int: %w", green.Record[greenFeaturesLEDTimeField], err)
	}

	return greenFeaturesState{
		GreenMode:        green.Record[greenFeaturesGreenModeField] == "Enable",
		ModeLEDTime:      ledTime,
		PhyAutoPowerDown: phy.Record[phyAutoPowerDownModeField] == "Enable",
	}, nil
}

func (r *greenFeaturesResource) setState(desired greenFeaturesState) error {
	tables, err := r.client.FetchTables(greenFeaturesPath)
	if err != nil {
		return err
	}
	green, ok := tables[greenFeaturesCaption]
	if !ok {
		return fmt.Errorf("table captioned %q not found on %s", greenFeaturesCaption, greenFeaturesPath)
	}
	phy, ok := tables[phyAutoPowerDownCaption]
	if !ok {
		return fmt.Errorf("table captioned %q not found on %s", phyAutoPowerDownCaption, greenFeaturesPath)
	}

	greenModeField, ok := green.FieldNames[greenFeaturesGreenModeField]
	if !ok {
		return fmt.Errorf("field %q not found in table %q", greenFeaturesGreenModeField, greenFeaturesCaption)
	}
	ledTimeField, ok := green.FieldNames[greenFeaturesLEDTimeField]
	if !ok {
		return fmt.Errorf("field %q not found in table %q", greenFeaturesLEDTimeField, greenFeaturesCaption)
	}
	phyModeField, ok := phy.FieldNames[phyAutoPowerDownModeField]
	if !ok {
		return fmt.Errorf("field %q not found in table %q", phyAutoPowerDownModeField, phyAutoPowerDownCaption)
	}

	enableDisable := func(b bool) string {
		if b {
			return "Enable"
		}
		return "Disable"
	}

	_, err = r.client.SubmitForm(greenFeaturesPath, map[string]string{
		greenModeField: enableDisable(desired.GreenMode),
		ledTimeField:   strconv.FormatInt(desired.ModeLEDTime, 10),
		phyModeField:   enableDisable(desired.PhyAutoPowerDown),
	})
	return err
}

func stateToModel(s greenFeaturesState) greenFeaturesResourceModel {
	return greenFeaturesResourceModel{
		ID:               types.StringValue("green_features"),
		GreenMode:        types.BoolValue(s.GreenMode),
		ModeLEDTime:      types.Int64Value(s.ModeLEDTime),
		PhyAutoPowerDown: types.BoolValue(s.PhyAutoPowerDown),
	}
}

func (r *greenFeaturesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan greenFeaturesResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired := greenFeaturesState{
		GreenMode:        plan.GreenMode.ValueBool(),
		ModeLEDTime:      plan.ModeLEDTime.ValueInt64(),
		PhyAutoPowerDown: plan.PhyAutoPowerDown.ValueBool(),
	}
	if err := r.setState(desired); err != nil {
		resp.Diagnostics.AddError("Unable to set Green Features state", err.Error())
		return
	}

	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back Green Features state", err.Error())
		return
	}

	state := stateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *greenFeaturesResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Green Features state", err.Error())
		return
	}

	state := stateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *greenFeaturesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan greenFeaturesResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired := greenFeaturesState{
		GreenMode:        plan.GreenMode.ValueBool(),
		ModeLEDTime:      plan.ModeLEDTime.ValueInt64(),
		PhyAutoPowerDown: plan.PhyAutoPowerDown.ValueBool(),
	}
	if err := r.setState(desired); err != nil {
		resp.Diagnostics.AddError("Unable to set Green Features state", err.Error())
		return
	}

	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back Green Features state", err.Error())
		return
	}

	state := stateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete resets all three fields to their documented factory defaults --
// safe here since this is a pure power-saving feature with no traffic
// impact (Safe tier in AGENT.md's risk triage), unlike e.g. Network Setup
// where touching anything on delete would be the risky move.
func (r *greenFeaturesResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	defaults := greenFeaturesState{GreenMode: true, ModeLEDTime: 10, PhyAutoPowerDown: true}
	if err := r.setState(defaults); err != nil {
		resp.Diagnostics.AddError("Unable to reset Green Features state on delete", err.Error())
	}
}
