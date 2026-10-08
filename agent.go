package cobrahelptree

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// RenderAgentHelp produces compact, token-conservative output when AGENT=1.
// It mutates cmd: see the package Concurrency note.
// Output is untruncated unless opt.MaxLineWidth asks for clipping.
//
// Agent mode has one screen rather than cobra's help and usage pair, because the
// two differ only in visual framing and a machine reader wants a single format.
func RenderAgentHelp(cmd *cobra.Command, cat TechCatalog, opt AgentOptions) string {
	if cmd == nil {
		return ""
	}

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

	// Agent mode never hides a command, so the line advertises subcommands
	// whenever cobra finds one available.
	sb.WriteString(fmt.Sprintf("usage: %s\n", usageLine(cmd, false)))

	// A deprecated command is absent from every listing, so an agent reaching this
	// screen is one that already knows the command and needs to be told not to keep
	// using it. Cobra prints the same string, but only once the command has run.
	if cmd.Deprecated != "" {
		sb.WriteString(fmt.Sprintf("deprecated: %s\n", strings.TrimSpace(cmd.Deprecated)))
	}

	if len(info.Args) > 0 {
		sb.WriteString("args:\n")
		for _, a := range info.Args {
			sb.WriteString(agentListItem(a.Name, a.Description))
		}
	}

	// ValidArgs is cobra's enum of accepted values for the first positional
	// argument (args.go, OnlyValidArgs), not the command's argument list, so it
	// gets a key of its own instead of being reported as "args". Entries built
	// with cobra.CompletionWithDesc carry their description after a tab, which is
	// split off here: emitting it raw would put a control character inside a value
	// and make the line unparseable.
	if len(cmd.ValidArgs) > 0 {
		sb.WriteString("valid_args:\n")
		for _, v := range cmd.ValidArgs {
			name, desc, _ := strings.Cut(v, "\t")
			sb.WriteString(agentListItem(name, desc))
		}
	}

	if len(info.Env) > 0 {
		sb.WriteString("env:\n")
		for _, e := range info.Env {
			sb.WriteString(agentListItem(e.Name, e.Description))
		}
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

	if subcommands := agentSubcommands(cmd, cat, 0); subcommands != "" {
		sb.WriteString("subcommands:\n")
		sb.WriteString(subcommands)
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
		line := fmt.Sprintf("  %s--%s %s: %s", short, f.Name, f.Value.Type(), f.Usage)
		// A default equal to the type's zero value says nothing the type does not,
		// so it is left out, as pflag leaves it out of the human screen.
		if hasNonZeroDefault(f) {
			line += fmt.Sprintf(" (default: %q)", f.DefValue)
		}
		flagLines = append(flagLines, line)
	})

	if len(flagLines) > 0 {
		sb.WriteString("flags:\n")
		for _, fl := range flagLines {
			sb.WriteString(fl + "\n")
		}
	}

	if len(info.Quickstart) > 0 {
		sb.WriteString("quickstart:\n")
		for _, q := range info.Quickstart {
			sb.WriteString(agentListItem(q.Command, cleanQuickstartComment(q.Comment)))
		}
	}

	// The alert is joined on after clipping. It tells the agent which command to
	// run first, and a clipped line would cut that command short.
	return skillAlert(cmd) + clipLines(sb.String(), opt.MaxLineWidth)
}

// agentSubcommands renders every command below parent as a nested list: one
// entry per command, with a group's own commands indented beneath it. It lists
// the whole subtree for the reason the human tree does, so that one request for
// help describes everything the command offers; an agent shown only the next
// level down would need a further call per group.
//
// It never hides cobra's generated commands: agent output describes the interface
// the binary actually accepts, and completion is a command it accepts. Hiding it
// is human-mode formatting, so it lives on TreeOptions.
func agentSubcommands(parent *cobra.Command, cat TechCatalog, depth int) string {
	var sb strings.Builder
	for _, sub := range visibleSubcommands(parent, false) {
		summary := sub.Short
		if info, ok := cat[sub.CommandPath()]; ok && info.Summary != "" {
			summary = info.Summary
		}
		// Use rather than Name, so an entry carries the command's arguments as its
		// branch in the human tree does and the list is enough to call the command.
		sb.WriteString(agentListItemAt(depth, sub.Use, summary))
		sb.WriteString(agentSubcommands(sub, cat, depth+1))
	}
	return sb.String()
}

// hasNonZeroDefault reports whether f has a default worth stating, by pflag's own
// judgement. pflag prints "(default ...)" in a flag's usage line for exactly those
// flags and exposes the test no other way, so the answer is read off that line.
// The probe carries no usage text, hidden mark or deprecation of its own: the
// first could contain the words looked for, and the other two keep a flag out of
// the usage altogether.
func hasNonZeroDefault(f *pflag.Flag) bool {
	probe := *f
	probe.Usage = ""
	probe.Hidden = false
	probe.Deprecated = ""

	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.AddFlag(&probe)
	return strings.Contains(flags.FlagUsages(), "(default ")
}

// agentListIndent is one level of an agent-mode list: the indent of a top-level
// entry, and what each level of nesting adds to it.
const agentListIndent = "  "

// agentListItem renders one entry of a top-level agent-mode list.
func agentListItem(name, desc string) string {
	return agentListItemAt(0, name, desc)
}

// agentListItemAt renders one entry of an agent-mode list nested depth levels
// below the top one. An entry without a description is emitted as a bare name
// rather than as a key with nothing behind it, which is the same rule clipLine
// applies: agent output is parsed by machine, and an empty value reads as a field
// that lost its data.
func agentListItemAt(depth int, name, desc string) string {
	indent := strings.Repeat(agentListIndent, depth+1)
	name = strings.TrimSpace(name)
	if desc = strings.TrimSpace(desc); desc == "" {
		return fmt.Sprintf("%s- %s\n", indent, name)
	}
	return fmt.Sprintf("%s- %s: %s\n", indent, name, desc)
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
