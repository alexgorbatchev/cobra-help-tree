package cobrahelptree

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// clipHumanLines applies the screen's width budget to every emitted line,
// including prose, usage, examples, long labels and the navigation footer.
func clipHumanLines(screen string, width int) string {
	if width <= 0 {
		return screen
	}
	marker := ellipsis
	if width < len(marker) {
		marker = strings.Repeat(".", width)
	}
	lines := strings.Split(screen, "\n")
	for i, line := range lines {
		lines[i] = runewidth.Truncate(expandTabs(line), width, marker)
	}
	return strings.Join(lines, "\n")
}

const tabStop = 8

func expandTabs(line string) string {
	var sb strings.Builder
	for _, r := range line {
		if r == '\t' {
			padding := tabStop - runewidth.StringWidth(sb.String())%tabStop
			sb.WriteString(strings.Repeat(" ", padding))
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
