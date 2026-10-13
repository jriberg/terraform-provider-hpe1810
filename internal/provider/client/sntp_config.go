// SNTP.html (SNTP Configuration). Unlike the single-Apply pages the generic
// SubmitForm callers write to, this page has THREE Apply buttons, and which
// one is right depends on the CURRENT state of the switch's unicast server
// table (row 1, UcastServerIndex v_1_11_1 -- _xe_SNTP.js xa_1_11_1 toggles
// them on whether that index is set):
//
//	v_1_12_1  Unicast, no server row yet  -> creates row 1 + sets Unicast
//	v_1_12_4  Unicast, row 1 exists       -> modifies row 1
//	v_1_12_2  Disabled                     -> sets Disabled AND DELETES row 1
//
// So a Disabled -> Unicast change must use v_1_12_1: v_1_12_4 there sets the
// mode but the switch rejects every server field ("Failed to Set 'SNTP/NTP
// Server'"), leaving Unicast with no server -- and from that state the UI's
// own Disabled fails too ("Failed to Set 'UcastServerRowStatus' with
// 'Delete' ... FILTER_MISSING", nothing to delete). All seen live on fw
// P.2.24, 2026-10-14. SetSNTPConfig therefore reads the page first and
// picks the button. Field names are hardcoded (same approach as
// dst_config.go) because buttons aren't in any captioned table.
//
// Browser captures (fw P.2.24, 2026-10-14), de-duplicated:
//
//	modify:   v_1_1_1=Unicast&v_1_4_1=IPv4&v_1_5_1=<ip>&v_1_6_1=123
//	          &v_1_13_1=24+Hour&v_1_11_1=1&v_1_12_4=Apply
//	create:   same, but v_1_11_1=%A0 (no row) and v_1_12_1=Apply
//	disable:  v_1_1_1=Disabled&v_1_13_1=24+Hour&v_1_11_1=1&v_1_12_2=Apply
//
// The browser sends every field twice (hidden input + visible control,
// plus "on" from the Enable checkbox); one value per field is enough, same
// as every other page. In Disabled mode the browser omits address type,
// server and port entirely (disabled inputs), so this does too. Empty
// fields come back as a lone non-breaking space (0xA0), see isBlank.
//
// Read-only fields (Last Update, Attempts, Last Update Status, Failures)
// are never sent. Address Type only offers "IPv4" on this firmware (DNS
// exists in the enum but isn't exposed), so the server must be an IPv4
// literal.
package client

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

const (
	sntpConfigPath        = "/SNTP.html"
	sntpModeField         = "v_1_1_1"
	sntpAddressTypeField  = "v_1_4_1"
	sntpServerField       = "v_1_5_1"
	sntpPortField         = "v_1_6_1"
	sntpLastUpdateField   = "v_1_7_1"
	sntpStatusField       = "v_1_9_1"
	sntpServerIndexField  = "v_1_11_1"
	sntpTimeFormatField   = "v_1_13_1"
	sntpApplyCreateBtn    = "v_1_12_1"
	sntpApplyModifyBtn    = "v_1_12_4"
	sntpApplyDisabledBtn  = "v_1_12_2"
	sntpAddressTypeIPv4   = "IPv4"
	sntpUnicastServerSlot = "1"

	SNTPModeDisabled = "Disabled"
	SNTPModeUnicast  = "Unicast"
)

type SNTPConfig struct {
	Mode       string
	Server     string
	Port       int
	TimeFormat string
	// Read-only, ignored by SetSNTPConfig.
	LastUpdate       string
	LastUpdateStatus string
	// HasServerRow: unicast server row 1 exists (UcastServerIndex set).
	HasServerRow bool
}

// isBlank: the firmware renders an empty field as a lone non-breaking
// space -- a raw 0xA0 byte (the pages are Latin-1), or &nbsp;.
func isBlank(v string) bool {
	v = strings.ReplaceAll(v, "&nbsp;", "")
	v = strings.ReplaceAll(v, "\u00a0", "")
	v = strings.ReplaceAll(v, "\xa0", "")
	return strings.TrimSpace(v) == ""
}

func blankToEmpty(v string) string {
	if isBlank(v) {
		return ""
	}
	return v
}

func (c *Client) ReadSNTPConfig() (SNTPConfig, error) {
	html, err := c.Fetch(sntpConfigPath)
	if err != nil {
		return SNTPConfig{}, err
	}
	return parseSNTPConfig(html)
}

func parseSNTPConfig(html string) (SNTPConfig, error) {
	mode := extractHiddenFieldValue(html, sntpModeField)
	if mode == "" {
		return SNTPConfig{}, fmt.Errorf("field %s (Enable SNTP) not found on %s", sntpModeField, sntpConfigPath)
	}
	// Server Port has no usable value in Disabled mode or without a server
	// row (seen live 2026-10-14), so blank is 0 rather than an error; the
	// resource's read-back still catches a Unicast write that didn't take.
	port := 0
	if raw := extractHiddenFieldValue(html, sntpPortField); !isBlank(raw) {
		var err error
		if port, err = strconv.Atoi(strings.TrimSpace(raw)); err != nil {
			return SNTPConfig{}, fmt.Errorf("parsing Server Port %q on %s: %w", raw, sntpConfigPath, err)
		}
	}
	return SNTPConfig{
		Mode:             mode,
		Server:           blankToEmpty(extractHiddenFieldValue(html, sntpServerField)),
		Port:             port,
		TimeFormat:       extractHiddenFieldValue(html, sntpTimeFormatField),
		LastUpdate:       blankToEmpty(extractHiddenFieldValue(html, sntpLastUpdateField)),
		LastUpdateStatus: blankToEmpty(extractHiddenFieldValue(html, sntpStatusField)),
		HasServerRow:     !isBlank(extractHiddenFieldValue(html, sntpServerIndexField)),
	}, nil
}

// sntpFormFields builds the POST fields for desired given whether server
// row 1 currently exists, mirroring the browser captures above. Pure, so
// it's unit-tested without a switch.
func sntpFormFields(desired SNTPConfig, hasServerRow bool) (map[string]string, error) {
	if desired.TimeFormat != "12 Hour" && desired.TimeFormat != "24 Hour" {
		return nil, fmt.Errorf("unsupported time format %q (want \"12 Hour\" or \"24 Hour\")", desired.TimeFormat)
	}
	// The browser echoes UcastServerIndex: "1" with a row, 0xA0 without.
	index := "\xa0"
	if hasServerRow {
		index = sntpUnicastServerSlot
	}
	fields := map[string]string{
		sntpModeField:        desired.Mode,
		sntpTimeFormatField:  desired.TimeFormat,
		sntpServerIndexField: index,
	}

	switch desired.Mode {
	case SNTPModeDisabled:
		fields[sntpApplyDisabledBtn] = "Apply"
	case SNTPModeUnicast:
		addr, err := netip.ParseAddr(desired.Server)
		if err != nil || !addr.Is4() || addr.IsUnspecified() {
			return nil, fmt.Errorf("server %q must be an IPv4 address other than 0.0.0.0 (the switch accepts no hostnames)", desired.Server)
		}
		if desired.Port < 1 || desired.Port > 65535 {
			return nil, fmt.Errorf("port %d out of range 1-65535", desired.Port)
		}
		fields[sntpAddressTypeField] = sntpAddressTypeIPv4
		fields[sntpServerField] = addr.String()
		fields[sntpPortField] = strconv.Itoa(desired.Port)
		if hasServerRow {
			fields[sntpApplyModifyBtn] = "Apply"
		} else {
			fields[sntpApplyCreateBtn] = "Apply"
		}
	default:
		return nil, fmt.Errorf("unsupported SNTP mode %q (want %q or %q)", desired.Mode, SNTPModeDisabled, SNTPModeUnicast)
	}
	return fields, nil
}

// SetSNTPConfig reads the page first to pick the right Apply button (see
// the file comment), then writes desired.
func (c *Client) SetSNTPConfig(desired SNTPConfig) error {
	current, err := c.ReadSNTPConfig()
	if err != nil {
		return fmt.Errorf("reading current SNTP state: %w", err)
	}
	fields, err := sntpFormFields(desired, current.HasServerRow)
	if err != nil {
		return err
	}
	_, err = c.SubmitForm(sntpConfigPath, fields)
	// Disabling with no server row: the switch tries to delete row 1 and
	// reports FILTER_MISSING even though there's nothing to delete. Not a
	// failure in itself -- the caller's read-back decides whether the mode
	// actually changed.
	if err != nil && desired.Mode == SNTPModeDisabled && !current.HasServerRow && strings.Contains(err.Error(), "FILTER_MISSING") {
		return nil
	}
	return err
}
