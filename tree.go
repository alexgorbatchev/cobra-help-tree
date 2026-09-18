// Package cobrahelptree renders nested Cobra command hierarchies as aligned
// ASCII trees in human mode, and as compact key-value output when AGENT=1.
//
// # Concurrency
//
// The renderers are not safe to call concurrently on commands that share a root.
// Reading a command's flags makes Cobra merge its persistent flag sets, and that
// merge writes to the root and to every ancestor rather than to the command
// being rendered, so two goroutines rendering two sibling subcommands race
// inside Cobra. Commands installed through Setup are unaffected, because Cobra
// executes a command tree on a single goroutine.
package cobrahelptree

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

// ellipsis marks a description clipped to fit the terminal width.
const ellipsis = "..."

// Fallbacks applied to any TreeOptions field left at its zero value, which is
// what makes the zero value of TreeOptions a fully configured value.
const (
	defaultMinPadding      = 2
	defaultMinCommandWidth = 20
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

// TreeOptions configures how the command tree is formatted. Every field applies
// to human-mode rendering only, so it is what the tree renderers accept.
type TreeOptions struct {
	IncludeRoot     bool // If true, renders the root command node at top
	MinPadding      int  // Minimum padding between command name and description (0 = 2)
	MinCommandWidth int  // Minimum cells reserved for the command column before padding (0 = 20)
	TerminalWidth   int  // Max line width before clipping descriptions with '...' (0 = auto-detect)
}

// Validate reports the first invalid field, if any. Zero means "use the default"
// for every sizing field, so a negative value is a caller mistake rather than a
// request for the default and is reported instead of being silently coerced.
func (o TreeOptions) Validate() error {
	for _, f := range []struct {
		name  string
		value int
	}{
		{"MinPadding", o.MinPadding},
		{"MinCommandWidth", o.MinCommandWidth},
		{"TerminalWidth", o.TerminalWidth},
	} {
		if f.value < 0 {
			return fmt.Errorf("cobrahelptree: %s must be zero (use the default) or positive, got %d", f.name, f.value)
		}
	}
	return nil
}

// resolve returns a copy with every unset sizing field replaced by its fallback,
// so callers can pass a partially populated TreeOptions without losing defaults.
func (o TreeOptions) resolve() TreeOptions {
	if o.MinPadding <= 0 {
		o.MinPadding = defaultMinPadding
	}
	if o.MinCommandWidth <= 0 {
		o.MinCommandWidth = defaultMinCommandWidth
	}
	return o
}

// AgentOptions configures AGENT=1 rendering. It is what the agent renderer
// accepts, mirroring TreeOptions for human mode.
type AgentOptions struct {
	TechCatalog TechCatalog // Optional machine metadata catalog

	// MaxLineWidth clips each rendered line to this many terminal cells.
	// Zero means unlimited, which is the default: agent output is machine-read,
	// so full untruncated content is the contract and clipping is opt-in. Unlike
	// TreeOptions.TerminalWidth, zero never triggers terminal auto-detection.
	//
	// It is a ceiling rather than a guarantee. Only the value half of a
	// "key: value" line is clipped, and a line whose key leaves no room for a
	// readable value is emitted in full, so clipping never yields an
	// unidentifiable key or a key with nothing behind it.
	MaxLineWidth int
}

// Validate reports the first invalid field, if any.
func (o AgentOptions) Validate() error {
	if o.MaxLineWidth < 0 {
		return fmt.Errorf("cobrahelptree: MaxLineWidth must be zero (unlimited) or positive, got %d", o.MaxLineWidth)
	}
	return nil
}

// HelpOptions configures a help screen across both modes. The two renderers have
// disjoint settings, so each one receives only the fields it actually reads and
// this struct exists solely where the composition is real: Setup.
type HelpOptions struct {
	Tree         TreeOptions  // Human-mode tree formatting
	Agent        AgentOptions // AGENT=1 rendering
	DisableAgent bool         // If true, always renders the human tree, ignoring AGENT
}

// Validate reports the first invalid field, if any.
func (o HelpOptions) Validate() error {
	if err := o.Tree.Validate(); err != nil {
		return err
	}
	return o.Agent.Validate()
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
func FormatCommandTree(root *cobra.Command, opt TreeOptions) string {
	if root == nil {
		return ""
	}
	opt = opt.resolve()

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

	// Align on terminal cell width, not rune count: a CJK ideograph or emoji is one
	// rune but occupies two columns, so counting runes shifts the description column.
	maxLeftWidth := 0
	for _, e := range entries {
		leftStr := e.prefix + e.cmd.Use
		if w := runewidth.StringWidth(leftStr); w > maxLeftWidth {
			maxLeftWidth = w
		}
	}
	if maxLeftWidth < opt.MinCommandWidth {
		maxLeftWidth = opt.MinCommandWidth
	}

	var sb strings.Builder
	descStartCol := maxLeftWidth + opt.MinPadding

	for _, e := range entries {
		leftStr := e.prefix + e.cmd.Use
		sb.WriteString(leftStr)

		if e.cmd.Short != "" {
			padLen := (maxLeftWidth + opt.MinPadding) - runewidth.StringWidth(leftStr)
			if padLen < opt.MinPadding {
				padLen = opt.MinPadding
			}
			sb.WriteString(strings.Repeat(" ", padLen))

			desc := e.cmd.Short
			if termWidth > 0 && descStartCol < termWidth {
				maxDescCells := termWidth - descStartCol
				// Below four cells there is no room for content plus the ellipsis.
				if maxDescCells > runewidth.StringWidth(ellipsis) {
					desc = runewidth.Truncate(desc, maxDescCells, ellipsis)
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
// It mutates cmd: see the package Concurrency note.
// Output is untruncated unless opt.MaxLineWidth asks for clipping.
func RenderAgentHelp(cmd *cobra.Command, opt AgentOptions) string {
	if cmd == nil {
		return ""
	}
	cat := opt.TechCatalog

	// Resolved up front, before UseLine is read: collecting the flags merges
	// cobra's persistent flag sets, which makes HasAvailableFlags true and so
	// changes whether UseLine appends "[flags]". Doing it first keeps the very
	// first render identical to every later one.
	flags := applicableFlags(cmd)

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
	// Metadata is caller-supplied, so it gets its own nested mapping. Emitting it
	// flat would let a key such as "usage" duplicate or shadow a reserved key above
	// and leave the output ambiguous to the machine readers agent mode exists for.
	// Keys are sorted because Go randomizes map iteration and this output must be
	// reproducible across runs.
	if len(info.Metadata) > 0 {
		sb.WriteString("metadata:\n")
		for _, k := range slices.Sorted(maps.Keys(info.Metadata)) {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, info.Metadata[k]))
		}
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

	return clipLines(sb.String(), opt.MaxLineWidth)
}

// applicableFlags returns every flag that applies to cmd: its own flags and
// persistent flags, plus those inherited from parents.
//
// cmd.Flags() is not enough on its own. Cobra only folds persistent flags into
// it via mergePersistentFlags, which runs during Execute and inside LocalFlags
// and InheritedFlags, so reading Flags() directly drops persistent flags on any
// command that has not been executed yet. LocalFlags and InheritedFlags are
// disjoint and together complete, and both perform that merge, so combining
// them reports the same set the human renderer shows.
func applicableFlags(cmd *cobra.Command) *pflag.FlagSet {
	all := pflag.NewFlagSet(cmd.Name(), pflag.ContinueOnError)
	all.AddFlagSet(cmd.LocalFlags())
	all.AddFlagSet(cmd.InheritedFlags())
	return all
}

// clipLines truncates every line of s to maxCells terminal cells. A maxCells of
// zero or less returns s untouched, which keeps agent output untruncated by
// default; clipping is something a caller opts into.
func clipLines(s string, maxCells int) string {
	if maxCells <= 0 {
		return s
	}

	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = clipLine(line, maxCells)
	}
	return strings.Join(lines, "\n")
}

// clipLine truncates the value half of a "key: value" line, leaving the key
// intact. Agent output is parsed by machine, so a line is clipped only when at
// least one cell of value survives: truncating the key itself would produce
// unidentifiable and possibly colliding field names, and leaving a key followed
// by nothing but an ellipsis would read as a field carrying no recoverable data.
// A line too narrow for both is returned in full, which makes MaxLineWidth a
// ceiling that never destroys a field rather than an absolute guarantee.
func clipLine(line string, maxCells int) string {
	if runewidth.StringWidth(line) <= maxCells {
		return line
	}

	// Structural lines such as "metadata:" open a block and carry no value.
	sep := strings.Index(line, ": ")
	if sep < 0 {
		return line
	}

	key := line[:sep+2]
	valueCells := maxCells - runewidth.StringWidth(key)
	if valueCells <= runewidth.StringWidth(ellipsis) {
		return line
	}
	return key + runewidth.Truncate(line[sep+2:], valueCells, ellipsis)
}

// RenderTreeHelp builds a standard Cobra help screen with the command tree under "Available Commands:".
// It mutates cmd: see the package Concurrency note.
func RenderTreeHelp(cmd *cobra.Command, opt TreeOptions) string {
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
	treeStr := FormatCommandTree(cmd, opt)
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
// The only error it can return is a nil command, since a zero TreeOptions is
// always valid.
func Setup(cmd *cobra.Command) error {
	return SetupWithOptions(cmd, HelpOptions{})
}

// SetupWithOptions is Setup with explicit options, where every field left at its
// zero value takes the documented default. It installs nothing and returns an
// error when cmd is nil or opt is invalid.
func SetupWithOptions(cmd *cobra.Command, opt HelpOptions) error {
	if cmd == nil {
		return errors.New("cobrahelptree: command is nil")
	}
	if err := opt.Validate(); err != nil {
		return err
	}

	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		var screen string
		if !opt.DisableAgent && IsAgentMode() {
			screen = RenderAgentHelp(c, opt.Agent)
		} else {
			screen = RenderTreeHelp(c, opt.Tree)
		}

		// Help the user explicitly asked for is a result, so it belongs on stdout,
		// matching cobra's own default help func (spf13/cobra#1002). Cobra's Print
		// falls back to stderr, which would break `--help | less` and `--help > file`.
		// Usage printed after a flag or argument error stays on stderr, unaffected.
		if _, err := fmt.Fprint(c.OutOrStdout(), screen); err != nil {
			c.PrintErrln(err)
		}
	})

	return nil
}
