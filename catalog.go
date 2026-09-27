package cobrahelptree

import "strings"

// ArgSpec describes one positional argument of a command. Cobra has no native
// field for this: Use carries the argument names as free text and ValidArgs is
// the enum of accepted values for the first positional argument, so a catalog
// entry is the only place a per-argument description can come from.
//
// Name is rendered verbatim in both modes. Supply it with whatever convention
// the CLI documents, such as "<name>", "[name]" or "<name...>"; the library
// never adds brackets of its own, because it cannot know whether an argument is
// required, optional or variadic.
type ArgSpec struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// EnvSpec describes one environment variable accepted by a command. Cobra has no
// native field for this, so a catalog entry is the only place an environment
// variable description can come from.
type EnvSpec struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// QuickstartItem describes one quickstart command line and an optional inline comment.
//
// Comment is rendered with an automatic "# " prefix in human mode if omitted by
// the caller, and aligned in a column across the quickstart block.
type QuickstartItem struct {
	Command string `json:"command"`
	Comment string `json:"comment,omitempty"`
}

// TechInfo provides optional per-command metadata. Args, Env, and Quickstart are
// read by both renderers; every other field is machine-level detail for AGENT=1 mode.
type TechInfo struct {
	Summary     string            `json:"summary,omitempty"`
	Description string            `json:"description,omitempty"`
	Args        []ArgSpec         `json:"args,omitempty"`
	Env         []EnvSpec         `json:"env,omitempty"`
	Quickstart  []QuickstartItem  `json:"quickstart,omitempty"`
	MutatesDB   bool              `json:"mutates_db,omitempty"`
	AutoBackup  bool              `json:"auto_backup,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TechCatalog maps full command paths (e.g. "mytool user create") to custom TechInfo.
//
// It is content rather than formatting, and both modes render its arguments, so
// it is passed to the renderers as its own parameter instead of living inside
// either mode's options struct.
type TechCatalog map[string]TechInfo

// cleanQuickstartComment strips any leading comment markers and surrounding
// whitespace from an inline quickstart comment.
func cleanQuickstartComment(comment string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(comment), "#"))
}

// argLabelIndent anchors each argument under its "Arguments:" heading. It is part
// of the measured label, so the argument descriptions land in the same column as
// the command tree's rather than two cells to its right.
const argLabelIndent = "  "

// argRows turns catalog argument specs into rows of the shared two-column block.
func argRows(args []ArgSpec) []labelRow {
	rows := make([]labelRow, 0, len(args))
	for _, a := range args {
		rows = append(rows, labelRow{label: argLabelIndent + a.Name, desc: a.Description})
	}
	return rows
}

// envLabelIndent anchors each environment variable under its "Environment Variables:" heading.
const envLabelIndent = "  "

// envRows turns catalog environment variable specs into rows of the shared two-column block.
func envRows(env []EnvSpec) []labelRow {
	rows := make([]labelRow, 0, len(env))
	for _, e := range env {
		rows = append(rows, labelRow{label: envLabelIndent + e.Name, desc: e.Description})
	}
	return rows
}
