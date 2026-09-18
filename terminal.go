package cobrahelptree

import (
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// IsAgentMode checks if the AGENT environment variable is set to a truthy value ("1", "true", "yes").
func IsAgentMode() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT")))
	return v == "1" || v == "true" || v == "yes"
}

// GetTerminalWidth returns the detected terminal width in columns, or 0 if unknown/non-terminal.
func GetTerminalWidth() int {
	if colStr := os.Getenv("COLUMNS"); colStr != "" {
		if w, err := strconv.Atoi(colStr); err == nil && w > 0 {
			return w
		}
	}
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 0
}

// effectiveTerminalWidth returns the column limit human output clips to: the
// width the caller pinned, or the detected terminal width when the caller left
// TerminalWidth unset.
func effectiveTerminalWidth(opt TreeOptions) int {
	if opt.TerminalWidth > 0 {
		return opt.TerminalWidth
	}
	return GetTerminalWidth()
}
