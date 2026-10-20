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
	for _, tt := range []struct {
		name string
		text string
		want string
	}{
		{"multiline wide characters", "a\tb\n界\tc\n", "a       b\n界      c\n"},
		{"multiple stops", "a\tb\tc", "a       b       c"},
		{"aligned stop", "12345678\tx", "12345678        x"},
		{"leading tabs", "\t\tx", "                x"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := clipHumanLines(tt.text, 20); got != tt.want {
				t.Fatalf("incorrect tab stops: got %q, want %q", got, tt.want)
			}
			if got := clipHumanLines(tt.text, 0); got != tt.text {
				t.Fatalf("unlimited internal render changed text: %q", got)
			}
		})
	}
}

func TestHumanScreensLeaveUnknownWidthUnlimited(t *testing.T) {
	withNonTerminalStdout(t)
	t.Setenv("COLUMNS", "")
	text := strings.Repeat("description ", 30)
	root := &cobra.Command{Use: "app", Long: text, Example: text}
	root.AddCommand(leafCommand("child", text))
	for _, screen := range []string{RenderTreeHelp(root, nil, TreeOptions{}), RenderTreeUsage(root, nil, TreeOptions{}), FormatCommandTree(root, TreeOptions{})} {
		if !strings.Contains(screen, text) {
			t.Fatalf("unknown width clipped content: %q", screen)
		}
	}
}
