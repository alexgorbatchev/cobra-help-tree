package cobrahelptree

import (
	"bytes"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

// leafCommand returns a runnable command with no children of its own.
//
// The Run matters. The renderers decide what to list with cobra's own
// Command.IsAvailableCommand, which drops a leaf that has no Run because
// invoking it would do nothing, so a fixture leaf has to be as runnable as a
// real CLI's leaf commands are.
func leafCommand(use, short string) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Run: func(*cobra.Command, []string) {}}
}

func buildSampleCommandHierarchy() *cobra.Command {
	root := &cobra.Command{
		Use:   "app",
		Short: "Sample CLI application",
		Long:  "app is a sample CLI tool demonstrating nested command hierarchies.",
	}
	root.PersistentFlags().StringP("config", "c", "config.yaml", "Configuration file path")

	playlistCmd := &cobra.Command{
		Use:   "playlist",
		Short: "Inspect and manage playlists",
	}
	playlistCmd.AddCommand(leafCommand("list", "Display all playlists"))
	playlistCmd.AddCommand(leafCommand("create <name>", "Create a playlist"))

	playlistTrackCmd := &cobra.Command{
		Use:   "track",
		Short: "Manage playlist track memberships",
	}
	playlistTrackCmd.AddCommand(leafCommand("add <pl|id> <tr|id|path...>", "Add track to playlist"))
	playlistTrackCmd.AddCommand(leafCommand("rm <pl|id> <tr|id|path...>", "Remove track from playlist"))
	playlistCmd.AddCommand(playlistTrackCmd)

	trackCmd := &cobra.Command{
		Use:   "track",
		Short: "Manage audio files",
	}
	trackCmd.AddCommand(leafCommand("add <path...>", "Import audio file"))

	root.AddCommand(playlistCmd)
	root.AddCommand(trackCmd)

	return root
}

// withNonTerminalStdout points os.Stdout at a pipe for the duration of the test.
// A pipe is not a terminal, so term.GetSize always fails and the fallback branch
// of GetTerminalWidth becomes deterministic regardless of how the suite is run.
func withNonTerminalStdout(t *testing.T) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}

	orig := os.Stdout
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = orig
		if err := w.Close(); err != nil {
			t.Errorf("closing stdout pipe writer: %v", err)
		}
		if err := r.Close(); err != nil {
			t.Errorf("closing stdout pipe reader: %v", err)
		}
	})
}

// descColumn returns the display column at which short begins on the tree line
// containing it, or -1 when no line contains it.
func descColumn(tree, short string) int {
	for _, line := range strings.Split(strings.TrimRight(tree, "\n"), "\n") {
		if at := strings.Index(line, short); at >= 0 {
			return runewidth.StringWidth(line[:at])
		}
	}
	return -1
}

// buildTwoCommandRoot returns a root whose widest command column is "├─ add",
// six cells wide, which keeps expected description columns easy to state.
func buildTwoCommandRoot() *cobra.Command {
	root := &cobra.Command{Use: "app", Short: "App"}
	root.AddCommand(leafCommand("add", "Add an item"))
	root.AddCommand(leafCommand("rm", "Remove an item"))
	return root
}

const (
	widestCommand = 6
	padding       = 2
)

// helpOutput runs `--help` and returns what the help function wrote. It installs
// a writer via SetOut, so it cannot distinguish stdout from stderr; stream choice
// is covered separately by TestSetupWritesHelpToStdout.
func helpOutput(t *testing.T, root *cobra.Command) string {
	t.Helper()

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("root --help execution failed: %v", err)
	}
	return buf.String()
}

// captureStdio swaps the real os.Stdout and os.Stderr for pipes while fn runs.
// Tests that install a writer via cmd.SetOut cannot tell stdout from stderr,
// because cobra's OutOrStdout and OutOrStderr both return that same writer.
// Replacing the process streams is the only way to exercise the fallback that
// actually ships to callers.
func captureStdio(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stderr pipe: %v", err)
	}

	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	restore := func() { os.Stdout, os.Stderr = origOut, origErr }
	t.Cleanup(restore)

	// Drain both pipes concurrently so a writer can never block on a full buffer.
	var wg sync.WaitGroup
	var outBuf, errBuf bytes.Buffer
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&outBuf, outR) // ends at EOF once the write end closes
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&errBuf, errR)
	}()

	fn()

	restore()
	if err := outW.Close(); err != nil {
		t.Fatalf("closing stdout pipe: %v", err)
	}
	if err := errW.Close(); err != nil {
		t.Fatalf("closing stderr pipe: %v", err)
	}
	wg.Wait()

	return outBuf.String(), errBuf.String()
}

// failingWriter rejects every write, standing in for a closed pipe or full disk.
type failingWriter struct{ err error }

func (w failingWriter) Write(p []byte) (int, error) { return 0, w.err }

// countTopLevelKeys counts lines that open a key at column 0, ignoring nested entries.
func countTopLevelKeys(out, key string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, key+": ") {
			n++
		}
	}
	return n
}

// findCommand returns the descendant of root whose full command path is path.
func findCommand(t *testing.T, root *cobra.Command, path string) *cobra.Command {
	t.Helper()

	var found *cobra.Command
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c.CommandPath() == path {
			found = c
			return
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)

	if found == nil {
		t.Fatalf("command %q not found under %q", path, root.CommandPath())
	}
	return found
}

// argCatalog documents the positional arguments of two commands in the sample
// hierarchy: a leaf, and a group that also has subcommands.
func argCatalog() TechCatalog {
	return TechCatalog{
		"app playlist create": TechInfo{
			Args: []ArgSpec{
				{Name: "<name>", Description: "Name of the new playlist"},
				{Name: "[parent]", Description: "Folder to create it under"},
			},
		},
		"app playlist": TechInfo{
			Args: []ArgSpec{{Name: "<playlist>", Description: "Playlist name or id"}},
		},
	}
}

// listedCommands returns the command names a screen actually lists, sorted. It
// reads both shapes the renderers produce: a tree branch ("├─ name") and an
// agent-mode list item ("  - name: summary"). Matching on entries rather than on
// raw substrings keeps prose such as the trailing `--help` hint out of the result.
func listedCommands(screen string) []string {
	var names []string
	for _, line := range strings.Split(screen, "\n") {
		var entry string
		switch {
		case strings.Contains(line, "─ "):
			_, entry, _ = strings.Cut(line, "─ ")
		case strings.HasPrefix(line, "  - "):
			entry = strings.TrimPrefix(line, "  - ")
		default:
			continue
		}
		name, _, _ := strings.Cut(strings.TrimSpace(entry), " ")
		names = append(names, strings.TrimSuffix(name, ":"))
	}
	slices.Sort(names)
	return names
}
