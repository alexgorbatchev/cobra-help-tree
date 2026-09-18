package cobrahelptree

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

func TestRenderTreeHelp(t *testing.T) {
	root := buildSampleCommandHierarchy()
	root.Example = "  app playlist list\n  app track add song.mp3"

	// 1. Root Help
	helpText := RenderTreeHelp(root, nil, TreeOptions{})
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
	childHelp := RenderTreeHelp(playlistCmd, nil, TreeOptions{})
	if !strings.Contains(childHelp, "Available Commands:") {
		t.Errorf("childHelp missing 'Available Commands:', got:\n%s", childHelp)
	}
	if !strings.Contains(childHelp, "Global Flags:") {
		t.Errorf("childHelp missing Global Flags section, got:\n%s", childHelp)
	}

	// 3. Nil command
	if got := RenderTreeHelp(nil, nil, TreeOptions{}); got != "" {
		t.Errorf("expected empty string for nil command, got %q", got)
	}

	// 4. Short-only command without Long
	shortCmd := &cobra.Command{
		Use:   "simple",
		Short: "A simple command",
	}
	shortHelp := RenderTreeHelp(shortCmd, nil, TreeOptions{})
	if !strings.Contains(shortHelp, "A simple command") {
		t.Errorf("expected short description in help, got:\n%s", shortHelp)
	}
}

func TestRenderTreeUsageRendersArguments(t *testing.T) {
	root := buildSampleCommandHierarchy()
	create := findCommand(t, root, "app playlist create")

	out := RenderTreeUsage(create, argCatalog(), TreeOptions{TerminalWidth: 200})

	if !strings.Contains(out, "\nArguments:\n") {
		t.Fatalf("usage missing the Arguments section:\n%s", out)
	}
	for _, label := range []string{"\n  <name>", "\n  [parent]"} {
		if !strings.Contains(out, label) {
			t.Errorf("usage missing argument label %q:\n%s", label, out)
		}
	}

	// The arguments block obeys the tree's alignment rule: one description column,
	// floored at MinLabelWidth + MinPadding.
	wantColumn := defaultMinLabelWidth + defaultMinPadding
	for _, desc := range []string{"Name of the new playlist", "Folder to create it under"} {
		if got := descColumn(out, desc); got != wantColumn {
			t.Errorf("argument description %q starts at column %d, want %d:\n%s", desc, got, wantColumn, out)
		}
	}
}

func TestRenderTreeHelpAlignsArgumentsWithCommandTree(t *testing.T) {
	root := buildSampleCommandHierarchy()
	playlist := findCommand(t, root, "app playlist")

	// A command that has both arguments and subcommands renders two blocks on one
	// screen. They share MinLabelWidth, so their description columns must match.
	out := RenderTreeHelp(playlist, argCatalog(), TreeOptions{TerminalWidth: 200})

	argColumn := descColumn(out, "Playlist name or id")
	treeColumn := descColumn(out, "Display all playlists")
	if argColumn != treeColumn {
		t.Errorf("argument column %d and command column %d disagree:\n%s", argColumn, treeColumn, out)
	}
}

func TestRenderTreeUsageClipsArgumentDescriptions(t *testing.T) {
	const termWidth = 40

	root := buildSampleCommandHierarchy()
	create := findCommand(t, root, "app playlist create")
	cat := TechCatalog{
		"app playlist create": TechInfo{Args: []ArgSpec{{
			Name:        "<name>",
			Description: strings.Repeat("very long argument description ", 10),
		}}},
	}

	out := RenderTreeUsage(create, cat, TreeOptions{TerminalWidth: termWidth})

	var argLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "  <name>") {
			argLine = line
		}
	}
	if argLine == "" {
		t.Fatalf("argument line missing from usage:\n%s", out)
	}
	if !strings.HasSuffix(argLine, "...") {
		t.Errorf("expected the argument description clipped with an ellipsis, got %q", argLine)
	}
	if got := runewidth.StringWidth(argLine); got > termWidth {
		t.Errorf("argument line is %d cells wide, want at most %d: %q", got, termWidth, argLine)
	}
}

func TestRenderTreeUsageArgumentsRequireACatalogEntry(t *testing.T) {
	root := buildSampleCommandHierarchy()
	create := findCommand(t, root, "app playlist create")

	tests := []struct {
		name string
		cat  TechCatalog
	}{
		{"nil catalog", nil},
		{"catalog without this command", TechCatalog{"app track add": TechInfo{Args: []ArgSpec{{Name: "<path>"}}}}},
		{"entry without arguments", TechCatalog{"app playlist create": TechInfo{Summary: "Create a playlist"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if out := RenderTreeUsage(create, tt.cat, TreeOptions{TerminalWidth: 200}); strings.Contains(out, "Arguments:") {
				t.Errorf("expected no Arguments section without catalog arguments:\n%s", out)
			}
		})
	}
}

func TestRenderTreeUsageOmitsTheLongDescription(t *testing.T) {
	root := buildSampleCommandHierarchy()

	// Cobra's help is the long description followed by the usage screen, and its
	// usage screen alone is what follows a flag error. The split has to hold here
	// too, or every error repeats the whole command description.
	help := RenderTreeHelp(root, nil, TreeOptions{TerminalWidth: 200})
	usage := RenderTreeUsage(root, nil, TreeOptions{TerminalWidth: 200})

	if !strings.HasPrefix(help, root.Long+"\n\n") {
		t.Errorf("help does not open with the long description:\n%s", help)
	}
	if !strings.HasSuffix(help, usage) {
		t.Errorf("help does not end with the usage screen:\n--- help ---\n%s\n--- usage ---\n%s", help, usage)
	}
	if strings.Contains(usage, root.Long) {
		t.Errorf("usage repeats the long description:\n%s", usage)
	}
	if got := RenderTreeUsage(nil, nil, TreeOptions{}); got != "" {
		t.Errorf("expected empty string for nil command, got %q", got)
	}
}

func TestRenderTreeUsageReportsDeprecation(t *testing.T) {
	// Cobra hides a deprecated command from its parent's help and prints the
	// Deprecated string only when the command runs, so its own help screen is the
	// one place a user who still has it scripted can be told before running it.
	old := &cobra.Command{
		Use:        "old <id>",
		Short:      "A superseded command",
		Deprecated: "use \"app new\" instead",
		Run:        func(*cobra.Command, []string) {},
	}

	usage := RenderTreeUsage(old, nil, TreeOptions{TerminalWidth: 200})
	want := "Deprecated: use \"app new\" instead\n"
	if !strings.Contains(usage, want) {
		t.Errorf("usage missing the deprecation notice %q:\n%s", want, usage)
	}
	if at := strings.Index(usage, want); at > strings.Index(usage, "Usage:") {
		t.Errorf("deprecation notice must precede the usage block:\n%s", usage)
	}

	help := RenderTreeHelp(old, nil, TreeOptions{TerminalWidth: 200})
	if !strings.Contains(help, want) {
		t.Errorf("help missing the deprecation notice %q:\n%s", want, help)
	}

	current := &cobra.Command{Use: "new", Short: "The replacement", Run: func(*cobra.Command, []string) {}}
	if got := RenderTreeUsage(current, nil, TreeOptions{TerminalWidth: 200}); strings.Contains(got, "Deprecated") {
		t.Errorf("a command that is not deprecated must carry no notice:\n%s", got)
	}
}
