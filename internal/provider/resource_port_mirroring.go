// Port of FDBConfig.html's "Port Mirroring Configuration" table only (the
// page's filename is misleading -- it's actually Port Mirroring, not FDB).
// The page also has a per-source-port "Direction" table (25 rows: ports
// 1-24 + CPU, per-port field names like "1.0.25.v_1_3_2") -- same shape of
// complexity as LLDPConfig.html's per-port Interface Mode table, and
// likewise deferred as a separate not-yet-built write target.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

const (
	portMirroringPath             = "/FDBConfig.html"
	portMirroringCaption          = "Port Mirroring Configuration"
	portMirroringEnableLabel      = "Enable Mirroring"
	portMirroringDestinationLabel = "Destination Port"
	// portMirroringHiddenContextField (v_2_2_2, always "1") sits next to
	// Destination Port in the DOM with no visible label -- same
	// "FILTER_MISSING if omitted" pattern as PortConfiguration.html's
	// hidden UnitIndex/mstid fields. Confirmed live: omitting it fails with
	// "Error! Failed to Set 'Enable Mirroring' ... error occured
	// FILTER_MISSING".
	portMirroringHiddenContextField = "v_2_2_2"
)

var (
	_ resource.Resource              = &portMirroringResource{}
	_ resource.ResourceWithConfigure = &portMirroringResource{}
)

func NewPortMirroringResource() resource.Resource {
	return &portMirroringResource{}
}

type portMirroringResource struct {
	client *client.Client
}

type portMirroringResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	DestinationPort types.String `tfsdk:"destination_port"`
}

func (r *portMirroringResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_port_mirroring"
}

func (r *portMirroringResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Controls the switch's global Port Mirroring settings (FDBConfig.html). Singleton -- there is only ever one of these per switch. Does NOT cover the page's per-source-port Direction table -- see the package doc comment. Delete disables mirroring and clears the destination port.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always \"port_mirroring\" -- this is a singleton resource.",
			},
			"enabled": schema.BoolAttribute{
				Required:    true,
				Description: "Whether port mirroring is enabled.",
			},
			"destination_port": schema.StringAttribute{
				Required:    true,
				Description: "\"None\", or a port number \"1\"..\"24\" to receive mirrored traffic.",
			},
		},
	}
}

func (r *portMirroringResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type portMirroringState struct {
	Enabled         bool
	DestinationPort string
}

func (r *portMirroringResource) readState() (portMirroringState, error) {
	tables, err := r.client.FetchTables(portMirroringPath)
	if err != nil {
		return portMirroringState{}, err
	}
	table, ok := tables[portMirroringCaption]
	if !ok {
		return portMirroringState{}, fmt.Errorf("table captioned %q not found on %s", portMirroringCaption, portMirroringPath)
	}
	return portMirroringState{
		Enabled:         table.Record[portMirroringEnableLabel] == "Enable",
		DestinationPort: table.Record[portMirroringDestinationLabel],
	}, nil
}

func (r *portMirroringResource) setState(desired portMirroringState) error {
	tables, err := r.client.FetchTables(portMirroringPath)
	if err != nil {
		return err
	}
	table, ok := tables[portMirroringCaption]
	if !ok {
		return fmt.Errorf("table captioned %q not found on %s", portMirroringCaption, portMirroringPath)
	}
	enableField, ok := table.FieldNames[portMirroringEnableLabel]
	if !ok {
		return fmt.Errorf("field %q not found in table %q", portMirroringEnableLabel, portMirroringCaption)
	}
	destField, ok := table.FieldNames[portMirroringDestinationLabel]
	if !ok {
		return fmt.Errorf("field %q not found in table %q", portMirroringDestinationLabel, portMirroringCaption)
	}

	value := "Disable"
	if desired.Enabled {
		value = "Enable"
	}
	_, err = r.client.SubmitForm(portMirroringPath, map[string]string{
		enableField:                     value,
		destField:                       desired.DestinationPort,
		portMirroringHiddenContextField: "1",
	})
	return err
}

func portMirroringStateToModel(s portMirroringState) portMirroringResourceModel {
	return portMirroringResourceModel{
		ID:              types.StringValue("port_mirroring"),
		Enabled:         types.BoolValue(s.Enabled),
		DestinationPort: types.StringValue(s.DestinationPort),
	}
}

func (r *portMirroringResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan portMirroringResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired := portMirroringState{Enabled: plan.Enabled.ValueBool(), DestinationPort: plan.DestinationPort.ValueString()}
	if err := r.setState(desired); err != nil {
		resp.Diagnostics.AddError("Unable to set Port Mirroring state", err.Error())
		return
	}

	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back Port Mirroring state", err.Error())
		return
	}

	state := portMirroringStateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *portMirroringResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Port Mirroring state", err.Error())
		return
	}

	state := portMirroringStateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *portMirroringResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan portMirroringResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired := portMirroringState{Enabled: plan.Enabled.ValueBool(), DestinationPort: plan.DestinationPort.ValueString()}
	if err := r.setState(desired); err != nil {
		resp.Diagnostics.AddError("Unable to set Port Mirroring state", err.Error())
		return
	}

	actual, err := r.readState()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read back Port Mirroring state", err.Error())
		return
	}

	state := portMirroringStateToModel(actual)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete disables mirroring and clears the destination port -- matches the
// switch's observed factory default, harmless.
func (r *portMirroringResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	if err := r.setState(portMirroringState{Enabled: false, DestinationPort: "None"}); err != nil {
		resp.Diagnostics.AddError("Unable to reset Port Mirroring state on delete", err.Error())
	}
}
