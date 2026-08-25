# cobra-help-tree

A lightweight, zero-configuration Go library that replaces Cobra's default flat help screens with clean, aligned ASCII command trees across arbitrary nesting levels.

# Features

- **Nested ASCII Hierarchy**: Automatically discovers and renders multi-level command trees using modern rounded box-drawing glyphs (`├─ `, `╰─ `, `│  `).
- **Exact Rune Column Alignment**: Accurately calculates UTF-8 display widths so command descriptions remain vertically aligned regardless of branch depth.
- **Dynamic Terminal Width Protection**: Detects terminal width via `golang.org/x/term` and `$COLUMNS` and truncates descriptions with trailing ellipsis (`...`) before line wrapping occurs.
- **Drop-In One-Liner Integration**: Call `cobrahelptree.Setup(rootCmd)` to upgrade your entire CLI application.
- **Zero Heavy Dependencies**: Built solely on Cobra and Go standard/extended terminal tooling.

# Installation

```bash
go get github.com/alexgorbatchev/cobra-help-tree
```

# Quick Start

```go
package main

import (
	"fmt"
	"os"

	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mytool",
		Short: "Multi-level CLI application",
	}

	// 1. Add your subcommands
	userCmd := &cobra.Command{Use: "user", Short: "Manage user accounts"}
	userCmd.AddCommand(&cobra.Command{Use: "create <name>", Short: "Create a new user"})
	userCmd.AddCommand(&cobra.Command{Use: "delete <id>", Short: "Remove a user"})
	rootCmd.AddCommand(userCmd)

	// 2. Enable tree help screen
	cobrahelptree.Setup(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

### Rendered Output Example

```
engine-cli is a command-line tool designed for DJs to inspect, repair, synchronize,
and safely back up Engine DJ music libraries and album cover art on your computer or external USB drives.

Usage:
  engine-cli [flags] [command]

Available Commands:
├─ artwork                                             Manage and fix album cover artwork
│  ├─ normalize                                        Crop album cover art to 1:1 square for DJ jo...
│  ╰─ prune                                            Clean up unused cover art and reclaim disk s...
├─ backup                                              Create a safety backup snapshot of your Engi...
├─ playlist                                            Inspect and manage playlists and folders
│  ├─ create <name>                                    Create a new playlist or folder
│  ├─ inspect <pl|id>                                  Display all songs inside a playlist
│  ├─ list                                             Display all playlists and folders in your li...
│  ├─ move <pl|id>                                     Relocate a playlist or folder under a parent...
│  ├─ rm <pl|id>                                       Delete a playlist or folder
│  ╰─ track                                            Manage track memberships inside playlists
│     ├─ add <pl|id> <tr|id|path...>                   Add track(s) to a playlist
│     ├─ move <src-pl|id> <dst-pl|id> <tr|id|path...>  Move track(s) from one playlist to another
│     ╰─ rm <pl|id> <tr|id|path...>                    Remove track(s) from a playlist
├─ restore <backup-dir>                                Restore your Engine DJ library from a previo...
├─ sync                                                Update song information in Engine DJ from au...
├─ track                                               Manage audio files and collection tracks
│  ├─ add <path...>                                    Import audio file(s) into your collection
│  ├─ move <tr|id|path> <dest-path>                    Move an audio file on disk and update its co...
│  ├─ relocate                                         Batch-fix audio file paths across drives or ...
│  ╰─ rm <tr|id|path...>                               Remove track(s) from your collection
╰─ verify                                              Check your library for missing songs, broken...

Flags:
  -h, --help             help for engine-cli
  -l, --lib-dir string   Path to the Engine Library directory (default "/Users/alex/Music/Engine Library")
      --no-backup        Disable automatic backup snapshot before modifying database
  -v, --version          version for engine-cli

Use "engine-cli [command] --help" for more information about a command.
```

# Configuration Options

Customize tree rendering by passing `TreeOptions`:

```go
cobrahelptree.Setup(rootCmd, cobrahelptree.TreeOptions{
    IncludeRoot:   false, // Set true to render root node at top
    MinPadding:    4,     // Spacing between command and description
    TerminalWidth: 100,   // Manual column width limit (0 = auto-detect)
})
```

# License

MIT License (c) 2026 Alex Gorbatchev
