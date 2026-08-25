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

### Rendered Output (`mytool --help`)

```
Multi-level CLI application

Usage:
  mytool [flags] [command]

Available Commands:
├─ user                                                Manage user accounts
│  ├─ create <name>                                    Create a new user
│  ╰─ delete <id>                                      Remove a user
╰─ version                                             Print version info

Flags:
  -h, --help   help for mytool

Use "mytool [command] --help" for more information about a command.
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
