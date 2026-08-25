package cobrahelptree

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

// TechInfo provides optional machine-level metadata for AGENT=1 mode.
type TechInfo struct {
	Summary     string            `json:"summary,omitempty"`
	Description string            `json:"description,omitempty"`
	Args        string            `json:"args,omitempty"`
	MutatesDB   bool              `json:"mutates_db,omitempty"`
	AutoBackup  bool              `json:"auto_backup,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TechCatalog maps full command paths (e.g. "mytool user create") to custom TechInfo.
type TechCatalog map[string]TechInfo

// TreeOptions configures how the command tree and help screens are formatted.
type TreeOptions struct {
	IncludeRoot   bool        // If true, renders the root command node at top
	MinPadding    int         // Minimum padding between command name and description (default 2)
	TerminalWidth int         // Max line width before clipping descriptions with '...' (0 = auto-detect)
	DisableAgent  bool        // If true, disables automatic AGENT=1 mode switching
	TechCatalog   TechCatalog // Optional machine metadata catalog for AGENT=1 mode
}

// DefaultOptions provides sensible defaults for CLI help screens.
var DefaultOptions = TreeOptions{
	IncludeRoot:   false,
	MinPadding:    2,
	TerminalWidth: 0,
	DisableAgent:  false,
	TechCatalog:   nil,
}

type treeEntry struct {
	prefix string
	cmd    *cobra.Command
}

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

// FormatCommandTree renders a Cobra command hierarchy into an aligned ASCII tree string.
// Direct subcommands start with '├─ ' and '╰─ ' at zero indent.
// If IncludeRoot is true, the root command is prepended at the top.
func FormatCommandTree(root *cobra.Command, opts ...TreeOptions) string {
	if root == nil {
		return ""
	}

	opt := DefaultOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.MinPadding <= 0 {
		opt.MinPadding = 2
	}

	termWidth := opt.TerminalWidth
	if termWidth <= 0 {
		termWidth = GetTerminalWidth()
	}

	var entries []treeEntry
	if opt.IncludeRoot {
		entries = append(entries, treeEntry{
			prefix: "",
			cmd:    root,
		})
	}
	collectSubcommandEntries(root, "", &entries)

	if len(entries) == 0 {
		return ""
	}

	// Calculate max width for alignment across all visible lines using rune counts (visual width)
	maxLeftWidth := 0
	for _, e := range entries {
		leftStr := e.prefix + e.cmd.Use
		runeCount := utf8.RuneCountInString(leftStr)
		if runeCount > maxLeftWidth {
			maxLeftWidth = runeCount
		}
	}
	if maxLeftWidth < 20 {
		maxLeftWidth = 20
	}

	var sb strings.Builder
	descStartCol := maxLeftWidth + opt.MinPadding

	for _, e := range entries {
		leftStr := e.prefix + e.cmd.Use
		sb.WriteString(leftStr)

		if e.cmd.Short != "" {
			runeCount := utf8.RuneCountInString(leftStr)
			padLen := (maxLeftWidth + opt.MinPadding) - runeCount
			if padLen < opt.MinPadding {
				padLen = opt.MinPadding
			}
			sb.WriteString(strings.Repeat(" ", padLen))

			desc := e.cmd.Short
			if termWidth > 0 && descStartCol < termWidth {
				maxDescLen := termWidth - descStartCol
				if maxDescLen > 3 && utf8.RuneCountInString(desc) > maxDescLen {
					runes := []rune(desc)
					desc = string(runes[:maxDescLen-3]) + "..."
				}
			}
			sb.WriteString(desc)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func collectSubcommandEntries(parent *cobra.Command, prefix string, entries *[]treeEntry) {
	// Filter visible child commands
	var visible []*cobra.Command
	for _, c := range parent.Commands() {
		if !c.Hidden && c.Name() != "help" && c.Name() != "completion" {
			visible = append(visible, c)
		}
	}

	for i, child := range visible {
		isLast := i == len(visible)-1
		branch := "├─ "
		nextPrefix := prefix + "│  "
		if isLast {
			branch = "╰─ "
			nextPrefix = prefix + "   "
		}

		*entries = append(*entries, treeEntry{
			prefix: prefix + branch,
			cmd:    child,
		})

		if len(child.Commands()) > 0 {
			collectSubcommandEntries(child, nextPrefix, entries)
		}
	}
}

// RenderAgentHelp produces compact, token-conservative output when AGENT=1.
func RenderAgentHelp(cmd *cobra.Command, catalog ...TechCatalog) string {
	if cmd == nil {
		return ""
	}

	var cat TechCatalog
	if len(catalog) > 0 {
		cat = catalog[0]
	}

	path := cmd.CommandPath()
	info, exists := cat[path]
	if !exists {
		info = TechInfo{
			Summary:     cmd.Short,
			Description: cmd.Long,
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("command: %s\n", path))
	if info.Summary != "" {
		sb.WriteString(fmt.Sprintf("summary: %s\n", strings.TrimSpace(info.Summary)))
	} else if cmd.Short != "" {
		sb.WriteString(fmt.Sprintf("summary: %s\n", strings.TrimSpace(cmd.Short)))
	}

	if info.Description != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", strings.TrimSpace(info.Description)))
	} else if cmd.Long != "" && strings.TrimSpace(cmd.Long) != strings.TrimSpace(cmd.Short) {
		sb.WriteString(fmt.Sprintf("description: %s\n", strings.TrimSpace(cmd.Long)))
	}

	sb.WriteString(fmt.Sprintf("usage: %s\n", cmd.UseLine()))

	if info.Args != "" {
		sb.WriteString(fmt.Sprintf("args: %s\n", info.Args))
	} else if len(cmd.ValidArgs) > 0 {
		sb.WriteString(fmt.Sprintf("args: %s\n", strings.Join(cmd.ValidArgs, ", ")))
	}

	if info.MutatesDB {
		sb.WriteString("mutates_db: true\n")
	}
	if info.AutoBackup {
		sb.WriteString("auto_backup: true (override with --no-backup)\n")
	}
	for k, v := range info.Metadata {
		sb.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}

	var visibleSubs []*cobra.Command
	for _, sub := range cmd.Commands() {
		if !sub.Hidden && sub.Name() != "help" && sub.Name() != "completion" {
			visibleSubs = append(visibleSubs, sub)
		}
	}
	if len(visibleSubs) > 0 {
		sb.WriteString("subcommands:\n")
		for _, sub := range visibleSubs {
			subPath := sub.CommandPath()
			subSummary := sub.Short
			if subInfo, ok := cat[subPath]; ok && subInfo.Summary != "" {
				subSummary = subInfo.Summary
			}
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", sub.Name(), strings.TrimSpace(subSummary)))
		}
	}

	flags := cmd.Flags()
	var flagLines []string
	flags.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		short := ""
		if f.Shorthand != "" {
			short = "-" + f.Shorthand + ", "
		}
		flagLines = append(flagLines, fmt.Sprintf("  %s--%s %s: %s (default: %q)", short, f.Name, f.Value.Type(), f.Usage, f.DefValue))
	})

	if len(flagLines) > 0 {
		sb.WriteString("flags:\n")
		for _, fl := range flagLines {
			sb.WriteString(fl + "\n")
		}
	}

	return sb.String()
}

// RenderTreeHelp builds a standard Cobra help screen with the command tree under "Available Commands:".
func RenderTreeHelp(cmd *cobra.Command, opts ...TreeOptions) string {
	if cmd == nil {
		return ""
	}

	var sb strings.Builder

	if cmd.Long != "" {
		sb.WriteString(strings.TrimSpace(cmd.Long))
		sb.WriteString("\n\n")
	} else if cmd.Short != "" {
		sb.WriteString(strings.TrimSpace(cmd.Short))
		sb.WriteString("\n\n")
	}

	sb.WriteString("Usage:\n  ")
	useLine := cmd.UseLine()
	hasSubs := false
	for _, c := range cmd.Commands() {
		if !c.Hidden && c.Name() != "help" && c.Name() != "completion" {
			hasSubs = true
			break
		}
	}
	if hasSubs && !strings.Contains(useLine, "[command]") {
		useLine += " [command]"
	}
	sb.WriteString(useLine)
	sb.WriteString("\n")

	// Available Commands Tree
	treeStr := FormatCommandTree(cmd, opts...)
	if treeStr != "" {
		sb.WriteString("\nAvailable Commands:\n")
		sb.WriteString(treeStr)
	}

	// Flags (Local flags specific to this command)
	localFlagsStr := strings.TrimRight(cmd.LocalFlags().FlagUsages(), "\n")
	if localFlagsStr != "" {
		sb.WriteString("\nFlags:\n")
		sb.WriteString(cmd.LocalFlags().FlagUsages())
	}

	// Global Flags (Inherited flags from parent commands)
	if cmd.HasInheritedFlags() {
		inheritedStr := strings.TrimRight(cmd.InheritedFlags().FlagUsages(), "\n")
		if inheritedStr != "" {
			sb.WriteString("\nGlobal Flags:\n")
			sb.WriteString(cmd.InheritedFlags().FlagUsages())
		}
	}

	if cmd.Example != "" {
		sb.WriteString("\nExamples:\n")
		sb.WriteString(strings.TrimRight(cmd.Example, "\n"))
		sb.WriteString("\n")
	}

	if treeStr != "" {
		sb.WriteString(fmt.Sprintf("\nUse %q for more information about a command.\n", cmd.CommandPath()+" [command] --help"))
	}

	return sb.String()
}

// Setup configures a Cobra command to render ASCII command trees in human mode
// and compact token-conservative YAML-like output in AGENT=1 mode.
func Setup(cmd *cobra.Command, opts ...TreeOptions) {
	if cmd == nil {
		return
	}
	opt := DefaultOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if !opt.DisableAgent && IsAgentMode() {
			c.Print(RenderAgentHelp(c, opt.TechCatalog))
			return
		}
		c.Print(RenderTreeHelp(c, opt))
	})
}
