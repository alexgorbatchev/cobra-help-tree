# cobra-help-tree

A lightweight, zero-configuration Go library that replaces Cobra's default flat help screens with clean, aligned ASCII command trees across arbitrary nesting levels.

# What It Does

- **Nested ASCII Hierarchy**: Renders multi-level command trees using rounded box-drawing glyphs (`├─ `, `╰─ `, `│  `).
- **Terminal Cell Alignment**: Measures display width in terminal cells rather than runes, so descriptions stay aligned when command names contain wide CJK characters or emoji.
- **Dynamic Terminal Width Protection**: Detects terminal width and truncates descriptions with a trailing ellipsis (`...`) before line wrapping occurs.
- **Native Dual-Mode (`AGENT=1`)**: Switches to token-conservative key-value help when `AGENT=1` is present.
- **Help on Stdout**: Writes requested help screens to stdout so `--help` survives pipes and redirection.
- **Drop-In One-Liner Integration**: Call `cobrahelptree.Setup(rootCmd)` to upgrade an entire CLI application.

# How It Works

- Call one function on your root command, and every `--help` screen in the CLI switches to a tree view.
- Commands appear as an indented tree, with their descriptions lined up in a column down the right-hand side.
- Asking for help on a command group shows everything nested beneath it, not just the next level down.
- Descriptions too long for the window are shortened with `...`, so lines never wrap and break the columns.
- Setting `AGENT=1` swaps the decorated tree for compact output aimed at scripts and AI agents.
- Help lands on stdout, so it can be piped into a pager or saved to a file.

# How it Really Works

- `Setup` installs a help function through Cobra's `SetHelpFunc` on the root command, which every subcommand inherits unless it sets its own.
- `FormatCommandTree` walks `cmd.Commands()` recursively, skipping hidden commands plus the generated `help` and `completion` commands, and records a branch prefix per node.
- Column width is measured in terminal cells with `runewidth.StringWidth`, because a CJK ideograph or emoji is a single rune occupying two cells; rune counts would shift the description column.
- Descriptions are clipped with `runewidth.Truncate` against the detected width, resolved from `$COLUMNS` first and then `term.GetSize` on the stdout file descriptor, falling back to no clipping when neither reports a size.
- `RenderAgentHelp` emits flat `key: value` lines. Caller-supplied `TechInfo.Metadata` is nested under its own `metadata:` key and sorted, so it cannot collide with a reserved key and renders byte-identically across runs.
- Both renderers report the same flags for a command, combining its local and inherited sets rather than reading `cmd.Flags()`, which carries persistent flags only after Cobra has merged them.
- Rendering mutates the command: Cobra's flag merge writes to the root and every ancestor, so the renderers are not safe to call concurrently on commands sharing a root. Commands installed through `Setup` are unaffected, because Cobra executes a command tree on one goroutine.

## Unexpected flags in help

Cobra folds `pflag.CommandLine` into every root command's persistent flags, so a flag registered through a package-level `pflag.String`, `pflag.Bool` and so on — by your code or by any library linked into the binary — becomes a real flag of your CLI. Help lists it because the binary genuinely accepts it:

```console
$ mytool --injected-global=xyz    # parses successfully
```

Suppressing it in help would document an interface the binary does not have, so these renderers show it. To keep such a flag out, register it on a `FlagSet` of your own instead of pflag's global one. This concerns pflag's global set only; the standard library's `flag` package is never consulted by Cobra, and its flags appear neither in help nor in parsing.
- Help is written to `c.OutOrStdout()`, matching Cobra's own default help function. Cobra's `c.Print` falls back to stderr, which would break redirection.
- Every `TreeOptions` field falls back to its default when left at zero, so `TreeOptions{}` is fully configured; `Validate` rejects negative sizing values rather than silently coercing them, and a nil command is reported rather than ignored.
- `TreeOptions` holds human-mode formatting and `AgentOptions` holds agent-mode settings, so each renderer receives only the struct it reads; `HelpOptions` composes both for `Setup`, the one caller that spans the two modes.
- Agent output is emitted in full and clipped only when `AgentOptions.MaxLineWidth` is set. Clipping shortens the value half of a `key: value` line with `runewidth.Truncate`, skips block openers, and leaves a line untouched when its key allows no readable value, so clipped output still parses. Neither `$COLUMNS` nor the terminal size affects agent mode.

# Prerequisites

- [Go 1.26 or newer](https://go.dev/dl/) — the module declares `go 1.26.2`.
- [`github.com/spf13/cobra`](https://github.com/spf13/cobra) — the CLI framework this library renders help for.

Remaining dependencies ([`go-runewidth`](https://github.com/mattn/go-runewidth) for character width and [`golang.org/x/term`](https://pkg.go.dev/golang.org/x/term) for terminal size) resolve automatically through `go get`.

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
		Long:  "mytool manages user accounts and their API tokens.",
	}
	rootCmd.PersistentFlags().StringP("config", "c", "~/.config/mytool.yaml", "Path to configuration file")

	// 1. Add your subcommands at any depth
	userCmd := &cobra.Command{Use: "user", Short: "Manage user accounts"}
	userCmd.AddCommand(&cobra.Command{Use: "create <name>", Short: "Create a new user"})
	userCmd.AddCommand(&cobra.Command{Use: "delete <id>", Short: "Remove a user"})

	tokenCmd := &cobra.Command{Use: "token", Short: "Manage API tokens for a user"}
	tokenCmd.AddCommand(&cobra.Command{Use: "issue <user-id>", Short: "Issue a new API token"})
	tokenCmd.AddCommand(&cobra.Command{Use: "revoke <token-id>", Short: "Revoke an existing API token"})
	userCmd.AddCommand(tokenCmd)

	rootCmd.AddCommand(userCmd)
	rootCmd.AddCommand(&cobra.Command{Use: "version", Short: "Print the version and exit"})

	// 2. Enable tree help screens
	if err := cobrahelptree.Setup(rootCmd); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

### Rendered Output (Human Mode: `AGENT=0` / Default)

`mytool --help` renders the whole hierarchy in one aligned tree:

```
mytool manages user accounts and their API tokens.

Usage:
  mytool [flags] [command]

Available Commands:
├─ user                     Manage user accounts
│  ├─ create <name>         Create a new user
│  ├─ delete <id>           Remove a user
│  ╰─ token                 Manage API tokens for a user
│     ├─ issue <user-id>    Issue a new API token
│     ╰─ revoke <token-id>  Revoke an existing API token
╰─ version                  Print the version and exit

Flags:
  -c, --config string   Path to configuration file (default "~/.config/mytool.yaml")
  -h, --help            help for mytool

Use "mytool [command] --help" for more information about a command.
```

Requesting help on a command group renders that group's full subtree, re-anchored at depth zero:

```
$ mytool user --help
Manage user accounts

Usage:
  mytool user [flags] [command]

Available Commands:
├─ create <name>         Create a new user
├─ delete <id>           Remove a user
╰─ token                 Manage API tokens for a user
   ├─ issue <user-id>    Issue a new API token
   ╰─ revoke <token-id>  Revoke an existing API token

Flags:
  -h, --help   help for user

Global Flags:
  -c, --config string   Path to configuration file (default "~/.config/mytool.yaml")

Use "mytool user [command] --help" for more information about a command.
```

### Rendered Output (Agent Mode: `AGENT=1`)

The same command under `AGENT=1` drops padding, glyphs, and dividers in favour of flat key-value lines:

```yaml
command: mytool
summary: Multi-level CLI application
description: mytool manages user accounts and their API tokens.
usage: mytool [flags]
subcommands:
  - user: Manage user accounts
  - version: Print the version and exit
flags:
  -c, --config string: Path to configuration file (default: "~/.config/mytool.yaml")
  -h, --help bool: help for mytool (default: "false")
```

### Machine Metadata (`TechCatalog`)

Attach per-command metadata for agent mode by supplying a `TechCatalog` keyed on full command paths:

```go
catalog := cobrahelptree.TechCatalog{
    "mytool user create": {
        Args:      "<name>",
        MutatesDB: true,
        Metadata: map[string]string{
            "table": "users",
            "scope": "admin",
            "usage": "1 write per call",
        },
    },
}

if err := cobrahelptree.SetupWithOptions(rootCmd, cobrahelptree.HelpOptions{
    Agent: cobrahelptree.AgentOptions{TechCatalog: catalog},
}); err != nil {
    return err
}
```

`Metadata` renders as a nested mapping with keys in alphabetical order, so the output is byte-identical across runs and a metadata key cannot collide with a reserved top-level key. Below, `usage` appears in both scopes without ambiguity:

```yaml
command: mytool user create
summary: Create a new user
usage: mytool user create <name> [flags]
args: <name>
mutates_db: true
metadata:
  scope: admin
  table: users
  usage: 1 write per call
flags:
  -h, --help bool: help for create (default: "false")
```

# Configuration

`Setup` applies the defaults. Use `SetupWithOptions` to customize rendering:

```go
err := cobrahelptree.SetupWithOptions(rootCmd, cobrahelptree.HelpOptions{
    Tree: cobrahelptree.TreeOptions{
        IncludeRoot:     false, // Set true to render the root node at the top
        MinPadding:      4,     // Cells between the command column and descriptions
        MinCommandWidth: 12,    // Cells reserved for the command column
        TerminalWidth:   100,   // Manual column width limit (0 = auto-detect)
    },
    Agent: cobrahelptree.AgentOptions{
        TechCatalog:  catalog, // Optional machine metadata catalog
        MaxLineWidth: 0,       // Clip agent lines to N cells (0 = unlimited)
    },
    DisableAgent: false, // Set true to disable automatic AGENT=1 mode switching
})
```

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `Tree.IncludeRoot` | `bool` | `false` | Render the root command as the first line of the tree |
| `Tree.MinPadding` | `int` | `2` | Minimum cells between the command column and the description column |
| `Tree.MinCommandWidth` | `int` | `20` | Minimum cells reserved for the command column, so narrow trees keep a stable description column |
| `Tree.TerminalWidth` | `int` | `0` | Column limit for description clipping; `0` detects the terminal |
| `Agent.TechCatalog` | `TechCatalog` | `nil` | Per-command machine metadata used in `AGENT=1` mode |
| `Agent.MaxLineWidth` | `int` | `0` | Clip the value half of each agent-mode line to fit this many cells; `0` is unlimited |
| `DisableAgent` | `bool` | `false` | Always render the human tree, ignoring `AGENT` |

`TreeOptions` carries human-mode formatting and `AgentOptions` carries `AGENT=1` settings, so each renderer takes only the struct it reads. `HelpOptions` composes the two because `Setup` is the one caller that spans both modes.

The two width fields are deliberately different. `Tree.TerminalWidth` defaults to auto-detecting the terminal, because human help must not wrap. `Agent.MaxLineWidth` defaults to unlimited and never auto-detects, because agent output is machine-read and full untruncated content is the contract; clipping is something a caller opts into.

`Agent.MaxLineWidth` is a ceiling rather than a hard guarantee, because a clipped line still has to parse. Only the value half of a `key: value` line is shortened, block openers such as `metadata:` are never touched, and a line whose key leaves no room for a readable value is emitted in full. Clipping therefore never produces a truncated key or a key with nothing behind it:

```yaml
# AgentOptions{MaxLineWidth: 24}
command: mytool
summary: Multi-level ...
usage: mytool
metadata:
  a_very_long_metadata_key_name: vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv
  note: vvvvvvvvvvvvv...
```

`summary` and `note` are clipped with their keys intact, while `a_very_long_metadata_key_name` keeps its full value because clipping it could only have produced an unreadable field.

Every field falls back to its default when left at its zero value, so `HelpOptions{}` is equivalent to a fully populated default and a partially populated struct keeps the defaults for the rest. A negative sizing value is a mistake rather than a request for the default, so `SetupWithOptions` returns an error and installs nothing:

```go
err := cobrahelptree.SetupWithOptions(rootCmd, cobrahelptree.HelpOptions{
    Tree: cobrahelptree.TreeOptions{MinPadding: -1},
})
// cobrahelptree: MinPadding must be zero (use the default) or positive, got -1
```

# Output Streams

Help screens that the user asked for are written to stdout, matching Cobra's own default help function. Usage text that Cobra prints after an invalid flag or argument remains on stderr, where diagnostics belong.

```bash
mytool --help | less        # paginates the help screen
mytool --help > help.txt    # captures the help screen
mytool --bogus-flag         # error and usage go to stderr
```

Calls to `SetOut` on the root command redirect help output as usual.

# API

| Function | Purpose |
| :--- | :--- |
| `Setup(cmd) error` | Install tree help on `cmd` using the defaults; returns an error when `cmd` is nil |
| `SetupWithOptions(cmd, opt) error` | Install tree help on `cmd` using a `HelpOptions`; returns an error and installs nothing when `cmd` is nil or `opt` is invalid |
| `FormatCommandTree(root, opt) string` | Render just the command tree from a `TreeOptions` |
| `RenderTreeHelp(cmd, opt) string` | Render a full human-mode help screen from a `TreeOptions` |
| `RenderAgentHelp(cmd, opt) string` | Render a full `AGENT=1` help screen from an `AgentOptions` |
| `TreeOptions.Validate() error` | Report the first invalid field in a `TreeOptions` |
| `AgentOptions.Validate() error` | Report the first invalid field in an `AgentOptions` |
| `HelpOptions.Validate() error` | Report the first invalid field, delegating to `Tree` then `Agent` |
| `IsAgentMode() bool` | Report whether `AGENT` is truthy |
| `GetTerminalWidth() int` | Detected terminal width in columns, or `0` when not a terminal |

# License

MIT License (c) 2026 Alex Gorbatchev
