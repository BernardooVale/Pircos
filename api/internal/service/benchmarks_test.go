package service

import (
	"testing"
	"time"
)

func TestParseBCBDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantYear int
		wantMon  time.Month
		wantDay  int
		wantErr  bool
	}{
		{"Valid date", "15/03/2024", 2024, time.March, 15, false},
		{"First day of year", "01/01/2023", 2023, time.January, 1, false},
		{"Last day of year", "31/12/2025", 2025, time.December, 31, false},
		{"Invalid format", "2024-03-15", 0, 0, 0, true},
		{"Empty string", "", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseBCBDate(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseBCBDate(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseBCBDate(%q) unexpected error: %v", tt.input, err)
			}
			if result.Year() != tt.wantYear || result.Month() != tt.wantMon || result.Day() != tt.wantDay {
				t.Errorf("parseBCBDate(%q) = %v, want %d-%02d-%02d",
					tt.input, result, tt.wantYear, tt.wantMon, tt.wantDay)
			}
		})
	}
}

func TestSGSSeriesMap(t *testing.T) {
	// Ensure all expected series codes are mapped
	expected := map[int]string{
		11:  "SELIC",
		12:  "CDI",
		433: "IPCA",
	}

	for code, name := range expected {
		got, ok := sgsSeriesMap[code]
		if !ok {
			t.Errorf("sgsSeriesMap missing series code %d (%s)", code, name)
			continue
		}
		if got != name {
			t.Errorf("sgsSeriesMap[%d] = %q, want %q", code, got, name)
		}
	}
}
