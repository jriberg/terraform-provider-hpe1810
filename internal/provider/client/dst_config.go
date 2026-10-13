// Summer_Time_Configuration.html has THREE DST modes (Disable, Recurring,
// NonRecurring), each with its own full set of fields, ALL present in the
// DOM simultaneously (JS shows/hides based on the selected mode) -- same
// shape of problem as PortConfiguration.html's duplicate Link Speed fields,
// except here several labels collide WITHIN one table (Month/Hours/Minutes/
// Offset each appear once in the NonRecurring block and once in the
// Recurring block). Rather than lean on generic xepage.go parsing and its
// document-order last-wins behavior, this reads/writes via hardcoded field
// names extracted directly with regex -- unambiguous regardless of DOM
// order.
//
// Only Disable and Recurring are supported; NonRecurring is rejected
// outright rather than guessed at (it uses a different, absolute-date field
// set: Month/Date/Year/Hours/Minutes, v_3_1_1..v_3_10_1).
//
// Confirmed live before this was wired into a resource: submitting mode +
// only the Recurring block's fields (omitting the NonRecurring block
// entirely) was accepted with no err_flag and no FILTER_MISSING -- unlike
// PortConfiguration.html, this page does NOT require hidden hold_all_fields.
package client

import (
	"fmt"
	"regexp"
	"strconv"
)

const (
	dstConfigPath         = "/Summer_Time_Configuration.html"
	dstModeField          = "v_1_1_1"
	dstStartWeekField     = "v_3_12_1"
	dstStartDayField      = "v_3_13_1"
	dstStartMonthField    = "v_3_14_1"
	dstStartHourField     = "v_3_15_1"
	dstStartMinuteField   = "v_3_399_1"
	dstEndWeekField       = "v_3_16_1"
	dstEndDayField        = "v_3_17_1"
	dstEndMonthField      = "v_3_18_1"
	dstEndHourField       = "v_3_19_1"
	dstEndMinuteField     = "v_3_498_1"
	dstOffsetField        = "v_3_20_1"
	dstOffsetAcronymField = "v_3_21_1"

	DSTModeDisable   = "Disable"
	DSTModeRecurring = "Recurring"
)

type DSTConfig struct {
	Mode          string
	StartWeek     int
	StartDay      string
	StartMonth    string
	StartHour     int
	StartMinute   int
	EndWeek       int
	EndDay        string
	EndMonth      string
	EndHour       int
	EndMinute     int
	OffsetMinutes int
	OffsetAcronym string
}

func extractHiddenFieldValue(html, name string) string {
	re := regexp.MustCompile(`(?i)NAME=` + regexp.QuoteMeta(name) + `\b[^>]*VALUE="([^"]*)"`)
	m := re.FindStringSubmatch(html)
	if m == nil {
		return ""
	}
	return m[1]
}

func (c *Client) ReadDSTConfig() (DSTConfig, error) {
	html, err := c.Fetch(dstConfigPath)
	if err != nil {
		return DSTConfig{}, err
	}

	atoi := func(name string) int {
		n, _ := strconv.Atoi(extractHiddenFieldValue(html, name))
		return n
	}

	return DSTConfig{
		Mode:          extractHiddenFieldValue(html, dstModeField),
		StartWeek:     atoi(dstStartWeekField),
		StartDay:      extractHiddenFieldValue(html, dstStartDayField),
		StartMonth:    extractHiddenFieldValue(html, dstStartMonthField),
		StartHour:     atoi(dstStartHourField),
		StartMinute:   atoi(dstStartMinuteField),
		EndWeek:       atoi(dstEndWeekField),
		EndDay:        extractHiddenFieldValue(html, dstEndDayField),
		EndMonth:      extractHiddenFieldValue(html, dstEndMonthField),
		EndHour:       atoi(dstEndHourField),
		EndMinute:     atoi(dstEndMinuteField),
		OffsetMinutes: atoi(dstOffsetField),
		OffsetAcronym: extractHiddenFieldValue(html, dstOffsetAcronymField),
	}, nil
}

func (c *Client) SetDSTConfig(desired DSTConfig) error {
	if desired.Mode != DSTModeDisable && desired.Mode != DSTModeRecurring {
		return fmt.Errorf("unsupported Daylight Saving Time mode %q -- this resource only supports %q and %q (NonRecurring is not supported)", desired.Mode, DSTModeDisable, DSTModeRecurring)
	}

	fields := map[string]string{
		dstModeField:          desired.Mode,
		dstStartWeekField:     strconv.Itoa(desired.StartWeek),
		dstStartDayField:      desired.StartDay,
		dstStartMonthField:    desired.StartMonth,
		dstStartHourField:     strconv.Itoa(desired.StartHour),
		dstStartMinuteField:   strconv.Itoa(desired.StartMinute),
		dstEndWeekField:       strconv.Itoa(desired.EndWeek),
		dstEndDayField:        desired.EndDay,
		dstEndMonthField:      desired.EndMonth,
		dstEndHourField:       strconv.Itoa(desired.EndHour),
		dstEndMinuteField:     strconv.Itoa(desired.EndMinute),
		dstOffsetField:        strconv.Itoa(desired.OffsetMinutes),
		dstOffsetAcronymField: desired.OffsetAcronym,
	}
	_, err := c.SubmitForm(dstConfigPath, fields)
	return err
}
