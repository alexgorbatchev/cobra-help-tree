package cobrahelptree

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

func TestSetup(t *testing.T) {
	root := buildSampleCommandHierarchy()

	// 1. Setup in human mode
	t.Setenv("AGENT", "0")
	if err := Setup(root); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("root --help execution failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Available Commands:") || !strings.Contains(out, "├─ playlist") {
		t.Errorf("expected tree help screen from Setup, got:\n%s", out)
	}

	// 2. Setup in agent mode
	t.Setenv("AGENT", "1")
	bufAgent := new(bytes.Buffer)
	root.SetOut(bufAgent)
	root.SetErr(bufAgent)
	root.SetArgs([]string{"--help"})

	err = root.Execute()
	if err != nil {
		t.Fatalf("root --help in agent mode failed: %v", err)
	}

	outAgent := bufAgent.String()
	if !strings.Contains(outAgent, "command: app") || strings.Contains(outAgent, "Available Commands:") {
		t.Errorf("expected agent mode output from Setup, got:\n%s", outAgent)
	}
}

func TestSetupUsesDefaultOptions(t *testing.T) {
	t.Setenv("AGENT", "0")
	t.Setenv("COLUMNS", "200")

	root := buildTwoCommandRoot()
	if err := Setup(root); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	out := helpOutput(t, root)
	want := defaultMinLabelWidth + padding
	if got := descColumn(out, "Add an item"); got != want {
		t.Errorf("Setup put the description column at %d, want the default %d:\n%s", got, want, out)
	}
}

// TestSetupMatchesZeroOptions locks the invariant that replaced the exported
// DefaultOptions var: the defaults live in TreeOptions.resolve, so the
// convenience entry point cannot drift from an explicit zero-value call.
func TestSetupMatchesZeroOptions(t *testing.T) {
	t.Setenv("AGENT", "0")
	t.Setenv("COLUMNS", "200")

	withSetup := buildTwoCommandRoot()
	if err := Setup(withSetup); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	withZero := buildTwoCommandRoot()
	if err := SetupWithOptions(withZero, HelpOptions{}); err != nil {
		t.Fatalf("SetupWithOptions: %v", err)
	}

	if got, want := helpOutput(t, withSetup), helpOutput(t, withZero); got != want {
		t.Errorf("Setup and SetupWithOptions(TreeOptions{}) disagree:\n--- Setup ---\n%s\n--- zero ---\n%s", got, want)
	}
}

func TestSetupWithOptionsPassesOptionsToRenderer(t *testing.T) {
	t.Setenv("AGENT", "0")
	t.Setenv("COLUMNS", "200")

	root := buildTwoCommandRoot()
	// Executing the command makes cobra generate its completion command, whose
	// shell subtree is wider than either fixture command and would set the column
	// on its own. Hiding it keeps the expected column a statement about the two
	// commands under test, and passes a second option through to the renderer.
	opt := TreeOptions{MinLabelWidth: 4, MinPadding: padding, HideGeneratedCommands: true}
	if err := SetupWithOptions(root, HelpOptions{Tree: opt}); err != nil {
		t.Fatalf("SetupWithOptions: %v", err)
	}

	out := helpOutput(t, root)
	want := widestCommand + padding
	if got := descColumn(out, "Add an item"); got != want {
		t.Errorf("MinLabelWidth did not reach the renderer: column %d, want %d:\n%s", got, want, out)
	}
	if strings.Contains(out, "completion") {
		t.Errorf("HideGeneratedCommands did not reach the renderer:\n%s", out)
	}
}

func TestSetupWithOptionsDisableAgentOverridesEnvironment(t *testing.T) {
	t.Setenv("AGENT", "1")
	t.Setenv("COLUMNS", "200")

	root := buildTwoCommandRoot()
	if err := SetupWithOptions(root, HelpOptions{DisableAgent: true}); err != nil {
		t.Fatalf("SetupWithOptions: %v", err)
	}

	out := helpOutput(t, root)
	if !strings.Contains(out, "Available Commands:") {
		t.Errorf("expected human-mode help despite AGENT=1, got:\n%s", out)
	}
	if strings.Contains(out, "command: app") {
		t.Errorf("DisableAgent did not suppress agent mode, got:\n%s", out)
	}
}

func TestSetupWithOptionsPassesCatalogToAgentMode(t *testing.T) {
	t.Setenv("AGENT", "1")
	t.Setenv("COLUMNS", "200")

	root := buildTwoCommandRoot()
	catalog := TechCatalog{
		"app": TechInfo{
			Summary:  "Catalog summary",
			Metadata: map[string]string{"region": "us-east-1"},
		},
	}
	if err := SetupWithOptions(root, HelpOptions{Catalog: catalog}); err != nil {
		t.Fatalf("SetupWithOptions: %v", err)
	}

	out := helpOutput(t, root)
	if !strings.Contains(out, "summary: Catalog summary") {
		t.Errorf("catalog did not reach agent-mode rendering:\n%s", out)
	}
	if !strings.Contains(out, "\nmetadata:\n  region: us-east-1\n") {
		t.Errorf("catalog metadata missing from agent output:\n%s", out)
	}
}

func TestSetupWithOptionsPassesMaxLineWidthToAgentMode(t *testing.T) {
	t.Setenv("AGENT", "1")

	const maxWidth = 30
	root := &cobra.Command{Use: "app", Short: strings.Repeat("x", 300)}
	if err := SetupWithOptions(root, HelpOptions{Agent: AgentOptions{MaxLineWidth: maxWidth}}); err != nil {
		t.Fatalf("SetupWithOptions: %v", err)
	}

	for _, line := range strings.Split(strings.TrimRight(helpOutput(t, root), "\n"), "\n") {
		if w := runewidth.StringWidth(line); w > maxWidth {
			t.Errorf("MaxLineWidth did not reach agent rendering: %d cells in %q", w, line)
		}
	}
}

func TestSetupRejectsNilCommand(t *testing.T) {
	// A nil root command is a caller mistake, and both entry points have an error
	// channel, so neither may swallow it.
	if err := SetupWithOptions(nil, HelpOptions{}); err == nil {
		t.Error("SetupWithOptions(nil) returned no error")
	} else if !strings.Contains(err.Error(), "nil") {
		t.Errorf("error %q does not mention the nil command", err)
	}

	if err := Setup(nil); err == nil {
		t.Error("Setup(nil) returned no error")
	}
}

func TestSetupWithOptionsRejectsInvalidOptions(t *testing.T) {
	t.Setenv("AGENT", "0")
	t.Setenv("COLUMNS", "200")

	root := buildTwoCommandRoot()
	err := SetupWithOptions(root, HelpOptions{Tree: TreeOptions{MinPadding: -1}})
	if err == nil {
		t.Fatal("expected SetupWithOptions to reject a negative MinPadding")
	}
	if !strings.Contains(err.Error(), "MinPadding") {
		t.Errorf("error %q does not name the offending field", err)
	}

	// A rejected call must not install a help function, leaving cobra's default.
	out := helpOutput(t, root)
	if strings.Contains(out, "├─ add") {
		t.Errorf("rejected options still installed the tree help function:\n%s", out)
	}
}

func TestSetupWritesHelpToStdout(t *testing.T) {
	tests := []struct {
		name     string
		agentEnv string
		want     string
		notWant  string
	}{
		{
			name:     "human mode",
			agentEnv: "0",
			want:     "Available Commands:",
			notWant:  "command: app",
		},
		{
			name:     "agent mode",
			agentEnv: "1",
			want:     "command: app",
			notWant:  "Available Commands:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AGENT", tt.agentEnv)
			t.Setenv("COLUMNS", "200") // keep descriptions untruncated and the run deterministic

			// No SetOut/SetErr here on purpose: the fallback writer is what is under test.
			root := buildSampleCommandHierarchy()
			if err := Setup(root); err != nil {
				t.Fatalf("Setup: %v", err)
			}
			root.SetArgs([]string{"--help"})

			var execErr error
			stdout, stderr := captureStdio(t, func() {
				execErr = root.Execute()
			})
			if execErr != nil {
				t.Fatalf("root --help execution failed: %v", execErr)
			}

			if !strings.Contains(stdout, tt.want) {
				t.Errorf("expected %q on stdout, got:\n%s", tt.want, stdout)
			}
			if strings.Contains(stdout, tt.notWant) {
				t.Errorf("unexpected %q on stdout, got:\n%s", tt.notWant, stdout)
			}
			if stderr != "" {
				t.Errorf("expected empty stderr for requested help, got:\n%s", stderr)
			}
		})
	}
}

func TestSetupReportsStdoutWriteFailure(t *testing.T) {
	t.Setenv("AGENT", "0")

	root := buildSampleCommandHierarchy()
	if err := Setup(root); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	stderrBuf := new(bytes.Buffer)
	root.SetOut(failingWriter{err: io.ErrClosedPipe})
	root.SetErr(stderrBuf)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("root --help execution failed: %v", err)
	}

	if !strings.Contains(stderrBuf.String(), io.ErrClosedPipe.Error()) {
		t.Errorf("expected the stdout write failure reported on stderr, got:\n%s", stderrBuf.String())
	}
}

func TestSetupReplacesUsageOnError(t *testing.T) {
	tests := []struct {
		name     string
		agentEnv string
		want     string
		notWant  string
	}{
		{
			name:     "human mode renders the tree",
			agentEnv: "0",
			want:     "├─ playlist",
			notWant:  "command: app",
		},
		{
			name:     "agent mode renders key-value lines",
			agentEnv: "1",
			want:     "command: app",
			notWant:  "├─ playlist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AGENT", tt.agentEnv)
			t.Setenv("COLUMNS", "200")

			// No SetOut/SetErr: cobra prints the usage screen after an error through
			// UsageFunc, and the stream it lands on is part of what is under test.
			root := buildSampleCommandHierarchy()
			if err := Setup(root); err != nil {
				t.Fatalf("Setup: %v", err)
			}
			root.SetArgs([]string{"--no-such-flag"})

			var execErr error
			stdout, stderr := captureStdio(t, func() {
				execErr = root.Execute()
			})
			if execErr == nil {
				t.Fatal("expected an error for an unknown flag")
			}

			if !strings.Contains(stderr, tt.want) {
				t.Errorf("expected %q on stderr, got:\n%s", tt.want, stderr)
			}
			if strings.Contains(stderr, tt.notWant) {
				t.Errorf("unexpected %q on stderr, got:\n%s", tt.notWant, stderr)
			}
			if stdout != "" {
				t.Errorf("expected empty stdout for an error, got:\n%s", stdout)
			}
		})
	}
}

func TestSetupUsageOmitsTheLongDescription(t *testing.T) {
	t.Setenv("AGENT", "0")
	t.Setenv("COLUMNS", "200")

	root := buildSampleCommandHierarchy()
	if err := Setup(root); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--no-such-flag"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected an error for an unknown flag")
	}

	if strings.Contains(buf.String(), root.Long) {
		t.Errorf("usage after an error repeats the long description:\n%s", buf.String())
	}
}

func TestSetupReportsUsageWriteFailure(t *testing.T) {
	t.Setenv("AGENT", "0")

	root := buildSampleCommandHierarchy()
	if err := Setup(root); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	// The failing writer goes in through SetOut, not SetErr: cobra's getOut backs
	// OutOrStderr as well, so SetOut is what the usage screen actually writes to.
	stderrBuf := new(bytes.Buffer)
	root.SetOut(failingWriter{err: io.ErrClosedPipe})
	root.SetErr(stderrBuf)

	if err := root.Usage(); !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("Usage() error = %v, want %v", err, io.ErrClosedPipe)
	}
	if !strings.Contains(stderrBuf.String(), io.ErrClosedPipe.Error()) {
		t.Errorf("expected the usage write failure reported on stderr, got:\n%s", stderrBuf.String())
	}
}

func TestSetupWithOptionsPassesCatalogToBothModes(t *testing.T) {
	cat := argCatalog()

	tests := []struct {
		name     string
		agentEnv string
		want     string
	}{
		{"human mode", "0", "Arguments:"},
		{"agent mode", "1", "args:\n  - <playlist>: Playlist name or id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AGENT", tt.agentEnv)
			t.Setenv("COLUMNS", "200")

			root := buildSampleCommandHierarchy()
			if err := SetupWithOptions(root, HelpOptions{Catalog: cat}); err != nil {
				t.Fatalf("SetupWithOptions: %v", err)
			}

			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs([]string{"playlist", "--help"})

			if err := root.Execute(); err != nil {
				t.Fatalf("playlist --help execution failed: %v", err)
			}
			if !strings.Contains(buf.String(), tt.want) {
				t.Errorf("expected %q in output, got:\n%s", tt.want, buf.String())
			}
		})
	}
}
