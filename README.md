# cobra-help-tree

A lightweight, zero-configuration Go library that replaces Cobra's default flat help screens with clean, aligned ASCII command trees across arbitrary nesting levels.

# What It Does

- **Nested ASCII Hierarchy**: Renders multi-level command trees using rounded box-drawing glyphs (`├─ `, `╰─ `, `│  `).
- **Terminal Cell Alignment**: Measures display width in terminal cells rather than runes, so descriptions stay aligned when command names contain wide CJK characters or emoji.
- **Dynamic Terminal Width Protection**: Detects terminal width and truncates descriptions with a trailing ellipsis (`...`) before line wrapping occurs.
- **Native Dual-Mode (`AGENT=1`)**: Switches to token-conservative key-value help when `AGENT=1` is present.
- **Documented Positional Arguments**: Renders per-argument descriptions in both modes, in a column shared with the command tree, from a catalog keyed on command paths.
- **Help on Stdout**: Writes requested help screens to stdout so `--help` survives pipes and redirection.
- **Drop-In One-Liner Integration**: Call `cobrahelptree.Setup(rootCmd)` to replace every screen Cobra prints for an entire CLI application, on `--help` and after an error alike.

# How It Works

- Call one function on your root command, and every screen the CLI prints switches to a tree view: the `--help` screen, and the one shown after a mistyped flag or argument.
- Commands appear as an indented tree, with their descriptions lined up in a column down the right-hand side.
- Commands whose arguments you describe list them above the tree, in the same description column.
- Asking for help on a command group shows everything nested beneath it, not just the next level down.
- Descriptions too long for the window are shortened with `...`, so lines never wrap and break the columns.
- Setting `AGENT=1` swaps the decorated tree for compact output aimed at scripts and AI agents.
- Help lands on stdout, so it can be piped into a pager or saved to a file.

# How it Really Works

- `Setup` installs a help function through Cobra's `SetHelpFunc` and a usage function through `SetUsageFunc`, both on the root command, which every subcommand inherits unless it sets its own. Cobra renders `--help` through the former and the screen that follows a flag or argument error through the latter, so replacing only one leaves the other printing Cobra's flat command list.
- The two functions split the screen the way Cobra's own defaults do: `RenderTreeHelp` prints the long description and then the usage screen, `RenderTreeUsage` prints the usage screen alone, so an error does not repeat the whole command description.
- Cobra has no field describing a positional argument — `Use` carries the names as free text and `ValidArgs` is the enum of accepted values for the first positional argument — so per-argument descriptions come from a `TechCatalog` entry. Human mode renders them under `Arguments:` and agent mode under `args:`; a command's `ValidArgs` is reported separately under `valid_args:`, with the description Cobra packs behind a tab split off.
- The arguments block and the command tree are measured together, so one description column runs down the whole screen.
- `FormatCommandTree` walks `cmd.Commands()` recursively and records a branch prefix per node. Which commands count as listable is Cobra's own decision, `Command.IsAvailableCommand`, so hidden commands, deprecated commands, and commands that are neither runnable nor the parent of a runnable one are absent from the tree exactly as they are absent from Cobra's default help.
- A deprecated command is listed nowhere, which is Cobra's behaviour, so its own help screen is the one place it can still be announced: `RenderTreeUsage` prints `Deprecated: <your message>` above the usage block and agent mode reports a `deprecated:` key. Cobra prints the same string, but only once the command has already run.
- Cobra's generated `help` command is absent for the same reason: `IsAvailableCommand` excludes it by identity (`Parent().helpCommand == c`), and only Cobra's help *template* re-adds it by name, which this library does not do. The generated `completion` command is listed by default, as Cobra lists it; `TreeOptions.HideGeneratedCommands` drops it from the human screens. That filter is aimed at a command named `completion` directly under the root, which is the only place Cobra generates one — `InitDefaultCompletionCmd` returns early when a root child already uses that name — so a `completion` command of your own deeper in the tree is never mistaken for Cobra's.
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
- `TreeOptions` holds human-mode formatting and `AgentOptions` holds agent-mode settings, so each renderer receives only the struct it reads; `HelpOptions` composes both for `Setup`, the one caller that spans the two modes. A `TechCatalog` is content rather than formatting and both modes render its arguments, so it is a parameter of its own rather than a field of either struct.
- Agent output is emitted in full and clipped only when `AgentOptions.MaxLineWidth` is set. Clipping shortens the value half of a `key: value` line with `runewidth.Truncate`, skips block openers, and leaves a line untouched when its key allows no readable value, so clipped output still parses. Neither `$COLUMNS` nor the terminal size affects agent mode.

# Prerequisites

- [Go 1.26 or newer](https://go.dev/dl/) — the module declares `go 1.26.2`.
- [`github.com/spf13/cobra`](https://github.com/spf13/cobra) — the CLI framework this library renders help for.

Remaining dependencies ([`go-runewidth`](https://github.com/mattn/go-runewidth) for character width and [`golang.org/x/term`](https://pkg.go.dev/golang.org/x/term) for terminal size) resolve automatically through `go get`.

# Installation

```bash
go get github.com/alexgorbatchev/cobra-help-tree/v2
```

# Quick Start

The hierarchy below is the one `examples/demo` builds, so every screen in this README can be reproduced with `just run` (human mode) or `just run-ai` (`AGENT=1`), passing any arguments through: `just run user create --help`.

```go
package main

import (
	"fmt"
	"os"

	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v2"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mytool",
		Short: "Multi-level CLI application",
		Long:  "mytool manages user accounts and their API tokens.",
	}
	rootCmd.PersistentFlags().StringP("config", "c", "~/.config/mytool.yaml", "Path to configuration file")

	// 1. Add your subcommands at any depth. A leaf command needs a Run to appear
	// in help: Cobra treats one without it as unavailable, and so does this
	// library. Groups such as "user" are listed on the strength of their children.
	userCmd := &cobra.Command{Use: "user", Short: "Manage user accounts"}
	userCmd.AddCommand(&cobra.Command{Use: "create <name>", Short: "Create a new user", Run: createUser})
	userCmd.AddCommand(&cobra.Command{Use: "delete <id>", Short: "Remove a user", Run: deleteUser})

	tokenCmd := &cobra.Command{Use: "token", Short: "Manage API tokens for a user"}
	tokenCmd.AddCommand(&cobra.Command{Use: "issue <user-id>", Short: "Issue a new API token", Run: issueToken})
	tokenCmd.AddCommand(&cobra.Command{Use: "revoke <token-id>", Short: "Revoke an existing API token", Run: revokeToken})
	userCmd.AddCommand(tokenCmd)

	rootCmd.AddCommand(userCmd)
	rootCmd.AddCommand(&cobra.Command{Use: "version", Short: "Print the version and exit", Run: printVersion})

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
├─ completion               Generate the autocompletion script for the specified shell
│  ├─ bash                  Generate the autocompletion script for bash
│  ├─ fish                  Generate the autocompletion script for fish
│  ├─ powershell            Generate the autocompletion script for powershell
│  ╰─ zsh                   Generate the autocompletion script for zsh
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

The `completion` subtree is Cobra's, not yours: Cobra generates that command and lists it in its own help, and this library matches that default. `TreeOptions.HideGeneratedCommands` drops it when the tree should show only the commands the CLI itself defines:

```
Available Commands:
├─ user                     Manage user accounts
│  ├─ create <name>         Create a new user
│  ├─ delete <id>           Remove a user
│  ╰─ token                 Manage API tokens for a user
│     ├─ issue <user-id>    Issue a new API token
│     ╰─ revoke <token-id>  Revoke an existing API token
╰─ version                  Print the version and exit
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
  - completion: Generate the autocompletion script for the specified shell
  - user: Manage user accounts
  - version: Print the version and exit
flags:
  -c, --config string: Path to configuration file (default: "~/.config/mytool.yaml")
  -h, --help bool: help for mytool (default: "false")
```

`HideGeneratedCommands` does not apply here. Agent output describes the interface the binary accepts, and `completion` is a command it accepts, so agent mode always reports Cobra's own availability verdict.

### Command Metadata (`TechCatalog`)

Attach per-command arguments and metadata by supplying a `TechCatalog` keyed on full command paths. `Args` is read by both modes; the remaining fields are machine detail for agent mode:

```go
catalog := cobrahelptree.TechCatalog{
    "mytool user create": {
        Args: []cobrahelptree.ArgSpec{
            {Name: "<name>", Description: "Login name for the new user"},
            {Name: "[email]", Description: "Address invitations are sent to"},
        },
        MutatesDB: true,
        Metadata: map[string]string{
            "table": "users",
            "scope": "admin",
            "usage": "1 write per call",
        },
    },
}

if err := cobrahelptree.SetupWithOptions(rootCmd, cobrahelptree.HelpOptions{
    Catalog: catalog,
}); err != nil {
    return err
}
```

`ArgSpec.Name` is rendered verbatim, so it carries whatever convention the CLI documents (`<name>`, `[name]`, `<name...>`); the library adds no brackets of its own, because it cannot know whether an argument is required, optional or variadic. Human mode lists the arguments above the command tree, sharing its description column:

```
$ mytool user create --help
Create a new user

Usage:
  mytool user create <name> [flags]

Arguments:
  <name>              Login name for the new user
  [email]             Address invitations are sent to

Flags:
  -h, --help   help for create

Global Flags:
  -c, --config string   Path to configuration file (default "~/.config/mytool.yaml")
```

`Metadata` renders as a nested mapping with keys in alphabetical order, so the output is byte-identical across runs and a metadata key cannot collide with a reserved top-level key. Below, `usage` appears in both scopes without ambiguity:

```yaml
command: mytool user create
summary: Create a new user
usage: mytool user create <name> [flags]
args:
  - <name>: Login name for the new user
  - [email]: Address invitations are sent to
mutates_db: true
metadata:
  scope: admin
  table: users
  usage: 1 write per call
flags:
  -h, --help bool: help for create (default: "false")
```

A command's `cobra.ValidArgs` is reported under its own `valid_args:` key rather than as `args:`, because it is the enum of values the first positional argument accepts, not the argument list. Entries built with `cobra.CompletionWithDesc` carry their description behind a tab, which is split off so every line stays parseable:

```yaml
valid_args:
  - admin: Full administrative access
  - member
```

# Configuration

`Setup` applies the defaults. Use `SetupWithOptions` to customize rendering:

```go
err := cobrahelptree.SetupWithOptions(rootCmd, cobrahelptree.HelpOptions{
    Catalog: catalog, // Optional per-command arguments and metadata
    Tree: cobrahelptree.TreeOptions{
        IncludeRoot:   false, // Set true to render the root node at the top
        MinPadding:    4,     // Cells between the label column and descriptions
        MinLabelWidth: 12,    // Cells reserved for the command and argument columns
        TerminalWidth: 100,   // Manual column width limit (0 = auto-detect)

        // Set true to drop the completion command cobra generates, and its shell
        // subtree, from the human help screens. The default lists it, as cobra does.
        HideGeneratedCommands: false,
    },
    Agent: cobrahelptree.AgentOptions{
        MaxLineWidth: 0, // Clip agent lines to N cells (0 = unlimited)
    },
    DisableAgent: false, // Set true to disable automatic AGENT=1 mode switching
})
```

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `Catalog` | `TechCatalog` | `nil` | Per-command arguments, read by both modes, plus machine metadata read by `AGENT=1` mode |
| `Tree.IncludeRoot` | `bool` | `false` | Render the root command as the first line of the tree |
| `Tree.MinPadding` | `int` | `2` | Minimum cells between the label column and the description column |
| `Tree.MinLabelWidth` | `int` | `20` | Minimum cells reserved for the command and argument columns, so narrow screens keep a stable description column |
| `Tree.TerminalWidth` | `int` | `0` | Column limit for description clipping; `0` detects the terminal |
| `Tree.HideGeneratedCommands` | `bool` | `false` | Drop the completion command Cobra generates, and its shell subtree, from the human help screens. The default matches Cobra, which lists it. Agent mode is unaffected |
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

Help screens that the user asked for are written to stdout, matching Cobra's own default help function. The usage screen printed after an invalid flag or argument goes to stderr, where diagnostics belong, and carries the tree rather than the long description.

```bash
mytool --help | less        # paginates the help screen
mytool --help > help.txt    # captures the help screen
mytool --bogus-flag         # error and usage go to stderr
```

Calls to `SetOut` on the root command redirect help output as usual.

# API

| Function | Purpose |
| :--- | :--- |
| `Setup(cmd) error` | Replace the help and usage screens of `cmd` and its descendants using the defaults; returns an error when `cmd` is nil |
| `SetupWithOptions(cmd, opt) error` | The same from a `HelpOptions`; returns an error and installs nothing when `cmd` is nil or `opt` is invalid |
| `FormatCommandTree(root, opt) string` | Render just the command tree from a `TreeOptions` |
| `RenderTreeHelp(cmd, cat, opt) string` | Render a full human-mode help screen: the long description followed by the usage screen |
| `RenderTreeUsage(cmd, cat, opt) string` | Render the human-mode usage screen alone, as printed after a flag or argument error |
| `RenderAgentHelp(cmd, cat, opt) string` | Render the `AGENT=1` screen, used for both help and usage |
| `TreeOptions.Validate() error` | Report the first invalid field in a `TreeOptions` |
| `AgentOptions.Validate() error` | Report the first invalid field in an `AgentOptions` |
| `HelpOptions.Validate() error` | Report the first invalid field, delegating to `Tree` then `Agent` |
| `IsAgentMode() bool` | Report whether `AGENT` is truthy |
| `GetTerminalWidth() int` | Detected terminal width in columns, or `0` when not a terminal |

# Development

Tasks run through [`just`](https://just.systems). The same recipes run in CI, so a green `just check` locally means a green build.

| Recipe | Purpose |
| :--- | :--- |
| `just run <args>` | Run `examples/demo` in human mode, e.g. `just run user create --help` |
| `just run-ai <args>` | Run `examples/demo` with `AGENT=1` |
| `just test` | Run the unit tests with coverage reported |
| `just coverage` | Run every test under `-race`, measure the library packages, and fail below the 90% statement coverage floor |
| `just check` | `just lint` followed by `just coverage`; what CI runs |
| `just fmt` | Format the source with `go fmt` |
| `just lint` | Fails on unformatted files (`gofmt -l`), `go vet` findings, or an untidy `go.mod` (`go mod tidy -diff`) |

The floor lives in the `min_coverage` variable at the top of the `justfile`. Every package's tests run, but only the library packages are measured, so `examples/demo` is exercised by CI without its own statements diluting the number.

Unit tests are offline, hermetic and deterministic. Tests that assert which stream output lands on replace `os.Stdout` and `os.Stderr` with pipes, because Cobra's `SetOut` writer backs both `OutOrStdout` and `OutOrStderr` and would mask the difference.

# License

MIT License (c) 2026 Alex Gorbatchev
