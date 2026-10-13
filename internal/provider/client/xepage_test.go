package client

import (
	"os"
	"testing"
)

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return string(data)
}

// Expected values below were captured live from the real switch (see
// ~/code/experiments/hpe/page_snapshots/current_state.json) at the time
// these fixtures were saved -- this test is about the parser's correctness,
// not about the switch's current state.

func TestParseXETables_ScalarShape_SysDescription(t *testing.T) {
	tables, err := ParseXETables(loadFixture(t, "SysDescription.html"))
	if err != nil {
		t.Fatalf("ParseXETables: %v", err)
	}

	var sysDesc *Table
	for i := range tables {
		if tables[i].Caption == "System Description" {
			sysDesc = &tables[i]
		}
	}
	if sysDesc == nil {
		t.Fatalf("no table captioned %q found; got captions: %v", "System Description", captions(tables))
	}
	if sysDesc.Kind != "scalar" {
		t.Errorf("Kind = %q, want %q", sysDesc.Kind, "scalar")
	}

	want := map[string]string{
		"System Name":     "Draupnir01",
		"System Location": "Lab",
		"Software Version": "P.2.24",
	}
	for label, wantValue := range want {
		if got := sysDesc.Record[label]; got != wantValue {
			t.Errorf("Record[%q] = %q, want %q", label, got, wantValue)
		}
	}

	if fieldName := sysDesc.FieldNames["System Name"]; fieldName != "v_1_2_1" {
		t.Errorf("FieldNames[%q] = %q, want %q", "System Name", fieldName, "v_1_2_1")
	}
}

func TestParseXETables_TabularShape_PortSummary(t *testing.T) {
	tables, err := ParseXETables(loadFixture(t, "PortSummary.html"))
	if err != nil {
		t.Fatalf("ParseXETables: %v", err)
	}

	var portSummary *Table
	for i := range tables {
		if tables[i].Caption == "Port Summary" {
			portSummary = &tables[i]
		}
	}
	if portSummary == nil {
		t.Fatalf("no table captioned %q found; got captions: %v", "Port Summary", captions(tables))
	}
	if portSummary.Kind != "tabular" {
		t.Errorf("Kind = %q, want %q", portSummary.Kind, "tabular")
	}
	if len(portSummary.Rows) != 24 {
		t.Fatalf("len(Rows) = %d, want 24", len(portSummary.Rows))
	}

	first := portSummary.Rows[0]
	if first["Interface"] != "1" {
		t.Errorf(`Rows[0]["Interface"] = %q, want "1"`, first["Interface"])
	}
	if first["Physical Type"] != "RJ45" {
		t.Errorf(`Rows[0]["Physical Type"] = %q, want "RJ45"`, first["Physical Type"])
	}
}

func TestFetchAllTables_BlankCaptionTable_FDBSearch(t *testing.T) {
	tables, err := ParseXETables(loadFixture(t, "FDBSearch.html"))
	if err != nil {
		t.Fatalf("ParseXETables: %v", err)
	}

	var summary, entries *Table
	for i := range tables {
		if tables[i].Caption == "MAC Table" {
			summary = &tables[i]
		}
		if tables[i].ID == "1_1" {
			entries = &tables[i]
		}
	}
	if summary == nil {
		t.Fatalf("no table captioned %q found; got captions: %v", "MAC Table", captions(tables))
	}
	if summary.Record["Current Entries"] != "2" {
		t.Errorf(`summary.Record["Current Entries"] = %q, want "2"`, summary.Record["Current Entries"])
	}

	if entries == nil {
		t.Fatalf("no table with id %q found", "1_1")
	}
	if entries.Kind != "tabular" {
		t.Errorf("entries.Kind = %q, want %q", entries.Kind, "tabular")
	}
	if len(entries.Rows) != 2 {
		t.Fatalf("len(entries.Rows) = %d, want 2", len(entries.Rows))
	}
	if entries.Rows[0]["MAC Address"] != "4C:EA:41:63:04:1B" {
		t.Errorf(`entries.Rows[0]["MAC Address"] = %q, want "4C:EA:41:63:04:1B"`, entries.Rows[0]["MAC Address"])
	}
}

func captions(tables []Table) []string {
	out := make([]string, len(tables))
	for i, t := range tables {
		out[i] = t.Caption
	}
	return out
}
