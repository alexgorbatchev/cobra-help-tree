package cobrahelptree

import "testing"

func TestGetTerminalWidth(t *testing.T) {
	tests := []struct {
		name    string
		columns string
		want    int
	}{
		{"explicit width", "100", 100},
		{"non-numeric falls back", "invalid", 0},
		{"zero falls back", "0", 0},
		{"negative falls back", "-5", 0},
		{"empty falls back", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withNonTerminalStdout(t)
			t.Setenv("COLUMNS", tt.columns)

			if got := GetTerminalWidth(); got != tt.want {
				t.Errorf("GetTerminalWidth() with COLUMNS=%q = %d, want %d", tt.columns, got, tt.want)
			}
		})
	}
}

func TestIsAgentMode(t *testing.T) {
	tests := []struct {
		envVal string
		want   bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"yes", true},
		{"YES", true},
		{"0", false},
		{"false", false},
		{"", false},
		{"random", false},
	}

	for _, tt := range tests {
		t.Setenv("AGENT", tt.envVal)
		if got := IsAgentMode(); got != tt.want {
			t.Errorf("IsAgentMode() with AGENT=%q = %v, want %v", tt.envVal, got, tt.want)
		}
	}
}
