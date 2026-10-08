package cobrahelptree

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

// RenderTreeHelp builds the screen cobra prints for --help: the command's long
// description followed by its usage screen. It mutates cmd: see the package
// Concurrency note.
//
// The split mirrors cobra's own. Its defaultHelpFunc prints the description and
// then delegates to UsageString, so replacing only one of the two leaves the
// other rendering cobra's flat command list.
func RenderTreeHelp(cmd *cobra.Command, cat TechCatalog, opt TreeOptions) string {
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

	sb.WriteString(RenderTreeUsage(cmd, cat, opt))

	return sb.String()
}

// RenderTreeUsage builds the screen cobra prints after a flag or argument error:
// everything RenderTreeHelp prints except the long description, with the command
// tree under "Available Commands:". It mutates cmd: see the package Concurrency
// note.
func RenderTreeUsage(cmd *cobra.Command, cat TechCatalog, opt TreeOptions) string {
	if cmd == nil {
		return ""
	}

	// Detected once and pinned, so the arguments block and the command tree clip
	// against the same width even if the terminal is resized mid-render.
	opt = opt.resolve()
	opt.TerminalWidth = effectiveTerminalWidth(opt)

	// Read the flag sets before UseLine. Reading them merges cobra's persistent
	// flag sets, which flips HasAvailableFlags and so decides whether UseLine
	// appends "[flags]"; collecting them afterwards makes a command's first render
	// differ from every later one.
	localFlagUsages := cmd.LocalFlags().FlagUsagesWrapped(opt.TerminalWidth)
	inheritedFlagUsages := cmd.InheritedFlags().FlagUsagesWrapped(opt.TerminalWidth)

	var sb strings.Builder

	// Cobra keeps a deprecated command out of its parent's listing and prints the
	// Deprecated string only once the command runs, so its own help screen is the
	// only place a user who still has it scripted can be warned before running it.
	// The wording is the caller's, exactly as cobra prints it at run time.
	if cmd.Deprecated != "" {
		sb.WriteString(fmt.Sprintf("Deprecated: %s\n\n", strings.TrimSpace(cmd.Deprecated)))
	}

	sb.WriteString("Usage:\n  ")
	sb.WriteString(usageLine(cmd, opt.HideGeneratedCommands))
	sb.WriteString("\n")

	// The arguments block, the command tree, and the environment variables block
	// are one screen, so they are measured together and share a description column.
	// Cobra carries no per-argument or per-env description of its own, so the
	// catalog is the only source for both.
	info := cat[cmd.CommandPath()]
	args := argRows(info.Args)
	tree := commandTreeRows(cmd, opt)
	envs := envRows(info.Env)
	descStartCol := descStartColumn(opt, args, tree, envs)

	if len(args) > 0 {
		sb.WriteString("\nArguments:\n")
		sb.WriteString(renderLabelRows(args, descStartCol, opt, opt.TerminalWidth))
	}

	treeStr := renderLabelRows(tree, descStartCol, opt, opt.TerminalWidth)
	if treeStr != "" {
		sb.WriteString("\nAvailable Commands:\n")
		sb.WriteString(treeStr)
	}

	if len(envs) > 0 {
		sb.WriteString("\nEnvironment Variables:\n")
		sb.WriteString(renderLabelRows(envs, descStartCol, opt, opt.TerminalWidth))
	}

	// Flags local to this command, then those inherited from its parents.
	if strings.TrimRight(localFlagUsages, "\n") != "" {
		sb.WriteString("\nFlags:\n")
		sb.WriteString(localFlagUsages)
	}
	if strings.TrimRight(inheritedFlagUsages, "\n") != "" {
		sb.WriteString("\nGlobal Flags:\n")
		sb.WriteString(inheritedFlagUsages)
	}

	quickstartStr := renderQuickstart(info.Quickstart, opt, opt.TerminalWidth)
	if quickstartStr != "" {
		sb.WriteString("\nQuickstart:\n")
		sb.WriteString(quickstartStr)
	}

	if cmd.Example != "" {
		sb.WriteString("\nExamples:\n")
		sb.WriteString(strings.TrimRight(cmd.Example, "\n"))
		sb.WriteString("\n")
	}

	if treeStr != "" {
		// The one sentence on the screen. It is wrapped rather than clipped like the
		// descriptions above it, because the end of it is the instruction.
		hint := fmt.Sprintf("Use %q for more information about a command.", cmd.CommandPath()+" [command] --help")
		sb.WriteString("\n")
		sb.WriteString(wrapWords(hint, opt.TerminalWidth))
		sb.WriteString("\n")
	}

	return sb.String()
}

// usageLine returns cobra's usage line for cmd, with " [command]" appended when
// cmd has subcommands to list and the line does not say so already. Cobra's own
// template prints that as a second usage line; both modes here state it on the
// one line, so the human screen and the agent screen describe the same syntax.
//
// Call it after the command's flags have been read: reading them decides whether
// cobra appends "[flags]". See applicableFlags.
func usageLine(cmd *cobra.Command, hideGenerated bool) string {
	line := cmd.UseLine()
	if len(visibleSubcommands(cmd, hideGenerated)) > 0 && !strings.Contains(line, "[command]") {
		line += " [command]"
	}
	return line
}

const quickstartIndent = "  "

// renderQuickstart renders quickstart items with commands indented and inline
// comments aligned to a shared column and prefixed with "# ".
func renderQuickstart(items []QuickstartItem, opt TreeOptions, termWidth int) string {
	if len(items) == 0 {
		return ""
	}

	widest := 0
	for _, item := range items {
		if w := runewidth.StringWidth(quickstartIndent + item.Command); w > widest {
			widest = w
		}
	}

	commentCol := widest + opt.MinPadding

	var sb strings.Builder
	for _, item := range items {
		cmdStr := quickstartIndent + item.Command
		clean := cleanQuickstartComment(item.Comment)
		if clean == "" {
			sb.WriteString(cmdStr)
			sb.WriteString("\n")
			continue
		}

		comment := "# " + clean
		padLen := commentCol - runewidth.StringWidth(cmdStr)
		sb.WriteString(cmdStr)
		sb.WriteString(strings.Repeat(" ", padLen))
		sb.WriteString(clipDescription(comment, commentCol, termWidth))
		sb.WriteString("\n")
	}

	return sb.String()
}
