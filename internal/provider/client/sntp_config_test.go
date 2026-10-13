package client

import (
	"reflect"
	"testing"
)

// testdata/SNTP.html is hp-iac's page snapshot from before NTP was fixed
// (server 10.0.0.1, every attempt timing out).
func TestParseSNTPConfig(t *testing.T) {
	got, err := parseSNTPConfig(loadFixture(t, "SNTP.html"))
	if err != nil {
		t.Fatalf("parseSNTPConfig: %v", err)
	}
	want := SNTPConfig{
		Mode:             SNTPModeUnicast,
		Server:           "10.0.0.1",
		Port:             123,
		TimeFormat:       "24 Hour",
		LastUpdate:       "Jan 01 00:00:00 1970\n",
		LastUpdateStatus: "Request timed out",
		HasServerRow:     true,
	}
	if got != want {
		t.Errorf("parseSNTPConfig = %+v, want %+v", got, want)
	}
}

// Expected maps are the browser captures in sntp_config.go, de-duplicated
// and minus the read-only fields and the bookkeeping SubmitForm adds.
func TestSNTPFormFields(t *testing.T) {
	cases := []struct {
		name    string
		in      SNTPConfig
		noRow   bool
		want    map[string]string
		wantErr bool
	}{
		{
			name: "unicast, row exists -> modify",
			in:   SNTPConfig{Mode: SNTPModeUnicast, Server: "194.58.200.20", Port: 123, TimeFormat: "24 Hour"},
			want: map[string]string{
				"v_1_1_1": "Unicast", "v_1_4_1": "IPv4", "v_1_5_1": "194.58.200.20", "v_1_6_1": "123",
				"v_1_13_1": "24 Hour", "v_1_11_1": "1", "v_1_12_4": "Apply",
			},
		},
		{
			name:  "unicast, no row -> create",
			in:    SNTPConfig{Mode: SNTPModeUnicast, Server: "194.58.200.20", Port: 123, TimeFormat: "24 Hour"},
			noRow: true,
			want: map[string]string{
				"v_1_1_1": "Unicast", "v_1_4_1": "IPv4", "v_1_5_1": "194.58.200.20", "v_1_6_1": "123",
				"v_1_13_1": "24 Hour", "v_1_11_1": "\xa0", "v_1_12_1": "Apply",
			},
		},
		{
			name: "disabled sends no server fields",
			in:   SNTPConfig{Mode: SNTPModeDisabled, Server: "194.58.200.20", Port: 123, TimeFormat: "24 Hour"},
			want: map[string]string{
				"v_1_1_1": "Disabled", "v_1_13_1": "24 Hour", "v_1_11_1": "1", "v_1_12_2": "Apply",
			},
		},
		{name: "hostname rejected", in: SNTPConfig{Mode: SNTPModeUnicast, Server: "ntp.se", Port: 123, TimeFormat: "24 Hour"}, wantErr: true},
		{name: "ipv6 rejected", in: SNTPConfig{Mode: SNTPModeUnicast, Server: "2a01:3f7::1", Port: 123, TimeFormat: "24 Hour"}, wantErr: true},
		{name: "0.0.0.0 rejected", in: SNTPConfig{Mode: SNTPModeUnicast, Server: "0.0.0.0", Port: 123, TimeFormat: "24 Hour"}, wantErr: true},
		{name: "port 0 rejected", in: SNTPConfig{Mode: SNTPModeUnicast, Server: "194.58.200.20", Port: 0, TimeFormat: "24 Hour"}, wantErr: true},
		{name: "broadcast rejected", in: SNTPConfig{Mode: "Broadcast", TimeFormat: "24 Hour"}, wantErr: true},
		{name: "bad time format", in: SNTPConfig{Mode: SNTPModeDisabled, TimeFormat: "24h"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sntpFormFields(tc.in, !tc.noRow)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("sntpFormFields: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("sntpFormFields = %v, want %v", got, tc.want)
			}
		})
	}
}

// Disabled mode renders Server Port without a value (seen live on the
// read-back after a Disabled write); that must not be a parse error.
func TestParseSNTPConfig_DisabledEmptyPort(t *testing.T) {
	html := `<INPUT xid=1_1_1 TYPE=hidden NAME=v_1_1_1 VALUE="Disabled">` +
		`<INPUT xid=1_6_1 DISABLED TYPE=hidden NAME=v_1_6_1 VALUE="">` +
		`<INPUT xid=1_13_1 TYPE=hidden NAME=v_1_13_1 VALUE="24 Hour">`
	got, err := parseSNTPConfig(html)
	if err != nil {
		t.Fatalf("parseSNTPConfig: %v", err)
	}
	if got.Mode != SNTPModeDisabled || got.Port != 0 || got.TimeFormat != "24 Hour" {
		t.Errorf("parseSNTPConfig = %+v", got)
	}
	// Absent entirely is the same case.
	if _, err := parseSNTPConfig(`<INPUT NAME=v_1_1_1 VALUE="Disabled">`); err != nil {
		t.Errorf("absent port: %v", err)
	}
}

// Unicast with no server row (left behind by a v_1_12_4 write from
// Disabled, seen live): every field is a lone 0xA0.
func TestParseSNTPConfig_NoServerRow(t *testing.T) {
	html := `<INPUT NAME=v_1_1_1 VALUE="Unicast">` +
		`<INPUT NAME=v_1_5_1 VALUE="` + "\xa0" + `">` +
		`<INPUT NAME=v_1_6_1 VALUE="&nbsp;">` +
		`<INPUT NAME=v_1_9_1 VALUE="` + "\xa0" + `">` +
		`<INPUT NAME=v_1_11_1 VALUE="` + "\xa0" + `">` +
		`<INPUT NAME=v_1_13_1 VALUE="24 Hour">`
	got, err := parseSNTPConfig(html)
	if err != nil {
		t.Fatalf("parseSNTPConfig: %v", err)
	}
	want := SNTPConfig{Mode: SNTPModeUnicast, TimeFormat: "24 Hour"}
	if got != want {
		t.Errorf("parseSNTPConfig = %+v, want %+v", got, want)
	}
}
