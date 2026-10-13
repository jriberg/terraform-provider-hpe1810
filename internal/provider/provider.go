package provider

import (
	"context"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-hpe1810/internal/provider/client"
)

var _ provider.Provider = &hpe1810Provider{}

type hpe1810Provider struct {
	version           string
	client            *client.Client
	saveRunningConfig bool
}

// hpe1810ProviderModel mirrors config.py's three settings (SWITCH_HOST,
// SWITCH_HTTPS, SWITCH_PASSWORD) from the Python reference implementation,
// plus save_running_config which has no Python-side equivalent.
type hpe1810ProviderModel struct {
	Host              types.String `tfsdk:"host"`
	HTTPS             types.Bool   `tfsdk:"https"`
	Password          types.String `tfsdk:"password"`
	SaveRunningConfig types.Bool   `tfsdk:"save_running_config"`
	AdminPort         types.Int64  `tfsdk:"admin_port"`
}

// New returns a single provider instance (not a fresh one per call) so that
// main.go can hold onto it and call Shutdown() after providerserver.Serve
// returns -- see Shutdown() below.
func New(version string) *hpe1810Provider {
	return &hpe1810Provider{version: version}
}

// Shutdown runs once, when the provider process is exiting (main.go calls
// it right after providerserver.Serve returns). It optionally persists any
// changes made this run to the switch's non-volatile config, then always
// releases the session slot.
//
// The save only happens if save_running_config=true AND the client actually
// wrote something this run (client.Dirty()) -- a plan-only invocation, or a
// run where nothing changed, does not trigger a save. This matters because
// "Save Configuration" writes to flash, and doing that on every single
// `tofu` invocation regardless of whether anything changed would wear it
// out for no reason.
func (p *hpe1810Provider) Shutdown() {
	if p.client == nil {
		return
	}
	if p.saveRunningConfig && p.client.Dirty() {
		_ = p.client.SaveRunningConfig()
	}
	_ = p.client.Logout()
}

func (p *hpe1810Provider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "hpe1810"
	resp.Version = p.version
}

func (p *hpe1810Provider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an HP ProCurve/OfficeConnect 1810 switch over its built-in web UI.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Optional:    true,
				Description: "Switch IP or hostname. Falls back to the HPE1810_HOST env var, then \"192.168.2.10\".",
			},
			"https": schema.BoolAttribute{
				Optional:    true,
				Description: "Use HTTPS instead of HTTP. Falls back to the HPE1810_HTTPS env var, then false.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Switch login password. Falls back to the HPE1810_PASSWORD env var, then empty.",
			},
			"save_running_config": schema.BoolAttribute{
				Optional:    true,
				Description: "If true, persist changes to the switch's non-volatile config (\"Save Configuration\") at the end of this run, but only when this run actually wrote something. Falls back to the HPE1810_SAVE_RUNNING_CONFIG env var, then false. Leave false while iterating -- IaC already has the config as source of truth, and saving unnecessarily wears the switch's flash.",
			},
			"admin_port": schema.Int64Attribute{
				Optional:    true,
				Description: "The interface number hpe1810_port refuses to write to under any circumstances -- protects the switch's uplink/management port. Falls back to the HPE1810_ADMIN_PORT env var, then 24. An int, not a string, so it can't be set to a non-numeric value that would silently never match a real interface.",
			},
		},
	}
}

func (p *hpe1810Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config hpe1810ProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	host := os.Getenv("HPE1810_HOST")
	if host == "" {
		host = "192.168.2.10"
	}
	if !config.Host.IsNull() {
		host = config.Host.ValueString()
	}

	https := false
	if envHTTPS := os.Getenv("HPE1810_HTTPS"); envHTTPS != "" {
		https, _ = strconv.ParseBool(envHTTPS)
	}
	if !config.HTTPS.IsNull() {
		https = config.HTTPS.ValueBool()
	}

	password := os.Getenv("HPE1810_PASSWORD")
	if !config.Password.IsNull() {
		password = config.Password.ValueString()
	}

	saveRunningConfig := false
	if envSave := os.Getenv("HPE1810_SAVE_RUNNING_CONFIG"); envSave != "" {
		saveRunningConfig, _ = strconv.ParseBool(envSave)
	}
	if !config.SaveRunningConfig.IsNull() {
		saveRunningConfig = config.SaveRunningConfig.ValueBool()
	}
	p.saveRunningConfig = saveRunningConfig

	adminPort := int64(24)
	if envAdminPort := os.Getenv("HPE1810_ADMIN_PORT"); envAdminPort != "" {
		if parsed, err := strconv.ParseInt(envAdminPort, 10, 64); err == nil {
			adminPort = parsed
		}
	}
	if !config.AdminPort.IsNull() {
		adminPort = config.AdminPort.ValueInt64()
	}

	c, err := client.NewClient(host, https, password)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create switch client", err.Error())
		return
	}
	c.AdminPort = adminPort
	if err := c.Login(); err != nil {
		resp.Diagnostics.AddError("Unable to log in to switch", err.Error())
		return
	}

	p.client = c
	resp.DataSourceData = c
	resp.ResourceData = c
}

// DataSources: one entry per extractors/*.py equivalent.
func (p *hpe1810Provider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSystemDescriptionDataSource,
		NewNetworkSetupDataSource,
		NewPortSummaryDataSource,
		NewLLDPStatisticsDataSource,
		NewBufferedLogDataSource,
		NewMACTableDataSource,
		NewTrunkStatusDataSource,
		NewLoopProtectionStatusDataSource,
		NewDualImageStatusDataSource,
		NewClockDataSource,
		NewSNTPDataSource,
		NewLogConfigurationDataSource,
		NewBackupManagerDataSource,
	}
}

// Resources: one entry per actions/*.py equivalent, as they're ported.
func (p *hpe1810Provider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewLocatorResource,
		NewGreenFeaturesResource,
		NewPortResource,
		NewSystemDescriptionResource,
		NewTimeZoneResource,
		NewLLDPConfigurationResource,
		NewDaylightSavingTimeResource,
		NewFlowControlResource,
		NewPortMirroringResource,
		NewTrunkResource,
		NewSNTPResource,
	}
}
