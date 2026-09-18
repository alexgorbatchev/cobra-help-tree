// Package cobrahelptree renders nested Cobra command hierarchies as aligned
// ASCII trees in human mode, and as compact key-value output when AGENT=1.
//
// Setup is the whole integration: it replaces both screens cobra prints, the
// help screen behind --help and the usage screen that follows a flag or argument
// error, for a command and every descendant.
//
// Cobra has no field for describing a positional argument, so per-argument
// descriptions come from a TechCatalog keyed on full command paths. Both modes
// render them: as an "Arguments:" block sharing the command tree's description
// column in human mode, and under an "args:" key in agent mode.
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
