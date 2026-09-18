package main

import (
	"bytes"
	"strings"
	"testing"
)

// The demo is what `just run` drives and what the README's screens are generated
// from, so these tests exist to keep it honest: a broken example would otherwise
// be caught only by the compiler, and a README example would quietly go stale.

func TestDemoRendersTheDocumentedScreens(t *testing.T) {
	t.Setenv("AGENT", "0")
	t.Setenv("COLUMNS", "200")

	root := newRootCommand()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"user", "create", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("user create --help: %v", err)
	}

	for _, want := range []string{
		"Usage:\n  mytool user create <name> [flags]",
		"Arguments:\n  <name>",
		"[email]",
		"Login name for the new user",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help screen missing %q:\n%s", want, out.String())
		}
	}
}

func TestDemoCommandsReportTheirInvocation(t *testing.T) {
	tests := []struct {
		name     string
		agentEnv string
		want     string
	}{
		{"human mode", "0", "[OK] mytool user create alice\n"},
		{"agent mode", "1", "command: mytool user create\nargs: alice\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AGENT", tt.agentEnv)

			root := newRootCommand()
			out := new(bytes.Buffer)
			root.SetOut(out)
			root.SetErr(out)
			root.SetArgs([]string{"user", "create", "alice"})

			if err := root.Execute(); err != nil {
				t.Fatalf("user create alice: %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}
