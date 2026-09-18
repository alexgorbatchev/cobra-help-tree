package cobrahelptree

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

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

func TestFormatCommandTreeMinLabelWidth(t *testing.T) {
	root := buildTwoCommandRoot()

	tests := []struct {
		name          string
		minLabelWidth int
		wantColumn    int
	}{
		{"zero applies the default floor", 0, defaultMinLabelWidth + padding},
		{"floor below content tracks the widest command", 4, widestCommand + padding},
		{"floor above content widens the column", 30, 30 + padding},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := FormatCommandTree(root, TreeOptions{
				MinPadding:    padding,
				MinLabelWidth: tt.minLabelWidth,
				TerminalWidth: 200,
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
		root.AddCommand(leafCommand(use, shorts[use]))
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
	root.AddCommand(leafCommand("wide", strings.Repeat("日", 40)))
	root.AddCommand(leafCommand("narrow", strings.Repeat("x", 80)))

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

func TestRenderersAgreeOnVisibleSubcommands(t *testing.T) {
	root := &cobra.Command{Use: "app", Short: "Sample CLI application"}
	root.AddCommand(&cobra.Command{Use: "visible", Short: "A listed command", Run: func(*cobra.Command, []string) {}})
	root.AddCommand(&cobra.Command{Use: "secret", Short: "An unlisted command", Hidden: true, Run: func(*cobra.Command, []string) {}})
	// Cobra's own help drops both of these, so these renderers drop them too: a
	// deprecated command, and a group that is neither runnable nor the parent of a
	// runnable command and so does nothing when invoked.
	root.AddCommand(&cobra.Command{Use: "old", Short: "A superseded command", Deprecated: "use visible", Run: func(*cobra.Command, []string) {}})
	root.AddCommand(&cobra.Command{Use: "empty", Short: "A group with nothing under it"})
	// A group is listed on the strength of its children, not its own Run.
	group := &cobra.Command{Use: "group", Short: "A group with a runnable child"}
	group.AddCommand(&cobra.Command{Use: "child", Short: "A listed child", Run: func(*cobra.Command, []string) {}})
	root.AddCommand(group)

	// Added the way cobra adds them, so the test sees the real generated commands.
	// Cobra's help lists completion; its IsAvailableCommand excludes the generated
	// help command by identity and only cobra's template re-adds it by name, which
	// this library does not do, so help is absent from every screen below.
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()

	// The tree recurses, so it lists completion's shell children as well; the agent
	// screen and the usage line report only the direct children.
	wantNested := []string{"bash", "child", "completion", "fish", "group", "powershell", "visible", "zsh"}
	wantDirect := []string{"completion", "group", "visible"}

	screens := []struct {
		name   string
		screen string
		want   []string
	}{
		{"tree", FormatCommandTree(root, TreeOptions{TerminalWidth: 200}), wantNested},
		{"usage", RenderTreeUsage(root, nil, TreeOptions{TerminalWidth: 200}), wantNested},
		{"agent", RenderAgentHelp(root, nil, AgentOptions{}), wantDirect},
	}

	for _, tt := range screens {
		t.Run(tt.name, func(t *testing.T) {
			if got := listedCommands(tt.screen); !slices.Equal(got, tt.want) {
				t.Errorf("%s screen lists %v, want %v:\n%s", tt.name, got, tt.want, tt.screen)
			}
		})
	}

	// The usage line advertises subcommands only when some remain after filtering.
	usage := RenderTreeUsage(root, nil, TreeOptions{TerminalWidth: 200})
	if !strings.Contains(usage, "app [command]") {
		t.Errorf("usage line does not advertise subcommands:\n%s", usage)
	}
	hiddenOnly := &cobra.Command{Use: "leaf", Short: "No visible children"}
	hiddenOnly.AddCommand(&cobra.Command{Use: "secret", Short: "Unlisted", Hidden: true, Run: func(*cobra.Command, []string) {}})
	if got := RenderTreeUsage(hiddenOnly, nil, TreeOptions{TerminalWidth: 200}); strings.Contains(got, "[command]") {
		t.Errorf("usage line advertises subcommands although every child is hidden:\n%s", got)
	}
}

func TestHideGeneratedCommands(t *testing.T) {
	root := &cobra.Command{Use: "app", Short: "Sample CLI application"}
	root.AddCommand(leafCommand("visible", "A listed command"))
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()

	hide := TreeOptions{TerminalWidth: 200, HideGeneratedCommands: true}

	t.Run("human screens drop the generated command and its subtree", func(t *testing.T) {
		want := []string{"visible"}
		for name, screen := range map[string]string{
			"tree":  FormatCommandTree(root, hide),
			"usage": RenderTreeUsage(root, nil, hide),
		} {
			if got := listedCommands(screen); !slices.Equal(got, want) {
				t.Errorf("%s screen lists %v, want %v:\n%s", name, got, want, screen)
			}
		}
	})

	// Agent output describes the interface a machine may drive, and completion is a
	// command the binary really accepts, so agent mode reports cobra's availability
	// verdict and nothing else. The option is human-mode formatting, which is why it
	// lives on TreeOptions alone.
	t.Run("agent output keeps reporting the whole interface", func(t *testing.T) {
		want := []string{"completion", "visible"}
		if got := listedCommands(RenderAgentHelp(root, nil, AgentOptions{})); !slices.Equal(got, want) {
			t.Errorf("agent screen lists %v, want %v", got, want)
		}
	})

	// The filter aims at the command cobra generates, which is always a direct child
	// of the root. A command of the CLI's own that happens to be named "completion"
	// deeper in the tree is the CLI's interface and stays listed.
	t.Run("a nested command of the same name survives", func(t *testing.T) {
		nested := &cobra.Command{Use: "app", Short: "Sample CLI application"}
		ai := &cobra.Command{Use: "ai", Short: "Language model helpers"}
		ai.AddCommand(leafCommand("completion", "Request a text completion"))
		nested.AddCommand(ai)
		nested.InitDefaultHelpCmd()
		nested.InitDefaultCompletionCmd()

		want := []string{"ai", "completion"}
		if got := listedCommands(FormatCommandTree(nested, hide)); !slices.Equal(got, want) {
			t.Errorf("tree lists %v, want %v:\n%s", got, want, FormatCommandTree(nested, hide))
		}
	})
}
