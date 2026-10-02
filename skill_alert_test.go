package cobrahelptree

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSkillAlertCoversHelpAndUsage(t *testing.T) {
	for _, mode := range []string{"", "1", "true", "yes"} {
		for _, args := range [][]string{{"--help"}, {"playlist", "--help"}, {"help", "playlist"}, {"completion", "bash", "--help"}, {"--invalid"}} {
			t.Run(mode+strings.Join(args, " "), func(t *testing.T) {
				t.Setenv("AGENT", mode)
				root := buildSampleCommandHierarchy()
				root.Version = "1.0.0"
				root.SetVersionTemplate("{{.Version}}\n")
				var out, stderr bytes.Buffer
				root.SetOut(&out)
				root.SetErr(&stderr)
				if err := SetupWithOptions(root, HelpOptions{Agent: AgentOptions{RequireSkill: true}}); err != nil {
					t.Fatal(err)
				}
				root.SetArgs(args)
				err := root.Execute()
				if args[0] != "--invalid" && err != nil {
					t.Fatal(err)
				}
				screen := out.String()
				alert := "ALERT: Agents must read `AGENT=1 app skill` before using this tool.\n"
				if mode != "" && !strings.HasPrefix(screen, alert) {
					t.Fatalf("missing alert: %q", screen)
				}
				if mode == "" && strings.Contains(screen, "ALERT:") {
					t.Fatalf("alert in human help: %q", screen)
				}
			})
		}
	}
}

func TestSkillAlertUsesRootNameAndPreservesScreen(t *testing.T) {
	root := &cobra.Command{Use: "tool"}
	child := leafCommand("inspect", "Inspect an item")
	root.AddCommand(child)
	plain := RenderAgentHelp(child, nil, AgentOptions{})
	for _, width := range []int{1, 20, 60, 100} {
		alerted := RenderAgentHelp(child, nil, AgentOptions{RequireSkill: true, MaxLineWidth: width})
		if !strings.HasPrefix(alerted, "ALERT: Agents must read `AGENT=1 tool skill` before using this tool.\n") {
			t.Fatal(alerted)
		}
	}
	full := RenderAgentHelp(child, nil, AgentOptions{RequireSkill: true})
	if !strings.HasSuffix(full, plain) {
		t.Fatal("alert altered the ordinary agent screen")
	}
}
