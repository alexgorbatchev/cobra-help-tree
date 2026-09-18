package cobrahelptree

import (
	"strings"
	"testing"
)

func TestAgentOptionsValidate(t *testing.T) {
	if err := (AgentOptions{}).Validate(); err != nil {
		t.Errorf("zero AgentOptions should be valid, got: %v", err)
	}
	if err := (AgentOptions{MaxLineWidth: 80}).Validate(); err != nil {
		t.Errorf("positive MaxLineWidth should be valid, got: %v", err)
	}

	err := AgentOptions{MaxLineWidth: -1}.Validate()
	if err == nil {
		t.Fatal("expected a negative MaxLineWidth to be rejected")
	}
	if !strings.Contains(err.Error(), "MaxLineWidth") {
		t.Errorf("error %q does not name the offending field", err)
	}
}

func TestHelpOptionsValidateDelegatesToTree(t *testing.T) {
	if err := (HelpOptions{}).Validate(); err != nil {
		t.Errorf("zero HelpOptions should be valid, got: %v", err)
	}

	err := HelpOptions{Tree: TreeOptions{MinLabelWidth: -3}}.Validate()
	if err == nil {
		t.Fatal("expected HelpOptions.Validate to surface the nested TreeOptions error")
	}
	if !strings.Contains(err.Error(), "MinLabelWidth") {
		t.Errorf("error %q does not name the offending field", err)
	}

	err = HelpOptions{Agent: AgentOptions{MaxLineWidth: -3}}.Validate()
	if err == nil {
		t.Fatal("expected HelpOptions.Validate to surface the nested AgentOptions error")
	}
	if !strings.Contains(err.Error(), "MaxLineWidth") {
		t.Errorf("error %q does not name the offending field", err)
	}
}

func TestTreeOptionsValidate(t *testing.T) {
	tests := []struct {
		name      string
		opt       TreeOptions
		wantField string
	}{
		{"zero value is valid", TreeOptions{}, ""},
		{"populated value is valid", TreeOptions{MinPadding: 4, MinLabelWidth: 12, TerminalWidth: 100}, ""},
		{"negative MinPadding", TreeOptions{MinPadding: -1}, "MinPadding"},
		{"negative MinLabelWidth", TreeOptions{MinLabelWidth: -1}, "MinLabelWidth"},
		{"negative TerminalWidth", TreeOptions{TerminalWidth: -1}, "TerminalWidth"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opt.Validate()

			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("expected valid options, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error naming %s, got nil", tt.wantField)
			}
			if !strings.Contains(err.Error(), tt.wantField) {
				t.Errorf("error %q does not name the offending field %q", err, tt.wantField)
			}
		})
	}
}
