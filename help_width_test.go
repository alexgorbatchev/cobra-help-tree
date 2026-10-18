package cobrahelptree

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

func TestHumanHelpAllLinesRespectWidth(t *testing.T) {
	for _, width := range []int{20, 60, 80} {
		root := &cobra.Command{Use: "application-with-a-long-name [a-very-long-argument]", Long: strings.Repeat("Long description 界 ", 20), Example: strings.Repeat("example ", 20)}
		root.Flags().String("long-option", strings.Repeat("value", 20), "A long flag default")
		root.AddCommand(&cobra.Command{Use: "child", Run: func(*cobra.Command, []string) {}})
		root.InitDefaultCompletionCmd()
		var visit func(*cobra.Command)
		visit = func(c *cobra.Command) {
			for _, line := range strings.Split(RenderTreeHelp(c, nil, TreeOptions{TerminalWidth: width}), "\n") {
				if got := runewidth.StringWidth(line); got > width {
					t.Errorf("%s width %d: got %d: %q", c.CommandPath(), width, got, line)
				}
			}
			for _, sub := range c.Commands() {
				visit(sub)
			}
		}
		visit(root)
	}
}
