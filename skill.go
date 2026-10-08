package cobrahelptree

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// skillCommandName is the command an agent runs to read the skill. It is fixed
// rather than configurable: the alert names it, and an agent that has met one CLI
// carrying a skill then knows the command for every other.
const skillCommandName = "skill"

// skillAnnotation marks the skill command this library adds. The alert is tied to
// the mark rather than to the name, so a command of the CLI's own that is merely
// called "skill" never has a document promised on its behalf.
const skillAnnotation = "cobrahelptree.skill"

// checkSkillTarget reports why cmd cannot carry the skill command, when it
// cannot. The command is the CLI's top-level entry to its guide, so it belongs on
// the root, and it must not shadow a command the CLI already defines. The name
// test is the one cobra applies before adding its own completion command.
func checkSkillTarget(cmd *cobra.Command) error {
	if cmd.HasParent() {
		return fmt.Errorf("cobrahelptree: Skill needs the root command, got %q", cmd.CommandPath())
	}
	for _, sub := range cmd.Commands() {
		if sub.Name() == skillCommandName || sub.HasAlias(skillCommandName) {
			return fmt.Errorf("cobrahelptree: %q already has a skill command", cmd.CommandPath())
		}
	}
	return nil
}

// newSkillCommand returns the command that prints skill.
//
// It writes the document byte for byte in both modes: the skill is what an agent
// reads before anything else, so it gets no alert of its own, no reformatting and
// no trailing newline the file did not have. It takes no arguments and defines no
// flags, and it returns a failed write instead of reporting success for a guide
// the reader never received.
func newSkillCommand(skill string) *cobra.Command {
	return &cobra.Command{
		Use:               skillCommandName,
		Short:             "Print the usage guide for AI agents",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Annotations:       map[string]string{skillAnnotation: "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := io.WriteString(cmd.OutOrStdout(), skill); err != nil {
				return fmt.Errorf("writing the skill: %w", err)
			}
			return nil
		},
	}
}

// SkillOmissions returns what skill fails to mention of the interface below root,
// one sorted entry per omission, and nil when it mentions everything. A CLI's
// tests call it with the embedded skill, so that adding or renaming a command,
// flag, argument or environment variable fails the build until the guide follows.
//
// The interface is what the binary accepts once it runs: every command an agent
// can invoke, by full path, the commands and flags cobra generates among them;
// every flag those commands take, with its shorthand; and the arguments and
// environment variables the catalog documents. Hidden and deprecated commands and
// hidden flags are left out, as they are left out of help.
//
// A name counts as mentioned only where it stands as a whole word, so "--admin" is
// not found inside "--administrator" and "-c" is not found inside "--config".
//
// A flag the skill names must also have its type stated, and its default when it
// has one, on a line that names the flag: the type as pflag reports it ("bool",
// "string", "int", "duration" and so on, in any letter case) and the default as
// pflag prints it, each as a whole word. A flag retyped or given a new default
// then fails until its line is corrected. A default equal to the type's zero
// value is not asked for, as help does not print it. What the skill says beyond
// that, a side effect or an error for instance, is not judged.
//
// It mutates root the way executing the CLI does, by adding cobra's generated
// commands and flags, and the package Concurrency note applies.
func SkillOmissions(root *cobra.Command, cat TechCatalog, skill string) []string {
	if root == nil {
		return nil
	}

	// Cobra adds these when the CLI executes, so a tree that has not run yet does
	// not hold the whole interface. Both calls do nothing the second time.
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()

	lines := strings.Split(skill, "\n")
	omissions := map[string]struct{}{}

	var visit func(cmd *cobra.Command)
	visit = func(cmd *cobra.Command) {
		// Generated on execution like the commands above, per command.
		cmd.InitDefaultHelpFlag()
		cmd.InitDefaultVersionFlag()

		path := cmd.CommandPath()
		if !mentionsWord(skill, path) {
			omissions["command "+path] = struct{}{}
		}

		// A flag shared by many commands is one flag to the reader, so it is
		// reported once however many commands accept it.
		applicableFlags(cmd).VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			long := "--" + f.Name
			if f.Shorthand != "" && !mentionsWord(skill, "-"+f.Shorthand) {
				omissions[fmt.Sprintf("flag -%s, the shorthand of %s", f.Shorthand, long)] = struct{}{}
			}

			// The lines naming the flag are where a reader looks for its type and
			// default. A flag named nowhere is one omission, not three.
			var naming []string
			for _, line := range lines {
				if mentionsWord(line, long) {
					naming = append(naming, line)
				}
			}
			if len(naming) == 0 {
				omissions["flag "+long] = struct{}{}
				return
			}

			typeName := f.Value.Type()
			if !slices.ContainsFunc(naming, func(line string) bool {
				return mentionsWord(strings.ToLower(line), strings.ToLower(typeName))
			}) {
				omissions[fmt.Sprintf("flag %s: its type %s is not stated on a line that names it", long, typeName)] = struct{}{}
			}
			if hasNonZeroDefault(f) && !slices.ContainsFunc(naming, func(line string) bool {
				return mentionsWord(line, f.DefValue)
			}) {
				omissions[fmt.Sprintf("flag %s: its default %q is not stated on a line that names it", long, f.DefValue)] = struct{}{}
			}
		})

		info := cat[path]
		for _, arg := range info.Args {
			// Argument names carry their own brackets, so they are matched as written.
			if !strings.Contains(skill, arg.Name) {
				omissions[fmt.Sprintf("argument %s of %s", arg.Name, path)] = struct{}{}
			}
		}
		for _, env := range info.Env {
			if !mentionsWord(skill, env.Name) {
				omissions["environment variable "+env.Name] = struct{}{}
			}
		}

		for _, sub := range cmd.Commands() {
			if sub.IsAvailableCommand() || isGeneratedHelpCommand(sub) {
				visit(sub)
			}
		}
	}
	visit(root)

	if len(omissions) == 0 {
		return nil
	}
	return slices.Sorted(maps.Keys(omissions))
}

// isGeneratedHelpCommand reports whether cmd is the help command cobra
// generated. Cobra exposes no handle on it, but its availability test fails a
// visible, current, runnable command for one reason only, which is being that
// command. Help screens leave it out as cobra's own predicate does; a skill has to
// name it, because the binary accepts it.
func isGeneratedHelpCommand(cmd *cobra.Command) bool {
	return !cmd.Hidden && cmd.Deprecated == "" && cmd.Runnable() && !cmd.IsAvailableCommand()
}

// mentionsWord reports whether text contains word standing on its own, bounded on
// both sides by the edge of the text or by a character that cannot continue a
// command, flag or variable name.
func mentionsWord(text, word string) bool {
	for from := 0; ; {
		at := strings.Index(text[from:], word)
		if at < 0 {
			return false
		}
		start := from + at
		end := start + len(word)

		before, _ := utf8.DecodeLastRuneInString(text[:start])
		after, _ := utf8.DecodeRuneInString(text[end:])
		if !continuesName(before, start > 0) && !continuesName(after, end < len(text)) {
			return true
		}
		from = start + 1
	}
}

// continuesName reports whether r, the character beside a candidate word, would
// make that word part of a longer name. present is false at an edge of the text,
// where there is no character.
func continuesName(r rune, present bool) bool {
	return present && (r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r))
}

// skillAlert returns the line that opens every agent screen of a CLI carrying the
// skill command, and "" for a CLI without one. It is derived from the command
// tree rather than from an option, so the alert can never name a command the
// binary does not have.
func skillAlert(cmd *cobra.Command) string {
	root := cmd.Root()
	for _, sub := range root.Commands() {
		if isSkillCommand(sub) {
			return fmt.Sprintf("ALERT: Agents must read `AGENT=1 %s %s` before using this tool.\n", root.CommandPath(), skillCommandName)
		}
	}
	return ""
}

// isSkillCommand reports whether cmd is the skill command this library added.
func isSkillCommand(cmd *cobra.Command) bool {
	_, marked := cmd.Annotations[skillAnnotation]
	return marked
}
