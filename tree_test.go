package cobrahelptree

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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
	tree := FormatCommandTree(root)
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
	if got := FormatCommandTree(nil); got != "" {
		t.Errorf("expected empty string for nil command, got %q", got)
	}

	// 4. Command with no subcommands
	leaf := &cobra.Command{Use: "leaf", Short: "leaf command"}
	if got := FormatCommandTree(leaf); got != "" {
		t.Errorf("expected empty string for leaf with default options, got %q", got)
	}
	if got := FormatCommandTree(leaf, TreeOptions{IncludeRoot: true}); !strings.HasPrefix(got, "leaf") {
		t.Errorf("expected root-only line for leaf with IncludeRoot: true, got %q", got)
	}
}

func TestGetTerminalWidth(t *testing.T) {
	t.Setenv("COLUMNS", "100")
	if w := GetTerminalWidth(); w != 100 {
		t.Errorf("expected width 100 from COLUMNS, got %d", w)
	}

	t.Setenv("COLUMNS", "invalid")
	_ = GetTerminalWidth()
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
	helpText := RenderTreeHelp(root)
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
	childHelp := RenderTreeHelp(playlistCmd)
	if !strings.Contains(childHelp, "Available Commands:") {
		t.Errorf("childHelp missing 'Available Commands:', got:\n%s", childHelp)
	}
	if !strings.Contains(childHelp, "Global Flags:") {
		t.Errorf("childHelp missing Global Flags section, got:\n%s", childHelp)
	}

	// 3. Nil command
	if got := RenderTreeHelp(nil); got != "" {
		t.Errorf("expected empty string for nil command, got %q", got)
	}

	// 4. Short-only command without Long
	shortCmd := &cobra.Command{
		Use:   "simple",
		Short: "A simple command",
	}
	shortHelp := RenderTreeHelp(shortCmd)
	if !strings.Contains(shortHelp, "A simple command") {
		t.Errorf("expected short description in help, got:\n%s", shortHelp)
	}
}

func TestRenderAgentHelp(t *testing.T) {
	root := buildSampleCommandHierarchy()

	// 1. Nil check
	if got := RenderAgentHelp(nil); got != "" {
		t.Errorf("expected empty string for nil command, got %q", got)
	}

	// 2. Default agent output
	agentOut := RenderAgentHelp(root)
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

	catOut := RenderAgentHelp(root, catalog)
	if !strings.Contains(catOut, "summary: Overridden Summary") || !strings.Contains(catOut, "mutates_db: true") {
		t.Errorf("techCatalog not reflected in agent output:\n%s", catOut)
	}
	if !strings.Contains(catOut, "custom_key: custom_val") {
		t.Errorf("metadata not reflected in agent output:\n%s", catOut)
	}
}

func TestSetup(t *testing.T) {
	root := buildSampleCommandHierarchy()

	// 1. Setup in human mode
	t.Setenv("AGENT", "0")
	Setup(root)

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

	// Setup nil check
	Setup(nil)
}
