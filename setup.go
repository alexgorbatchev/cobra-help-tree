package cobrahelptree

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// Setup replaces every screen cobra prints for a command and its descendants:
// ASCII command trees in human mode, compact token-conservative key-value output
// in AGENT=1 mode. The only error it can return is a nil command, since a zero
// HelpOptions is always valid.
func Setup(cmd *cobra.Command) error {
	return SetupWithOptions(cmd, HelpOptions{})
}

// SetupWithOptions is Setup with explicit options, where every field left at its
// zero value takes the documented default. It installs nothing and returns an
// error when cmd is nil or opt is invalid.
//
// It replaces both of cobra's printing paths, which is what makes one call
// enough. Cobra renders --help through HelpFunc but renders the screen that
// follows a flag or argument error through UsageFunc, via UsageString; leaving
// UsageFunc alone would answer --help with a tree and every error with cobra's
// flat command list. Both funcs are inherited by descendants, so installing them
// on the root covers the whole command tree.
func SetupWithOptions(cmd *cobra.Command, opt HelpOptions) error {
	if cmd == nil {
		return errors.New("cobrahelptree: command is nil")
	}
	if err := opt.Validate(); err != nil {
		return err
	}

	agentMode := func() bool { return !opt.DisableAgent && IsAgentMode() }

	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		screen := RenderTreeHelp(c, opt.Catalog, opt.Tree)
		if agentMode() {
			screen = RenderAgentHelp(c, opt.Catalog, opt.Agent)
		}

		// Help the user explicitly asked for is a result, so it belongs on stdout,
		// matching cobra's own default help func (spf13/cobra#1002). Cobra's Print
		// falls back to stderr, which would break `--help | less` and `--help > file`.
		if _, err := fmt.Fprint(c.OutOrStdout(), screen); err != nil {
			c.PrintErrln(err)
		}
	})

	cmd.SetUsageFunc(func(c *cobra.Command) error {
		screen := RenderTreeUsage(c, opt.Catalog, opt.Tree)
		if agentMode() {
			screen = RenderAgentHelp(c, opt.Catalog, opt.Agent)
		}

		// Usage follows a flag or argument error, so it is diagnostic output and
		// stays on stderr, where cobra's own default usage func puts it. Reporting
		// and returning the write error matches that default too, so a caller that
		// overrides one of the two funcs still sees consistent behaviour.
		_, err := fmt.Fprint(c.OutOrStderr(), screen)
		if err != nil {
			c.PrintErrln(err)
		}
		return err
	})

	return nil
}
