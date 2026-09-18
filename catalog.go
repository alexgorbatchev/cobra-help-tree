package cobrahelptree

// argLabelIndent anchors each argument under its "Arguments:" heading. It is part
// of the measured label, so the argument descriptions land in the same column as
// the command tree's rather than two cells to its right.
const argLabelIndent = "  "

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

// TechInfo provides optional per-command metadata. Args is read by both
// renderers; every other field is machine-level detail for AGENT=1 mode.
type TechInfo struct {
	Summary     string            `json:"summary,omitempty"`
	Description string            `json:"description,omitempty"`
	Args        []ArgSpec         `json:"args,omitempty"`
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

// argRows turns catalog argument specs into rows of the shared two-column block.
func argRows(args []ArgSpec) []labelRow {
	rows := make([]labelRow, 0, len(args))
	for _, a := range args {
		rows = append(rows, labelRow{label: argLabelIndent + a.Name, desc: a.Description})
	}
	return rows
}
