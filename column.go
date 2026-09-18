package cobrahelptree

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// ellipsis marks a description clipped to fit the terminal width.
const ellipsis = "..."

// labelRow is one line of a two-column block: a label such as a tree branch or an
// argument name, and the description that follows it.
type labelRow struct {
	label string
	desc  string
}

// descStartColumn returns the column at which every description on one screen
// begins: the widest label across all of the screen's blocks, floored at
// MinLabelWidth, plus the padding. Passing every block's rows in one call is what
// lines the arguments block up with the command tree below it.
//
// It measures terminal cell width rather than rune count: a CJK ideograph or
// emoji is one rune in two columns, so counting runes shifts the column.
func descStartColumn(opt TreeOptions, rowSets ...[]labelRow) int {
	widest := opt.MinLabelWidth
	for _, rows := range rowSets {
		for _, r := range rows {
			if w := runewidth.StringWidth(r.label); w > widest {
				widest = w
			}
		}
	}
	return widest + opt.MinPadding
}

// renderLabelRows renders rows as a two-column block whose descriptions start at
// descStartCol. It is the single alignment rule shared by the command tree and
// the arguments block.
//
// opt must already be resolved. A termWidth of zero or less disables clipping,
// which is what a non-terminal stdout reports.
func renderLabelRows(rows []labelRow, descStartCol int, opt TreeOptions, termWidth int) string {
	if len(rows) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, r := range rows {
		sb.WriteString(r.label)

		if r.desc != "" {
			padLen := descStartCol - runewidth.StringWidth(r.label)
			if padLen < opt.MinPadding {
				padLen = opt.MinPadding
			}
			sb.WriteString(strings.Repeat(" ", padLen))
			sb.WriteString(clipDescription(r.desc, descStartCol, termWidth))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// clipDescription truncates a description that would run past termWidth when it
// starts at startCol, so a long description never wraps and breaks the column.
func clipDescription(desc string, startCol, termWidth int) string {
	if termWidth <= 0 || startCol >= termWidth {
		return desc
	}
	maxDescCells := termWidth - startCol
	// Below four cells there is no room for content plus the ellipsis.
	if maxDescCells <= runewidth.StringWidth(ellipsis) {
		return desc
	}
	return runewidth.Truncate(desc, maxDescCells, ellipsis)
}
