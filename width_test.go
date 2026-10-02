package cobrahelptree

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

func TestHumanScreensRespectEveryTerminalWidth(t *testing.T) {
	for _, width := range []int{1, 2, 20, 40, 60, 80, 100} {
		root := buildSampleCommandHierarchy()
		root.Long = "\t" + strings.Repeat("Long description 界 ", 20)
		root.Example = strings.Repeat("example ", 30)
		root.Deprecated = strings.Repeat("deprecation ", 20)
		root.AddCommand(leafCommand(strings.Repeat("long-command", 10), "Long label"))
		opt := TreeOptions{TerminalWidth: width}
		for _, screen := range []string{RenderTreeHelp(root, nil, opt), RenderTreeUsage(root, nil, opt), FormatCommandTree(root, opt)} {
			for _, line := range strings.Split(screen, "\n") {
				if got := runewidth.StringWidth(line); got > width {
					t.Errorf("width %d: line uses %d cells: %q", width, got, line)
				}
			}
		}
	}
}

func TestClipHumanLinesExpandsTabsAndLeavesUnlimitedText(t *testing.T) {
	text := "a\tb\n界\tc\n"
	if got := clipHumanLines(text, 20); got != "a       b\n界      c\n" {
		t.Fatalf("incorrect tab stops: %q", got)
	}
	if got := clipHumanLines(text, 0); got != text {
		t.Fatalf("unlimited internal render changed text: %q", got)
	}
}

func TestHumanHelpUsesFallbackWidth(t *testing.T) {
	withNonTerminalStdout(t)
	t.Setenv("COLUMNS", "")
	root := &cobra.Command{Use: "app", Long: strings.Repeat("description ", 30)}
	for _, line := range strings.Split(RenderTreeHelp(root, nil, TreeOptions{}), "\n") {
		if got := runewidth.StringWidth(line); got > 100 {
			t.Errorf("fallback line uses %d cells: %q", got, line)
		}
	}
}
