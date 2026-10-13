// Trunking (link aggregation / LAG) spans two pages with a genuinely
// different shape from everything else ported so far: TrunkConfig.html is a
// dynamic list of named entities (create/delete trunks, each gets a
// switch-assigned "TrunkN" id), and TrunkMembership.html assigns member
// ports to one of them. Every other resource so far is a singleton or a
// fixed for_each set (24 ports); this is the first with real create/delete
// of sub-entities.
//
// All three operations (Create, set members, Delete) were verified live
// before being wired into a Terraform resource, using a throwaway test
// trunk on ports 5 and 6 (never port 24), cleaned up afterward:
//
//   - Create: POST TrunkConfig.html with v_1_1_1=Enable (the "Create"
//     field), v_1_2_1=<name>, v_1_4_1=Add. The switch assigns the next
//     "TrunkN" id automatically.
//   - Set members: POST TrunkMembership.html with the selected trunk id in
//     v_1_1_1, plus ALL 24 ports' v_2_3_1 (port number), v_2_3_2 (per-port
//     LACP mode), v_2_3_3 (membership: the trunk id string if a member,
//     "1" if not -- the exact sentinel the page's own JS uses for "off").
//     Two gotchas confirmed live: (1) the page's other hidden context
//     fields (v_1_2_1, v_1_2_2, v_1_3_1, v_5_1_1, v_6_1_1, v_2_1_2, v_2_1_3,
//     v_2_1_4, v_3_1_1, v_3_2_1, v_3_3_1, v_3_4_1) must be resubmitted
//     unchanged or the switch rejects with FILTER_MISSING; (2) v_2_3_1 is
//     marked DISABLED in the raw HTML (a real browser wouldn't normally
//     submit a disabled field), but the page's own JS
//     (lagViewAutoGen.js's showvalues()) explicitly un-disables it right
//     before submit -- omitting it also fails with FILTER_MISSING.
//   - Delete: POST TrunkConfig.html with that trunk's row fields
//     (v_1_5_1..v_1_5_6) resubmitted unchanged, plus v_1_5_7=Enable (the
//     "Delete" trigger) and v_1_5_8=Delete (a fixed action-name constant).
//     Omitting the row's current-value fields fails with "Error! Failed to
//     remove LAG." Critically, including the page's top-level Create
//     section (v_1_1_1/v_1_2_1/v_1_4_1) in the SAME request as a delete
//     causes the switch to create ANOTHER trunk as a side effect instead --
//     confirmed live (deleting "Trunk1" while resending v_1_4_1=Add
//     produced a brand new empty "Trunk2"). So a delete request must omit
//     those fields entirely.
//
// The per-row/per-trunk field name prefix ("1.<index>.1.") uses this
// trunk's zero-based position in ListTrunks()'s order, the same positional
// pattern used throughout this firmware (PortConfiguration, Port
// Mirroring, LLDP, Trunk Membership). Verified live only for the
// single-trunk case (index 0); multi-trunk indexing is inferred by pattern,
// not independently confirmed against a live switch with 2+ trunks.
package client

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	trunkConfigPath     = "/TrunkConfig.html"
	trunkMembershipPath = "/TrunkMembership.html"
	trunkSummaryTableID = "1_5"

	trunkInterfaceLabel = "Interface"
	trunkNameLabel      = "Trunk Name"
	trunkMembersLabel   = "Trunk Members"
	trunkAdminModeLabel = "Admin Mode"
	trunkStaticCapLabel = "Static Capability"

	trunkCreateField    = "v_1_1_1"
	trunkNameField      = "v_1_2_1"
	trunkAddActionField = "v_1_4_1"

	trunkRowInterfaceField    = "v_1_5_1"
	trunkRowNameField         = "v_1_5_2"
	trunkRowMembersField      = "v_1_5_5"
	trunkRowAdminField        = "v_1_5_3"
	trunkRowStaticField       = "v_1_5_4"
	trunkRowModifyField       = "v_1_5_6"
	trunkRowDeleteField       = "v_1_5_7"
	trunkRowDeleteActionField = "v_1_5_8"
)

type Trunk struct {
	ID               string
	Name             string
	Members          []string
	AdminMode        bool
	StaticCapability bool
}

// ListTrunks returns every configured trunk, in the switch's own display
// order -- that order also determines the zero-based row index used in
// per-row field names, needed by DeleteTrunk.
func (c *Client) ListTrunks() ([]Trunk, error) {
	tables, err := c.FetchAllTables(trunkConfigPath)
	if err != nil {
		return nil, err
	}
	var summary *Table
	for i := range tables {
		if tables[i].ID == trunkSummaryTableID {
			summary = &tables[i]
			break
		}
	}
	if summary == nil {
		return nil, fmt.Errorf("table id %q not found on %s", trunkSummaryTableID, trunkConfigPath)
	}

	trunks := make([]Trunk, 0, len(summary.Rows))
	for _, row := range summary.Rows {
		var members []string
		if m := row[trunkMembersLabel]; m != "" {
			members = strings.Split(m, ",")
		}
		trunks = append(trunks, Trunk{
			ID:               row[trunkInterfaceLabel],
			Name:             row[trunkNameLabel],
			Members:          members,
			AdminMode:        row[trunkAdminModeLabel] == "Enable",
			StaticCapability: row[trunkStaticCapLabel] == "Enable",
		})
	}
	return trunks, nil
}

// FindTrunkByName returns the trunk with the given name, its zero-based
// row index (for DeleteTrunk), or (nil, -1, nil) if no trunk has that name.
func (c *Client) FindTrunkByName(name string) (*Trunk, int, error) {
	trunks, err := c.ListTrunks()
	if err != nil {
		return nil, -1, err
	}
	for i, t := range trunks {
		if t.Name == name {
			found := t
			return &found, i, nil
		}
	}
	return nil, -1, nil
}

// CreateTrunk creates a new trunk with the given name (1-15 alphanumeric
// characters) and returns it once created, including its switch-assigned
// "TrunkN" id.
func (c *Client) CreateTrunk(name string) (Trunk, error) {
	_, err := c.SubmitForm(trunkConfigPath, map[string]string{
		trunkCreateField:    "Enable",
		trunkNameField:      name,
		trunkAddActionField: "Add",
	})
	if err != nil {
		return Trunk{}, err
	}
	trunk, _, err := c.FindTrunkByName(name)
	if err != nil {
		return Trunk{}, err
	}
	if trunk == nil {
		return Trunk{}, fmt.Errorf("trunk %q not found after creation", name)
	}
	return *trunk, nil
}

// DeleteTrunk removes the named trunk. See the package doc comment for the
// two confirmed-live gotchas (must resend the row's current values, must
// NOT include the page's Create-section fields).
func (c *Client) DeleteTrunk(name string) error {
	trunk, index, err := c.FindTrunkByName(name)
	if err != nil {
		return err
	}
	if trunk == nil {
		return nil // already gone
	}

	adminValue := "Disable"
	if trunk.AdminMode {
		adminValue = "Enable"
	}
	staticValue := "Disable"
	if trunk.StaticCapability {
		staticValue = "Enable"
	}

	prefix := fmt.Sprintf("1.%d.1.", index)
	_, err = c.SubmitForm(trunkConfigPath, map[string]string{
		prefix + trunkRowInterfaceField:    trunk.ID,
		prefix + trunkRowNameField:         trunk.Name,
		prefix + trunkRowMembersField:      strings.Join(trunk.Members, ","),
		prefix + trunkRowAdminField:        adminValue,
		prefix + trunkRowStaticField:       staticValue,
		prefix + trunkRowModifyField:       "Disable",
		prefix + trunkRowDeleteField:       "Enable",
		prefix + trunkRowDeleteActionField: "Delete",
	})
	return err
}

const trunkMembershipTrunkIDField = "v_1_1_1"

// trunkMembershipHiddenFields must be resubmitted unchanged on every
// SetTrunkMembers call or the switch rejects the write with FILTER_MISSING
// -- confirmed live.
var trunkMembershipHiddenFields = []string{
	"v_1_2_1", "v_1_2_2", "v_1_3_1", "v_5_1_1", "v_6_1_1",
	"v_2_1_2", "v_2_1_3", "v_2_1_4", "v_3_1_1", "v_3_2_1", "v_3_3_1", "v_3_4_1",
}

// SetTrunkMembers sets exactly which of the switch's 24 ports belong to
// trunkID (e.g. "Trunk1"), replacing any previous membership entirely.
func (c *Client) SetTrunkMembers(trunkID string, members map[string]bool) error {
	// Select the trunk first (submit_flag=1, reload -- doesn't apply
	// anything, just like PortConfiguration.html's port selector) so the
	// hidden context fields we read back reflect this specific trunk, not
	// whatever was last selected.
	html, err := c.postForm(trunkMembershipPath, map[string]string{
		"submit_flag":               "1",
		"submit_target":             "TrunkMembership.html",
		"err_flag":                  "0",
		"err_msg":                   "",
		"clazz_information":         "TrunkMembership.html",
		trunkMembershipTrunkIDField: trunkID,
	})
	if err != nil {
		return err
	}

	fields := map[string]string{trunkMembershipTrunkIDField: trunkID}
	for _, name := range trunkMembershipHiddenFields {
		fields[name] = extractHiddenFieldValue(html, name)
	}

	for i := 0; i < 24; i++ {
		port := strconv.Itoa(i + 1)
		prefix := fmt.Sprintf("1.%d.24.", i)
		fields[prefix+"v_2_3_1"] = port
		fields[prefix+"v_2_3_2"] = "Enable"
		if members[port] {
			fields[prefix+"v_2_3_3"] = trunkID
		} else {
			fields[prefix+"v_2_3_3"] = "1"
		}
	}

	_, err = c.SubmitForm(trunkMembershipPath, fields)
	return err
}
