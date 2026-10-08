package cobrahelptree

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// sampleSkill stands in for an embedded SKILL.md. The frontmatter, the blank
// lines and the missing final newline are all there to be reproduced exactly.
const sampleSkill = "---\nname: app\ndescription: Use when operating app.\n---\n\n# app\n\nRun `app playlist list`.\n\n\tindented line\nno newline at the end"

const sampleSkillAlert = "ALERT: Agents must read `AGENT=1 app skill` before using this tool.\n"

// setupWithSkill installs the help screens and the sample skill on a fresh sample
// hierarchy.
func setupWithSkill(t *testing.T) *cobra.Command {
	t.Helper()

	root := buildSampleCommandHierarchy()
	if err := SetupWithOptions(root, HelpOptions{Skill: sampleSkill}); err != nil {
		t.Fatalf("SetupWithOptions: %v", err)
	}
	return root
}

func TestSkillCommandPrintsTheSkillVerbatim(t *testing.T) {
	// The skill is a document an agent reads whole, so both modes print the same
	// bytes: no alert, no reformatting, nothing appended.
	for _, agentEnv := range []string{"0", "1"} {
		t.Run("AGENT="+agentEnv, func(t *testing.T) {
			t.Setenv("AGENT", agentEnv)

			// No SetOut here on purpose: the stream the command falls back to is what
			// is under test.
			root := setupWithSkill(t)
			root.SetArgs([]string{"skill"})

			var execErr error
			stdout, stderr := captureStdio(t, func() {
				execErr = root.Execute()
			})
			if execErr != nil {
				t.Fatalf("skill: %v", execErr)
			}

			if stdout != sampleSkill {
				t.Errorf("skill printed %q, want the skill byte for byte: %q", stdout, sampleSkill)
			}
			if stderr != "" {
				t.Errorf("skill wrote to stderr: %q", stderr)
			}
		})
	}
}

func TestSkillCommandRejectsArguments(t *testing.T) {
	t.Setenv("AGENT", "0")

	root := setupWithSkill(t)
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"skill", "extra"})

	if err := root.Execute(); err == nil {
		t.Fatal("skill accepted a positional argument")
	}
	if strings.Contains(out.String(), "indented line") {
		t.Errorf("skill printed the skill despite the rejected argument:\n%s", out.String())
	}
}

func TestSkillCommandReturnsAWriteFailure(t *testing.T) {
	t.Setenv("AGENT", "0")

	root := setupWithSkill(t)
	root.SetOut(failingWriter{err: io.ErrClosedPipe})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"skill"})

	if err := root.Execute(); !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("skill returned %v, want the write failure %v", err, io.ErrClosedPipe)
	}
}

func TestSkillCommandIsListedInBothModes(t *testing.T) {
	root := setupWithSkill(t)

	screens := map[string]string{
		"tree":  FormatCommandTree(root, TreeOptions{TerminalWidth: 200}),
		"agent": RenderAgentHelp(root, nil, AgentOptions{}),
	}
	for name, screen := range screens {
		if !slices.Contains(listedCommands(screen), "skill") {
			t.Errorf("%s screen does not list the skill command:\n%s", name, screen)
		}
	}
}

func TestAgentHelpStartsWithTheSkillAlert(t *testing.T) {
	t.Setenv("AGENT", "1")

	// Every screen an agent can reach, the generated commands and the screen that
	// follows a mistake included, because the alert is only useful if it cannot be
	// missed.
	tests := []struct {
		name string
		args []string
	}{
		{"the root", []string{"--help"}},
		{"a group", []string{"playlist", "--help"}},
		{"a leaf", []string{"playlist", "list", "--help"}},
		{"the skill command", []string{"skill", "--help"}},
		{"the generated completion group", []string{"completion", "--help"}},
		{"a generated completion command", []string{"completion", "bash", "--help"}},
		{"the generated help command", []string{"help", "playlist"}},
		{"the screen after an unknown flag", []string{"playlist", "list", "--bogus"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := setupWithSkill(t)
			// The usage screen follows cobra's own "Error:" line, so that line is
			// silenced to leave the screen as the whole output.
			root.SilenceErrors = true
			out := new(bytes.Buffer)
			root.SetOut(out)
			root.SetErr(out)
			root.SetArgs(tt.args)

			// The unknown flag case fails by design; the screen is what matters.
			_ = root.Execute()

			if !strings.HasPrefix(out.String(), sampleSkillAlert+"command: app") {
				t.Errorf("screen does not start with the alert followed by the command:\n%s", out.String())
			}
			if strings.Count(out.String(), "ALERT:") != 1 {
				t.Errorf("screen repeats the alert:\n%s", out.String())
			}
		})
	}
}

func TestSkillAlertIsNeverClipped(t *testing.T) {
	// A clipped alert would cut the command it tells the agent to run.
	root := setupWithSkill(t)

	out := RenderAgentHelp(root, nil, AgentOptions{MaxLineWidth: 20})
	if !strings.HasPrefix(out, sampleSkillAlert) {
		t.Errorf("MaxLineWidth clipped the alert:\n%s", out)
	}
}

func TestSkillAlertIsAbsentWithoutTheSkillCommand(t *testing.T) {
	t.Setenv("AGENT", "1")

	t.Run("no skill configured", func(t *testing.T) {
		root := buildSampleCommandHierarchy()
		if err := Setup(root); err != nil {
			t.Fatalf("Setup: %v", err)
		}
		if out := helpOutput(t, root); strings.Contains(out, "ALERT") {
			t.Errorf("alert printed for a CLI with no skill:\n%s", out)
		}
	})

	t.Run("a skill command of the CLI's own", func(t *testing.T) {
		// The alert promises the document this library prints. A command that only
		// shares the name makes no such promise.
		root := buildSampleCommandHierarchy()
		root.AddCommand(leafCommand("skill", "Manage skills"))
		if out := RenderAgentHelp(root, nil, AgentOptions{}); strings.Contains(out, "ALERT") {
			t.Errorf("alert printed for a skill command this library did not add:\n%s", out)
		}
	})
}

func TestSkillAlertIsForAgentsOnly(t *testing.T) {
	tests := []struct {
		name     string
		agentEnv string
		opt      HelpOptions
		args     []string
		want     string
	}{
		{"human help", "0", HelpOptions{Skill: sampleSkill}, []string{"--help"}, "Available Commands:"},
		{"agent mode disabled", "1", HelpOptions{Skill: sampleSkill, DisableAgent: true}, []string{"--help"}, "Available Commands:"},
		{"the version", "1", HelpOptions{Skill: sampleSkill}, []string{"--version"}, "app version 1.2.3\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AGENT", tt.agentEnv)
			t.Setenv("COLUMNS", "200")

			root := buildSampleCommandHierarchy()
			root.Version = "1.2.3"
			if err := SetupWithOptions(root, tt.opt); err != nil {
				t.Fatalf("SetupWithOptions: %v", err)
			}
			out := new(bytes.Buffer)
			root.SetOut(out)
			root.SetErr(out)
			root.SetArgs(tt.args)
			if err := root.Execute(); err != nil {
				t.Fatalf("%v: %v", tt.args, err)
			}

			if !strings.Contains(out.String(), tt.want) {
				t.Errorf("output missing %q:\n%s", tt.want, out.String())
			}
			if strings.Contains(out.String(), "ALERT") {
				t.Errorf("alert printed outside agent help:\n%s", out.String())
			}
		})
	}
}

func TestSetupWithSkillRejectsACommandThatCannotCarryIt(t *testing.T) {
	t.Setenv("AGENT", "0")
	t.Setenv("COLUMNS", "200")

	tests := []struct {
		name    string
		build   func() (target, root *cobra.Command)
		wantErr string
	}{
		{
			name: "a root that already has a skill command",
			build: func() (*cobra.Command, *cobra.Command) {
				root := buildTwoCommandRoot()
				root.AddCommand(leafCommand("skill", "Manage skills"))
				return root, root
			},
			wantErr: "already has a skill command",
		},
		{
			name: "a root with a command aliased to skill",
			build: func() (*cobra.Command, *cobra.Command) {
				root := buildTwoCommandRoot()
				aliased := leafCommand("guide", "Print the guide")
				aliased.Aliases = []string{"skill"}
				root.AddCommand(aliased)
				return root, root
			},
			wantErr: "already has a skill command",
		},
		{
			name: "a command below the root",
			build: func() (*cobra.Command, *cobra.Command) {
				root := buildTwoCommandRoot()
				group := &cobra.Command{Use: "group", Short: "A group"}
				group.AddCommand(leafCommand("child", "A child"))
				root.AddCommand(group)
				return group, root
			},
			wantErr: "root command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, root := tt.build()

			err := SetupWithOptions(target, HelpOptions{Skill: sampleSkill})
			if err == nil {
				t.Fatal("SetupWithOptions accepted a command that cannot carry the skill")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not say %q", err, tt.wantErr)
			}

			// A rejected call must not install a help function, leaving cobra's default.
			if out := helpOutput(t, root); strings.Contains(out, "├─ add") {
				t.Errorf("rejected skill still installed the tree help function:\n%s", out)
			}
		})
	}
}

// buildSkillFixture returns a small CLI holding one of everything a skill has to
// mention and one of everything it may leave out, with the catalog documenting it.
func buildSkillFixture() (*cobra.Command, TechCatalog) {
	root := &cobra.Command{Use: "app", Short: "App"}
	root.PersistentFlags().StringP("config", "c", "", "Config path")

	create := leafCommand("create <name>", "Create a user")
	create.Flags().Bool("admin", false, "Grant admin rights")
	create.Flags().String("trace", "", "Internal tracing")
	// MarkHidden fails only for a flag that does not exist, and this one was just defined.
	_ = create.Flags().MarkHidden("trace")

	user := &cobra.Command{Use: "user", Short: "Manage users"}
	user.AddCommand(create)
	root.AddCommand(user)

	internal := leafCommand("internal", "Not for users")
	internal.Hidden = true
	root.AddCommand(internal)

	old := leafCommand("old", "Superseded")
	old.Deprecated = "use user create"
	root.AddCommand(old)

	catalog := TechCatalog{
		"app user create": TechInfo{
			Args: []ArgSpec{{Name: "<name>", Description: "Login name"}},
			Env:  []EnvSpec{{Name: "APP_ROLE", Description: "Role given to new users"}},
		},
	}
	return root, catalog
}

// completeSkillLines is a skill that mentions the whole interface of the fixture,
// one mention per line so a test can take any one of them away.
var completeSkillLines = []string{
	"# app",
	"Run app user create <name> to add a user.",
	"Pass --admin to grant admin rights.",
	"Set APP_ROLE to choose the role.",
	"Every command accepts --config (-c)",
	"and --help (-h).",
	"app help prints the help of a command.",
	"app completion bash, app completion fish,",
	"app completion powershell and",
	"app completion zsh print a shell script; --no-descriptions shortens it.",
}

func TestSkillOmissions(t *testing.T) {
	t.Run("a skill naming the whole interface omits nothing", func(t *testing.T) {
		root, catalog := buildSkillFixture()
		if got := SkillOmissions(root, catalog, strings.Join(completeSkillLines, "\n")); len(got) != 0 {
			t.Errorf("complete skill reported omissions: %q", got)
		}
	})

	t.Run("an empty skill omits the whole interface and nothing hidden", func(t *testing.T) {
		root, catalog := buildSkillFixture()
		want := []string{
			"argument <name> of app user create",
			"command app",
			"command app completion",
			"command app completion bash",
			"command app completion fish",
			"command app completion powershell",
			"command app completion zsh",
			"command app help",
			"command app user",
			"command app user create",
			"environment variable APP_ROLE",
			"flag --admin",
			"flag --config",
			"flag --help",
			"flag --no-descriptions",
			"flag -c, the shorthand of --config",
			"flag -h, the shorthand of --help",
		}
		if got := SkillOmissions(root, catalog, ""); !slices.Equal(got, want) {
			t.Errorf("SkillOmissions = %q, want %q", got, want)
		}
	})

	// Each case rewrites one line of the complete skill and names the one omission
	// that follows from it.
	tests := []struct {
		name    string
		line    string
		rewrite string
		want    string
	}{
		{"a command", "app completion zsh print a shell script; --no-descriptions shortens it.", "--no-descriptions shortens it.", "command app completion zsh"},
		{"the generated help command", "app help prints the help of a command.", "", "command app help"},
		{"a flag", "Pass --admin to grant admin rights.", "", "flag --admin"},
		{"a flag named only inside a longer one", "Pass --admin to grant admin rights.", "Pass --administrator to grant admin rights.", "flag --admin"},
		{"a shorthand", "Every command accepts --config (-c)", "Every command accepts --config", "flag -c, the shorthand of --config"},
		{"a shorthand named only as the start of a word", "Every command accepts --config (-c)", "Every command accepts --config (-cfg)", "flag -c, the shorthand of --config"},
		{"an argument", "Run app user create <name> to add a user.", "Run app user create NAME to add a user.", "argument <name> of app user create"},
		{"an environment variable", "Set APP_ROLE to choose the role.", "Set APP_ROLES to choose the role.", "environment variable APP_ROLE"},
	}

	for _, tt := range tests {
		t.Run("omitting "+tt.name, func(t *testing.T) {
			root, catalog := buildSkillFixture()

			lines := slices.Clone(completeSkillLines)
			at := slices.Index(lines, tt.line)
			if at < 0 {
				t.Fatalf("the complete skill has no line %q", tt.line)
			}
			lines[at] = tt.rewrite

			got := SkillOmissions(root, catalog, strings.Join(lines, "\n"))
			if !slices.Equal(got, []string{tt.want}) {
				t.Errorf("SkillOmissions = %q, want only %q", got, tt.want)
			}
		})
	}

	t.Run("a nil command has no interface", func(t *testing.T) {
		if got := SkillOmissions(nil, nil, ""); got != nil {
			t.Errorf("SkillOmissions(nil) = %q, want nil", got)
		}
	})

	t.Run("the skill command is part of the interface", func(t *testing.T) {
		root, catalog := buildSkillFixture()
		skill := strings.Join(completeSkillLines, "\n")
		if err := SetupWithOptions(root, HelpOptions{Catalog: catalog, Skill: skill}); err != nil {
			t.Fatalf("SetupWithOptions: %v", err)
		}
		if got, want := SkillOmissions(root, catalog, skill), []string{"command app skill"}; !slices.Equal(got, want) {
			t.Errorf("SkillOmissions = %q, want %q", got, want)
		}
	})
}
