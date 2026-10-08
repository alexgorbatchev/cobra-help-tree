package main

import (
	"bytes"
	"strings"
	"testing"

	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v2"
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
		"Environment Variables:\n  MYTOOL_DEFAULT_ROLE  Initial role assigned to new users",
		"Quickstart:\n  mytool user create alice                # create user with default role\n  mytool user create bob bob@example.com  # create user and send invite",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help screen missing %q:\n%s", want, out.String())
		}
	}
}

func TestDemoRendersAgentHelp(t *testing.T) {
	t.Setenv("AGENT", "1")

	root := newRootCommand()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"user", "create", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("user create --help (agent): %v", err)
	}

	for _, want := range []string{
		"command: mytool user create",
		"env:\n  - MYTOOL_DEFAULT_ROLE: Initial role assigned to new users",
		"quickstart:\n  - mytool user create alice: create user with default role\n  - mytool user create bob bob@example.com: create user and send invite",
		"metadata:\n  scope: admin\n  table: users\n  usage: 1 write per call",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("agent help screen missing %q:\n%s", want, out.String())
		}
	}
}

func TestDemoPrintsItsSkill(t *testing.T) {
	for _, agentEnv := range []string{"0", "1"} {
		t.Run("AGENT="+agentEnv, func(t *testing.T) {
			t.Setenv("AGENT", agentEnv)

			root := newRootCommand()
			out := new(bytes.Buffer)
			root.SetOut(out)
			root.SetErr(out)
			root.SetArgs([]string{"skill"})

			if err := root.Execute(); err != nil {
				t.Fatalf("skill: %v", err)
			}
			if out.String() != skill {
				t.Errorf("skill printed something other than the embedded SKILL.md:\n%s", out.String())
			}
		})
	}
}

func TestDemoAgentHelpOpensWithTheSkillAlert(t *testing.T) {
	t.Setenv("AGENT", "1")

	root := newRootCommand()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"user", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("user --help (agent): %v", err)
	}

	want := "ALERT: Agents must read `AGENT=1 mytool skill` before using this tool.\ncommand: mytool user\n"
	if !strings.HasPrefix(out.String(), want) {
		t.Errorf("agent help does not open with %q:\n%s", want, out.String())
	}
}

// The skill is the reference an agent works from, so a command, flag, argument
// or environment variable the demo gains has to reach it in the same change.
func TestDemoSkillCoversTheWholeInterface(t *testing.T) {
	if missing := cobrahelptree.SkillOmissions(newRootCommand(), newCatalog(), skill); len(missing) > 0 {
		t.Errorf("SKILL.md does not mention:\n  %s", strings.Join(missing, "\n  "))
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
