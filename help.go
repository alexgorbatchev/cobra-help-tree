package cobrahelptree

import (
	"fmt"
	"strings"

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
	useLine := cmd.UseLine()
	if len(visibleSubcommands(cmd, opt.HideGeneratedCommands)) > 0 && !strings.Contains(useLine, "[command]") {
		useLine += " [command]"
	}
	sb.WriteString(useLine)
	sb.WriteString("\n")

	// The arguments block and the command tree are one screen, so they are measured
	// together and share a description column. Cobra carries no per-argument
	// description of its own, so the catalog is the only source for the former.
	args := argRows(cat[cmd.CommandPath()].Args)
	tree := commandTreeRows(cmd, opt)
	descStartCol := descStartColumn(opt, args, tree)

	if len(args) > 0 {
		sb.WriteString("\nArguments:\n")
		sb.WriteString(renderLabelRows(args, descStartCol, opt, opt.TerminalWidth))
	}

	treeStr := renderLabelRows(tree, descStartCol, opt, opt.TerminalWidth)
	if treeStr != "" {
		sb.WriteString("\nAvailable Commands:\n")
		sb.WriteString(treeStr)
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
