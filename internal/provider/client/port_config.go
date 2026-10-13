// PortConfiguration.html is unlike every other page ported so far: it's a
// single-port form with an Interface selector (v_1_1_1), not a per-port
// table. Selecting a port is done with a "reload" POST (submit_flag=1, not
// the usual submit_flag=8) carrying v_1_1_1=<port> -- confirmed live against
// real ports (1 and 5) before any write code was written.
//
// The page also has three rows all captioned "Link Speed" (v_1_8_1,
// v_1_13_1, v_1_15_1 -- one per SFP/PHY type group). READS are fine via the
// generic xepage.go parser regardless (both ports probed showed all three
// mirroring the same displayed value). WRITES are NOT interchangeable,
// though -- confirmed live: submitting the same value to all three at once
// was rejected by the switch ("Error! Failed to Set 'Link Speed' ... error
// occured") because each field has its own server-side accepted-value
// enum (v_1_13_1 is 100FX-SFP-specific, v_1_15_1 is 1000Mbps-SFP-specific).
// Per _xe_PortConfiguration.js's xa_1_12_1 mapping table, "No SFP" (i.e.
// RJ45 copper -- confirmed via port_summary to be every port on this
// switch, an all-copper 1810G-24GE) maps to v_1_8_1 as the one real field;
// this client only ever writes that one and rejects any other Physical
// Type outright rather than guessing.
//
// The write also needs three additional hidden fields the read-only
// generic parser can't see (they're TRs with no defleft/defright, just a
// bare hidden <TD>, so parseTable's scalar-row logic skips them entirely):
// v_1_21_1 (UnitIndex), v_1_31_1 (duplicate of the interface), v_1_2_1
// (mstid). Omitting them was confirmed live to fail with "FILTER_MISSING"
// on Admin Mode. Both probed ports show v_1_21_1=v_1_2_1="1" always and
// v_1_31_1 mirroring the interface -- hardcoded accordingly rather than
// scraped, since this switch is a single, non-stacked unit.
package client

import "fmt"

const (
	portConfigPath          = "/PortConfiguration.html"
	portConfigCaption       = "Port Configuration"
	portInterfaceField      = "v_1_1_1"
	portUnitIndexField      = "v_1_21_1"
	portDupInterfaceField   = "v_1_31_1"
	portMstidField          = "v_1_2_1"
	portAdminModeField      = "v_1_5_1"
	portLinkSpeedFieldNoSFP = "v_1_8_1"
	portInterfaceLabel      = "Interface"
	portLinkStatusLabel     = "Link Status"
	portPhysicalTypeLabel   = "Physical Type"
	portAdminModeLabel      = "Admin Mode"
	portLinkSpeedLabel      = "Link Speed"
	portPhysicalTypeNoSFP   = "No SFP"
)

type PortConfig struct {
	Interface    string
	AdminMode    bool // true == Enable
	LinkSpeed    string
	PhysicalType string
	LinkStatus   string
}

// reloadPortConfig POSTs submit_flag=1 (reload, not apply) with
// v_1_1_1=interfaceNum to select that port, then parses the "Port
// Configuration" table out of the response the same way FetchTables would.
func (c *Client) reloadPortConfig(interfaceNum string) (Table, error) {
	target := "PortConfiguration.html"
	form := map[string]string{
		"submit_flag":       "1", // xui_operation_reload -- select the port, apply nothing
		"submit_target":     target,
		"err_flag":          "0",
		"err_msg":           "",
		"clazz_information": target,
		portInterfaceField:  interfaceNum,
	}
	body, err := c.postForm(portConfigPath, form)
	if err != nil {
		return Table{}, err
	}
	tables, err := ParseXETables(body)
	if err != nil {
		return Table{}, err
	}
	for _, t := range tables {
		if t.Caption == portConfigCaption {
			return t, nil
		}
	}
	return Table{}, fmt.Errorf("table captioned %q not found on %s", portConfigCaption, portConfigPath)
}

// ReadPortConfig returns the current configuration for one port.
func (c *Client) ReadPortConfig(interfaceNum string) (PortConfig, error) {
	table, err := c.reloadPortConfig(interfaceNum)
	if err != nil {
		return PortConfig{}, err
	}
	return PortConfig{
		Interface:    table.Record[portInterfaceLabel],
		AdminMode:    table.Record[portAdminModeLabel] == "Enable",
		LinkSpeed:    table.Record[portLinkSpeedLabel],
		PhysicalType: table.Record[portPhysicalTypeLabel],
		LinkStatus:   table.Record[portLinkStatusLabel],
	}, nil
}

// SetPortConfig applies Admin Mode and Link Speed to one port. Only
// supports "No SFP" (RJ45 copper) ports -- see the package doc comment.
func (c *Client) SetPortConfig(interfaceNum string, adminMode bool, linkSpeed string) error {
	current, err := c.reloadPortConfig(interfaceNum)
	if err != nil {
		return err
	}
	physicalType := current.Record[portPhysicalTypeLabel]
	if physicalType != portPhysicalTypeNoSFP {
		return fmt.Errorf("interface %q has Physical Type %q -- this resource only supports %q (RJ45 copper) ports", interfaceNum, physicalType, portPhysicalTypeNoSFP)
	}

	value := "Disable"
	if adminMode {
		value = "Enable"
	}
	fields := map[string]string{
		portInterfaceField:      interfaceNum,
		portUnitIndexField:      "1",
		portDupInterfaceField:   interfaceNum,
		portMstidField:          "1",
		portAdminModeField:      value,
		portLinkSpeedFieldNoSFP: linkSpeed,
	}
	_, err = c.SubmitForm(portConfigPath, fields)
	return err
}
