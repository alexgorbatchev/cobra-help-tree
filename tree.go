package cobrahelptree

import "github.com/spf13/cobra"

type treeEntry struct {
	prefix string
	cmd    *cobra.Command
}

// FormatCommandTree renders a Cobra command hierarchy into an aligned ASCII tree string.
// Direct subcommands start with '├─ ' and '╰─ ' at zero indent.
// If IncludeRoot is true, the root command is prepended at the top.
func FormatCommandTree(root *cobra.Command, opt TreeOptions) string {
	if root == nil {
		return ""
	}
	opt = opt.resolve()

	rows := commandTreeRows(root, opt)

	return renderLabelRows(rows, descStartColumn(opt, rows), opt, effectiveTerminalWidth(opt))
}

// commandTreeRows returns one row per rendered command, each labelled with its
// branch glyphs and described by its Short.
func commandTreeRows(root *cobra.Command, opt TreeOptions) []labelRow {
	var entries []treeEntry
	if opt.IncludeRoot {
		entries = append(entries, treeEntry{
			prefix: "",
			cmd:    root,
		})
	}
	collectSubcommandEntries(root, "", opt.HideGeneratedCommands, &entries)

	rows := make([]labelRow, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, labelRow{label: e.prefix + e.cmd.Use, desc: e.cmd.Short})
	}
	return rows
}

// completionCommandName is the name cobra gives the completion command it
// generates (compCmdName in its completions.go).
const completionCommandName = "completion"

// visibleSubcommands returns the children of cmd that belong in a rendered help
// screen. Every renderer goes through it, so the command tree, the agent-mode
// subcommand list and the "[command]" suffix on the usage line can never
// disagree about what the CLI offers.
//
// Availability is cobra's own judgement, not a reimplementation of it: a
// hand-written equivalent drifts from upstream as cobra evolves, and the drift is
// invisible until someone compares the two screens. Command.IsAvailableCommand
// excludes hidden commands, deprecated commands, the generated help command, and
// any command that is neither runnable nor the parent of a runnable one.
//
// Note what this means for the generated help command: cobra's predicate already
// excludes it by identity, and only cobra's help template re-adds it by name, so
// it is absent here without any filtering of ours.
//
// hideGenerated additionally drops the generated completion command, which cobra
// does list. It is off by default, so the default screen matches cobra's.
func visibleSubcommands(cmd *cobra.Command, hideGenerated bool) []*cobra.Command {
	var visible []*cobra.Command
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() {
			continue
		}
		if hideGenerated && isGeneratedCompletionCommand(c) {
			continue
		}
		visible = append(visible, c)
	}
	return visible
}

// isGeneratedCompletionCommand reports whether c is the completion command cobra
// generated. Cobra exposes no handle on it, so the test is its name and its
// position: InitDefaultCompletionCmd adds it to the root and returns early when a
// root child already carries that name or alias, so a command named "completion"
// anywhere below the root belongs to the CLI and is never cobra's.
func isGeneratedCompletionCommand(c *cobra.Command) bool {
	return c.Name() == completionCommandName && c.HasParent() && !c.Parent().HasParent()
}

func collectSubcommandEntries(parent *cobra.Command, prefix string, hideGenerated bool, entries *[]treeEntry) {
	visible := visibleSubcommands(parent, hideGenerated)

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
			collectSubcommandEntries(child, nextPrefix, hideGenerated, entries)
		}
	}
}
