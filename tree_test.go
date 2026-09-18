package cobrahelptree

import (
	"bytes"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func buildSampleCommandHierarchy() *cobra.Command {
	root := &cobra.Command{
		Use:   "app",
		Short: "Sample CLI application",
		Long:  "app is a sample CLI tool demonstrating nested command hierarchies.",
	}
	root.PersistentFlags().StringP("config", "c", "config.yaml", "Configuration file path")

	playlistCmd := &cobra.Command{
		Use:   "playlist",
		Short: "Inspect and manage playlists",
	}
	playlistCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Display all playlists",
	})
	playlistCmd.AddCommand(&cobra.Command{
		Use:   "create <name>",
		Short: "Create a playlist",
	})

	playlistTrackCmd := &cobra.Command{
		Use:   "track",
		Short: "Manage playlist track memberships",
	}
	playlistTrackCmd.AddCommand(&cobra.Command{
		Use:   "add <pl|id> <tr|id|path...>",
		Short: "Add track to playlist",
	})
	playlistTrackCmd.AddCommand(&cobra.Command{
		Use:   "rm <pl|id> <tr|id|path...>",
		Short: "Remove track from playlist",
	})
	playlistCmd.AddCommand(playlistTrackCmd)

	trackCmd := &cobra.Command{
		Use:   "track",
		Short: "Manage audio files",
	}
	trackCmd.AddCommand(&cobra.Command{
		Use:   "add <path...>",
		Short: "Import audio file",
	})

	root.AddCommand(playlistCmd)
	root.AddCommand(trackCmd)

	return root
}

func TestFormatCommandTree(t *testing.T) {
	root := buildSampleCommandHierarchy()

	// 1. Default formatting (direct subcommands start with ├─ / ╰─ at 0 indent)
	tree := FormatCommandTree(root, TreeOptions{})
	if strings.HasPrefix(tree, "app") {
		t.Errorf("expected tree without root command line, got:\n%s", tree)
	}
	if !strings.HasPrefix(tree, "├─ playlist") || !strings.Contains(tree, "\n│  ├─ list") {
		t.Errorf("expected playlist hierarchy in tree:\n%s", tree)
	}
	if !strings.Contains(tree, "\n│  ╰─ track") || !strings.Contains(tree, "\n│     ├─ add <pl|id> <tr|id|path...>") {
		t.Errorf("expected 3-level nesting in tree:\n%s", tree)
	}
	if !strings.Contains(tree, "\n╰─ track") || !strings.Contains(tree, "\n   ╰─ add <path...>") {
		t.Errorf("expected second top-level command attached with ╰─, got:\n%s", tree)
	}

	// 2. IncludeRoot: true option (root at top) and terminal width clipping
	treeWithRoot := FormatCommandTree(root, TreeOptions{IncludeRoot: true, MinPadding: 3, TerminalWidth: 50})
	if !strings.HasPrefix(treeWithRoot, "app") {
		t.Errorf("expected tree with root, got:\n%s", treeWithRoot)
	}
	if !strings.Contains(treeWithRoot, "...") {
		t.Errorf("expected clipped description with '...' for narrow width, got:\n%s", treeWithRoot)
	}

	// 3. Nil command
	if got := FormatCommandTree(nil, TreeOptions{}); got != "" {
		t.Errorf("expected empty string for nil command, got %q", got)
	}

	// 4. Command with no subcommands
	leaf := &cobra.Command{Use: "leaf", Short: "leaf command"}
	if got := FormatCommandTree(leaf, TreeOptions{}); got != "" {
		t.Errorf("expected empty string for leaf with default options, got %q", got)
	}
	if got := FormatCommandTree(leaf, TreeOptions{IncludeRoot: true}); !strings.HasPrefix(got, "leaf") {
		t.Errorf("expected root-only line for leaf with IncludeRoot: true, got %q", got)
	}
}

// withNonTerminalStdout points os.Stdout at a pipe for the duration of the test.
// A pipe is not a terminal, so term.GetSize always fails and the fallback branch
// of GetTerminalWidth becomes deterministic regardless of how the suite is run.
func withNonTerminalStdout(t *testing.T) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}

	orig := os.Stdout
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = orig
		if err := w.Close(); err != nil {
			t.Errorf("closing stdout pipe writer: %v", err)
		}
		if err := r.Close(); err != nil {
			t.Errorf("closing stdout pipe reader: %v", err)
		}
	})
}

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

// descColumn returns the display column at which short begins on the tree line
// containing it, or -1 when no line contains it.
func descColumn(tree, short string) int {
	for _, line := range strings.Split(strings.TrimRight(tree, "\n"), "\n") {
		if at := strings.Index(line, short); at >= 0 {
			return runewidth.StringWidth(line[:at])
		}
	}
	return -1
}

// buildTwoCommandRoot returns a root whose widest command column is "├─ add",
// six cells wide, which keeps expected description columns easy to state.
func buildTwoCommandRoot() *cobra.Command {
	root := &cobra.Command{Use: "app", Short: "App"}
	root.AddCommand(&cobra.Command{Use: "add", Short: "Add an item"})
	root.AddCommand(&cobra.Command{Use: "rm", Short: "Remove an item"})
	return root
}

const (
	widestCommand = 6
	padding       = 2
)

// helpOutput runs `--help` and returns what the help function wrote. It installs
// a writer via SetOut, so it cannot distinguish stdout from stderr; stream choice
// is covered separately by TestSetupWritesHelpToStdout.
func helpOutput(t *testing.T, root *cobra.Command) string {
	t.Helper()

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("root --help execution failed: %v", err)
	}
	return buf.String()
}

func TestFormatCommandTreeMinCommandWidth(t *testing.T) {
	root := buildTwoCommandRoot()

	tests := []struct {
		name            string
		minCommandWidth int
		wantColumn      int
	}{
		{"zero applies the default floor", 0, defaultMinCommandWidth + padding},
		{"floor below content tracks the widest command", 4, widestCommand + padding},
		{"floor above content widens the column", 30, 30 + padding},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := FormatCommandTree(root, TreeOptions{
				MinPadding:      padding,
				MinCommandWidth: tt.minCommandWidth,
				TerminalWidth:   200,
			})

			for _, short := range []string{"Add an item", "Remove an item"} {
				if got := descColumn(tree, short); got != tt.wantColumn {
					t.Errorf("description %q starts at column %d, want %d:\n%s",
						short, got, tt.wantColumn, tree)
				}
			}
		})
	}
}

func TestFormatCommandTreeAlignsByDisplayWidth(t *testing.T) {
	// "日本語" is 3 runes but occupies 6 terminal columns, and "🚀" is 1 rune in 2
	// columns. Padding by rune count instead of display width shifts the description
	// column right by one extra cell per wide character.
	shorts := map[string]string{
		"日本語":     "Wide command name",
		"ascii":   "Narrow command name",
		"emoji-🚀": "Emoji command name",
	}

	root := &cobra.Command{Use: "app", Short: "App"}
	for _, use := range slices.Sorted(maps.Keys(shorts)) {
		root.AddCommand(&cobra.Command{Use: use, Short: shorts[use]})
	}

	tree := FormatCommandTree(root, TreeOptions{TerminalWidth: 200})

	descColumns := make(map[int][]string)
	for _, line := range strings.Split(strings.TrimRight(tree, "\n"), "\n") {
		for _, short := range shorts {
			at := strings.Index(line, short)
			if at < 0 {
				continue
			}
			col := runewidth.StringWidth(line[:at])
			descColumns[col] = append(descColumns[col], line)
		}
	}

	if len(descColumns) != 1 {
		t.Errorf("descriptions start at %d different display columns, want 1:\n%s\ncolumns: %v",
			len(descColumns), tree, descColumns)
	}
	if got := len(shorts); len(descColumns) == 1 {
		for _, lines := range descColumns {
			if len(lines) != got {
				t.Errorf("expected %d aligned description lines, got %d:\n%s", got, len(lines), tree)
			}
		}
	}
}

func TestFormatCommandTreeTruncatesByDisplayWidth(t *testing.T) {
	const termWidth = 50

	root := &cobra.Command{Use: "app", Short: "App"}
	// 40 ideographs occupy 80 columns; truncating by rune count leaves twice the
	// intended number of cells and overruns the terminal.
	root.AddCommand(&cobra.Command{Use: "wide", Short: strings.Repeat("日", 40)})
	root.AddCommand(&cobra.Command{Use: "narrow", Short: strings.Repeat("x", 80)})

	tree := FormatCommandTree(root, TreeOptions{TerminalWidth: termWidth})

	lines := strings.Split(strings.TrimRight(tree, "\n"), "\n")
	for _, line := range lines {
		if w := runewidth.StringWidth(line); w > termWidth {
			t.Errorf("line occupies %d columns, exceeding terminal width %d: %q", w, termWidth, line)
		}
		if !strings.HasSuffix(line, "...") {
			t.Errorf("expected an over-long description to be clipped with an ellipsis: %q", line)
		}
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

func TestRenderTreeHelp(t *testing.T) {
	root := buildSampleCommandHierarchy()
	root.Example = "  app playlist list\n  app track add song.mp3"

	// 1. Root Help
	helpText := RenderTreeHelp(root, TreeOptions{})
	if !strings.Contains(helpText, "Available Commands:") {
		t.Errorf("helpText missing 'Available Commands:', got:\n%s", helpText)
	}
	if !strings.Contains(helpText, "├─ playlist") || !strings.Contains(helpText, "├─ add") {
		t.Errorf("helpText missing ASCII tree branches, got:\n%s", helpText)
	}
	if !strings.Contains(helpText, "Usage:\n  app [flags] [command]") && !strings.Contains(helpText, "Usage:\n  app [command]") {
		t.Errorf("helpText missing Usage section, got:\n%s", helpText)
	}
	if !strings.Contains(helpText, "Examples:") {
		t.Errorf("helpText missing Examples section, got:\n%s", helpText)
	}

	// 2. Child Command Help (with inherited flags)
	playlistCmd := root.Commands()[0]
	childHelp := RenderTreeHelp(playlistCmd, TreeOptions{})
	if !strings.Contains(childHelp, "Available Commands:") {
		t.Errorf("childHelp missing 'Available Commands:', got:\n%s", childHelp)
	}
	if !strings.Contains(childHelp, "Global Flags:") {
		t.Errorf("childHelp missing Global Flags section, got:\n%s", childHelp)
	}

	// 3. Nil command
	if got := RenderTreeHelp(nil, TreeOptions{}); got != "" {
		t.Errorf("expected empty string for nil command, got %q", got)
	}

	// 4. Short-only command without Long
	shortCmd := &cobra.Command{
		Use:   "simple",
		Short: "A simple command",
	}
	shortHelp := RenderTreeHelp(shortCmd, TreeOptions{})
	if !strings.Contains(shortHelp, "A simple command") {
		t.Errorf("expected short description in help, got:\n%s", shortHelp)
	}
}

func TestRenderAgentHelp(t *testing.T) {
	root := buildSampleCommandHierarchy()

	// 1. Nil check
	if got := RenderAgentHelp(nil, AgentOptions{}); got != "" {
		t.Errorf("expected empty string for nil command, got %q", got)
	}

	// 2. Default agent output
	agentOut := RenderAgentHelp(root, AgentOptions{})
	if !strings.Contains(agentOut, "command: app") || !strings.Contains(agentOut, "summary: Sample CLI application") {
		t.Errorf("unexpected agent help output:\n%s", agentOut)
	}
	if !strings.Contains(agentOut, "subcommands:\n  - playlist: Inspect and manage playlists") {
		t.Errorf("agent help missing subcommands:\n%s", agentOut)
	}

	// 3. With TechCatalog
	catalog := TechCatalog{
		"app": TechInfo{
			Summary:     "Overridden Summary",
			Description: "Overridden Description",
			Args:        "<required-arg>",
			MutatesDB:   true,
			AutoBackup:  true,
			Metadata: map[string]string{
				"custom_key": "custom_val",
			},
		},
	}

	catOut := RenderAgentHelp(root, AgentOptions{TechCatalog: catalog})
	if !strings.Contains(catOut, "summary: Overridden Summary") || !strings.Contains(catOut, "mutates_db: true") {
		t.Errorf("techCatalog not reflected in agent output:\n%s", catOut)
	}
	if !strings.Contains(catOut, "\nmetadata:\n  custom_key: custom_val\n") {
		t.Errorf("metadata not nested under a metadata: block in agent output:\n%s", catOut)
	}
}

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
	want := defaultMinCommandWidth + padding
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
	if err := SetupWithOptions(root, HelpOptions{Tree: TreeOptions{MinCommandWidth: 4, MinPadding: padding}}); err != nil {
		t.Fatalf("SetupWithOptions: %v", err)
	}

	out := helpOutput(t, root)
	want := widestCommand + padding
	if got := descColumn(out, "Add an item"); got != want {
		t.Errorf("MinCommandWidth did not reach the renderer: column %d, want %d:\n%s", got, want, out)
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
	if err := SetupWithOptions(root, HelpOptions{Agent: AgentOptions{TechCatalog: catalog}}); err != nil {
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

func TestRenderAgentHelpIncludesPersistentAndInheritedFlags(t *testing.T) {
	// cobra's Flags() does not merge persistent flags; only LocalFlags() and
	// InheritedFlags() call mergePersistentFlags. Rendering straight off Flags()
	// therefore drops persistent flags whenever nothing has merged them yet,
	// which is every direct call that has not gone through Execute.
	root := &cobra.Command{Use: "app", Short: "App"}
	root.PersistentFlags().StringP("config", "c", "cfg.yaml", "Config path")

	child := &cobra.Command{Use: "child", Short: "Child"}
	child.Flags().Bool("force", false, "Force it")
	root.AddCommand(child)

	t.Run("own persistent flag on the root", func(t *testing.T) {
		out := RenderAgentHelp(root, AgentOptions{})
		if !strings.Contains(out, "--config") {
			t.Errorf("persistent flag missing from agent output:\n%s", out)
		}
	})

	t.Run("inherited flag on a subcommand", func(t *testing.T) {
		out := RenderAgentHelp(child, AgentOptions{})
		if !strings.Contains(out, "--force") {
			t.Errorf("local flag missing from agent output:\n%s", out)
		}
		if !strings.Contains(out, "--config") {
			t.Errorf("inherited flag missing from agent output:\n%s", out)
		}
	})

	t.Run("first render matches later ones", func(t *testing.T) {
		// Collecting flags merges cobra's persistent flag sets, which flips
		// HasAvailableFlags and so changes UseLine. Unless that happens before the
		// usage line is built, the first render differs from every later one.
		fresh := &cobra.Command{Use: "app", Short: "App"}
		fresh.PersistentFlags().String("token", "", "API token")

		first := RenderAgentHelp(fresh, AgentOptions{})
		second := RenderAgentHelp(fresh, AgentOptions{})
		if first != second {
			t.Errorf("render is not idempotent:\n--- first ---\n%s\n--- second ---\n%s", first, second)
		}
	})

	t.Run("agrees with the human renderer", func(t *testing.T) {
		// Both renderers must report the same set of applicable flags.
		fresh := &cobra.Command{Use: "app", Short: "App"}
		fresh.PersistentFlags().String("token", "", "API token")
		freshChild := &cobra.Command{Use: "child", Short: "Child"}
		fresh.AddCommand(freshChild)

		agent := RenderAgentHelp(freshChild, AgentOptions{})
		human := RenderTreeHelp(freshChild, TreeOptions{})
		if strings.Contains(human, "--token") != strings.Contains(agent, "--token") {
			t.Errorf("renderers disagree on --token\n--- agent ---\n%s\n--- human ---\n%s", agent, human)
		}
	})
}

func TestRenderersReportPflagGlobals(t *testing.T) {
	// Cobra folds pflag.CommandLine into every root's persistent flags, so a flag
	// registered on pflag's global set really is accepted by the CLI. Help must
	// therefore list it: hiding it would document an interface the binary does not
	// have. Do not "fix" this by filtering. Note this covers pflag's global set,
	// not the standard library's flag package, which cobra never consults.
	saved := pflag.CommandLine
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
	t.Cleanup(func() { pflag.CommandLine = saved })
	pflag.String("injected-global", "", "Registered on pflag.CommandLine")

	root := &cobra.Command{Use: "app", Short: "App"}
	root.Flags().String("declared", "", "Declared by the author")

	human := RenderTreeHelp(root, TreeOptions{TerminalWidth: 200})
	agent := RenderAgentHelp(root, AgentOptions{})

	for name, out := range map[string]string{"human": human, "agent": agent} {
		if !strings.Contains(out, "--declared") {
			t.Errorf("%s output dropped the author's own flag:\n%s", name, out)
		}
		if !strings.Contains(out, "--injected-global") {
			t.Errorf("%s output hid a pflag global the CLI actually accepts:\n%s", name, out)
		}
	}
}

func TestRenderAgentHelpIsUnclippedByDefault(t *testing.T) {
	// The agent-mode contract is full untruncated content, so neither a narrow
	// terminal nor $COLUMNS may clip it. Only an explicit MaxLineWidth can.
	t.Setenv("COLUMNS", "20")
	withNonTerminalStdout(t)

	long := strings.Repeat("x", 300)
	root := &cobra.Command{Use: "app", Short: long}

	out := RenderAgentHelp(root, AgentOptions{})
	if !strings.Contains(out, long) {
		t.Errorf("agent output was clipped without an explicit MaxLineWidth:\n%s", out)
	}
	if strings.Contains(out, ellipsis) {
		t.Errorf("agent output contains an ellipsis by default:\n%s", out)
	}
}

func TestRenderAgentHelpMaxLineWidth(t *testing.T) {
	const maxWidth = 40

	root := &cobra.Command{Use: "app", Short: strings.Repeat("x", 300)}
	catalog := TechCatalog{
		"app": TechInfo{
			// A wide-character value confirms clipping counts cells, not runes.
			Metadata: map[string]string{"note": strings.Repeat("日", 60)},
		},
	}

	out := RenderAgentHelp(root, AgentOptions{TechCatalog: catalog, MaxLineWidth: maxWidth})

	// Every key in this fixture is narrow enough to leave room for a value, so
	// every line fits. The case where a key is too wide to clip around is covered
	// by TestRenderAgentHelpClipNeverStrandsAKey.
	var clipped int
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if w := runewidth.StringWidth(line); w > maxWidth {
			t.Errorf("line occupies %d cells, exceeding MaxLineWidth %d: %q", w, maxWidth, line)
		}
		if strings.HasSuffix(line, ellipsis) {
			clipped++
		}
	}
	if clipped < 2 {
		t.Errorf("expected the long summary and the wide metadata value to be clipped, got %d clipped lines:\n%s", clipped, out)
	}
}

func TestRenderAgentHelpClipNeverStrandsAKey(t *testing.T) {
	// Agent output is parsed by machine, so clipping must not leave a key whose
	// value is nothing but an ellipsis: that reads as a field while carrying no
	// recoverable data. Keys too wide to leave room for a value stay unclipped.
	const maxWidth = 24
	longValue := strings.Repeat("v", 80)

	root := &cobra.Command{Use: "app", Short: "App"}
	catalog := TechCatalog{
		"app": TechInfo{
			Metadata: map[string]string{
				"note":                          longValue, // room for a value
				"dangling_key_here":             longValue, // key + ellipsis exactly fills the budget
				"a_very_long_metadata_key_name": longValue, // key alone overruns the budget
			},
		},
	}

	out := RenderAgentHelp(root, AgentOptions{TechCatalog: catalog, MaxLineWidth: maxWidth})

	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if !strings.HasSuffix(line, ellipsis) {
			continue
		}
		sep := strings.Index(line, ": ")
		if sep < 0 {
			t.Errorf("clipped a line carrying no key: %q", line)
			continue
		}
		value := strings.TrimSuffix(line[sep+2:], ellipsis)
		if strings.TrimSpace(value) == "" {
			t.Errorf("clipping stranded key %q with no value: %q", line[:sep], line)
		}
	}

	// The two wide keys keep their full value rather than becoming dangling keys.
	for _, key := range []string{"dangling_key_here", "a_very_long_metadata_key_name"} {
		want := "  " + key + ": " + longValue
		if !strings.Contains(out, want) {
			t.Errorf("expected %q to stay unclipped, got:\n%s", key, out)
		}
	}

	// A key with room to spare is still clipped, so the option keeps working.
	noteClipped := false
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "  note: ") {
			noteClipped = strings.HasSuffix(line, ellipsis) && runewidth.StringWidth(line) <= maxWidth
		}
	}
	if !noteClipped {
		t.Errorf("expected the narrow key to be clipped within %d cells:\n%s", maxWidth, out)
	}
}

func TestRenderAgentHelpClipPreservesBlockHeaders(t *testing.T) {
	// A block opener carries no value to shorten, so clipping one would destroy
	// the structure of the document rather than trim a field. Even an absurdly
	// narrow MaxLineWidth must leave these intact.
	root := &cobra.Command{Use: "app", Short: "App"}
	root.Flags().Bool("x", false, "X")
	root.AddCommand(&cobra.Command{Use: "sub", Short: "Sub"})
	catalog := TechCatalog{"app": TechInfo{Metadata: map[string]string{"k": "v"}}}

	out := RenderAgentHelp(root, AgentOptions{TechCatalog: catalog, MaxLineWidth: 6})

	for _, header := range []string{"metadata:", "subcommands:", "flags:"} {
		if !strings.Contains(out, "\n"+header+"\n") {
			t.Errorf("block header %q was clipped away:\n%s", header, out)
		}
	}
}

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

func TestHelpOptionsValidateDelegatesToTree(t *testing.T) {
	if err := (HelpOptions{}).Validate(); err != nil {
		t.Errorf("zero HelpOptions should be valid, got: %v", err)
	}

	err := HelpOptions{Tree: TreeOptions{MinCommandWidth: -3}}.Validate()
	if err == nil {
		t.Fatal("expected HelpOptions.Validate to surface the nested TreeOptions error")
	}
	if !strings.Contains(err.Error(), "MinCommandWidth") {
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

func TestTreeOptionsValidate(t *testing.T) {
	tests := []struct {
		name      string
		opt       TreeOptions
		wantField string
	}{
		{"zero value is valid", TreeOptions{}, ""},
		{"populated value is valid", TreeOptions{MinPadding: 4, MinCommandWidth: 12, TerminalWidth: 100}, ""},
		{"negative MinPadding", TreeOptions{MinPadding: -1}, "MinPadding"},
		{"negative MinCommandWidth", TreeOptions{MinCommandWidth: -1}, "MinCommandWidth"},
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

// captureStdio swaps the real os.Stdout and os.Stderr for pipes while fn runs.
// Tests that install a writer via cmd.SetOut cannot tell stdout from stderr,
// because cobra's OutOrStdout and OutOrStderr both return that same writer.
// Replacing the process streams is the only way to exercise the fallback that
// actually ships to callers.
func captureStdio(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stderr pipe: %v", err)
	}

	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	restore := func() { os.Stdout, os.Stderr = origOut, origErr }
	t.Cleanup(restore)

	// Drain both pipes concurrently so a writer can never block on a full buffer.
	var wg sync.WaitGroup
	var outBuf, errBuf bytes.Buffer
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&outBuf, outR) // ends at EOF once the write end closes
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&errBuf, errR)
	}()

	fn()

	restore()
	if err := outW.Close(); err != nil {
		t.Fatalf("closing stdout pipe: %v", err)
	}
	if err := errW.Close(); err != nil {
		t.Fatalf("closing stderr pipe: %v", err)
	}
	wg.Wait()

	return outBuf.String(), errBuf.String()
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

// failingWriter rejects every write, standing in for a closed pipe or full disk.
type failingWriter struct{ err error }

func (w failingWriter) Write(p []byte) (int, error) { return 0, w.err }

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

// countTopLevelKeys counts lines that open a key at column 0, ignoring nested entries.
func countTopLevelKeys(out, key string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, key+": ") {
			n++
		}
	}
	return n
}

func TestRenderAgentHelpMetadataIsNested(t *testing.T) {
	root := &cobra.Command{Use: "app", Short: "Sample CLI application"}
	catalog := TechCatalog{
		"app": TechInfo{
			Summary: "Sample CLI application",
			// "usage" and "summary" deliberately collide with reserved top-level keys.
			Metadata: map[string]string{
				"usage":   "42 calls/min",
				"summary": "quota exhausted",
				"region":  "us-east-1",
			},
		},
	}

	out := RenderAgentHelp(root, AgentOptions{TechCatalog: catalog})

	if !strings.Contains(out, "\nmetadata:\n") {
		t.Fatalf("expected a nested metadata: block, got:\n%s", out)
	}

	nested := []string{
		"\n  region: us-east-1\n",
		"\n  summary: quota exhausted\n",
		"\n  usage: 42 calls/min\n",
	}
	for _, want := range nested {
		if !strings.Contains(out, want) {
			t.Errorf("expected nested metadata entry %q, got:\n%s", want, out)
		}
	}

	// Nesting exists so a metadata key can never shadow or duplicate a reserved key.
	for _, key := range []string{"command", "summary", "usage"} {
		if n := countTopLevelKeys(out, key); n != 1 {
			t.Errorf("expected exactly one top-level %q line, got %d in:\n%s", key, n, out)
		}
	}
	if strings.Contains(out, "\nusage: 42 calls/min\n") {
		t.Errorf("metadata value leaked into the top-level usage key:\n%s", out)
	}
	if strings.Contains(out, "\nsummary: quota exhausted\n") {
		t.Errorf("metadata value leaked into the top-level summary key:\n%s", out)
	}
}

func TestRenderAgentHelpMetadataIsDeterministic(t *testing.T) {
	// Keys are declared out of alphabetical order so a sorted render is not
	// accidentally satisfied by insertion order.
	sortedKeys := []string{"alpha_key", "beta_key", "delta_key", "gamma_key", "zeta_key"}
	catalog := TechCatalog{
		"app": TechInfo{
			Summary: "Sample CLI application",
			Metadata: map[string]string{
				"zeta_key":  "5",
				"gamma_key": "3",
				"alpha_key": "1",
				"delta_key": "4",
				"beta_key":  "2",
			},
		},
	}

	root := buildSampleCommandHierarchy()
	first := RenderAgentHelp(root, AgentOptions{TechCatalog: catalog})

	// Go randomizes map iteration per range statement, so repeated renders of the
	// same input surface any ordering instability with near certainty.
	const renders = 50
	for i := 1; i < renders; i++ {
		if got := RenderAgentHelp(root, AgentOptions{TechCatalog: catalog}); got != first {
			t.Fatalf("render %d differs from render 0:\n--- first ---\n%s\n--- got ---\n%s", i, first, got)
		}
	}

	prev := -1
	for _, k := range sortedKeys {
		at := strings.Index(first, "\n  "+k+": ")
		if at < 0 {
			t.Fatalf("metadata key %q missing from output:\n%s", k, first)
		}
		if at < prev {
			t.Errorf("metadata key %q is out of alphabetical order:\n%s", k, first)
		}
		prev = at
	}
}
