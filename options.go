package cobrahelptree

import "fmt"

// Fallbacks applied to any TreeOptions field left at its zero value, which is
// what makes the zero value of TreeOptions a fully configured value.
const (
	defaultMinPadding    = 2
	defaultMinLabelWidth = 20
)

// TreeOptions configures how the command tree is formatted. Every field applies
// to human-mode rendering only, so it is what the tree renderers accept.
type TreeOptions struct {
	IncludeRoot   bool // If true, renders the root command node at top
	MinPadding    int  // Minimum padding between a label and its description (0 = 2)
	MinLabelWidth int  // Minimum cells reserved for the command and argument columns before padding (0 = 20)
	TerminalWidth int  // Max line width before clipping descriptions with '...' and wrapping flags (0 = auto-detect)

	// HideGeneratedCommands drops the commands the CLI did not define itself from
	// the human help screens: the completion command cobra generates, with its
	// per-shell subtree, and the skill command HelpOptions.Skill adds. Both still
	// run when invoked.
	//
	// The zero value keeps them, which for completion is what cobra's own help
	// does. Set this when the tree should show only the commands the CLI itself
	// defines: the completion command arrives with four shell children, so it
	// costs five lines above the CLI's first real command in a format whose purpose
	// is a readable hierarchy, and the skill command is one a person has no use for.
	//
	// It is human-mode formatting, so it does not apply to agent mode, where the
	// contract is a full description of the interface the binary accepts.
	HideGeneratedCommands bool
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
		{"MinLabelWidth", o.MinLabelWidth},
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
	if o.MinLabelWidth <= 0 {
		o.MinLabelWidth = defaultMinLabelWidth
	}
	return o
}

// AgentOptions configures AGENT=1 rendering. It is what the agent renderer
// accepts, mirroring TreeOptions for human mode.
type AgentOptions struct {
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
	Catalog      TechCatalog  // Optional per-command metadata, read by both modes
	Tree         TreeOptions  // Human-mode tree formatting
	Agent        AgentOptions // AGENT=1 rendering
	DisableAgent bool         // If true, always renders the human tree, ignoring AGENT

	// Skill is the CLI's usage guide for AI agents: the contents of its SKILL.md,
	// embedded in the binary with go:embed. When it is set, Setup adds a "skill"
	// command to the root that prints it byte for byte, and every AGENT=1 screen
	// opens with an alert telling the agent to read it first.
	//
	// The empty default adds neither. It is content rather than formatting, like
	// Catalog, so it sits here rather than in either mode's options.
	Skill string
}

// Validate reports the first invalid field, if any.
func (o HelpOptions) Validate() error {
	if err := o.Tree.Validate(); err != nil {
		return err
	}
	return o.Agent.Validate()
}
