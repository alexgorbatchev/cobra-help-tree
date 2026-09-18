---
created_on: 2026-08-24 16:30
last_modified: 2026-09-17 19:08
status: current
---

# cobra-help-tree

Reusable Go library for rendering nested Cobra CLI command hierarchies as clean, aligned ASCII trees in terminal help screens.

## Commands
- **Run Tests:** `just test` (`go test -v -cover ./...`)
- **Run Static Analysis & Tests:** `just check` (`go vet ./... && go test -v ./...`)
- **Run Linter:** `just lint` (`go vet ./...`)
- **Format Source Code:** `just fmt` (`go fmt ./...`)

## Setup & Environment
- **Prerequisites:** Go 1.26+, `just`.
- **Dependencies:** `github.com/spf13/cobra`, `github.com/spf13/pflag`, `github.com/mattn/go-runewidth`, `golang.org/x/term`.

## Conventions
- **ASCII Tree Rendering:** Uses Unicode box-drawing branches (`├─ `, `╰─ `, `│  `) with single-line aligned descriptions.
- **Display Cell Width:** Measure and truncate by terminal cells with `runewidth.StringWidth` and `runewidth.Truncate`, never `utf8.RuneCountInString` or rune slicing. A CJK ideograph or emoji is one rune in two cells, so rune counts break column alignment and overrun the width limit.
- **Terminal Width Clipping:** Automatically detects terminal width via `golang.org/x/term` / `$COLUMNS` and truncates descriptions with `...` before wrapping.
- **Requested Help Goes to Stdout:** Render help via `c.OutOrStdout()`. Cobra's `c.Print` falls back to **stderr**, which silently breaks `--help | less` and `--help > file`.
- **Deterministic Agent Output:** `AGENT=1` output must be byte-identical across runs. Sort any map before emitting it, and nest caller-supplied maps such as `TechInfo.Metadata` under their own key so they cannot collide with reserved top-level keys.
- **No Optional Variadic Parameters:** Exported functions take explicit parameters. Optional configuration gets a second named function (`Setup` / `SetupWithOptions`), never `opts ...TreeOptions`, which compiles fine while silently discarding every argument past the first.
- **Zero-Value Options:** `TreeOptions.resolve()` supplies the fallback for every unset sizing field, which is what makes `TreeOptions{}` a fully configured value. Add new sizing defaults as constants consumed by `resolve()`; never inline a literal such as `if maxLeftWidth < 20` into the render path.
- **No Exported Mutable Package State:** Defaults live in unexported constants, never in an exported `var`. An exported `var DefaultOptions` let any caller or goroutine reassign the library's defaults process-wide and have it leak into unrelated `Setup` calls.
- **Validate, Do Not Coerce:** Zero means "use the default" for every sizing field, but a negative value or a nil command is a caller mistake. `Validate` reports the former, `SetupWithOptions` reports both, and neither entry point installs anything on failure. Both `Setup` and `SetupWithOptions` return `error`; the string renderers stay total and return `""` for a nil command, because empty output is a well-defined result for a pure function while a failed install is not.
- **Options Match Their Renderer:** `TreeOptions` is what `FormatCommandTree` and `RenderTreeHelp` accept; `AgentOptions` is what `RenderAgentHelp` accepts. `HelpOptions` composes them and exists only for `Setup`, the one caller spanning both modes. Do not add a field to `TreeOptions` that only agent mode reads, or vice versa.
- **Agent Output Stays Untruncated by Default:** `cli-best-practices` requires full untruncated content under `AGENT=1`, so agent mode never consults `$COLUMNS` or the terminal size. `AgentOptions.MaxLineWidth` is opt-in and defaults to unlimited. `TestRenderAgentHelpIsUnclippedByDefault` guards this; do not "fix" it by wiring `GetTerminalWidth` into `RenderAgentHelp`.
- **Renderers Mutate and Are Not Concurrency-Safe:** Reading a command's flags makes cobra merge persistent flag sets, and `updateParentsPflags` writes to the **root and every ancestor**, not the command being rendered. Two goroutines rendering sibling subcommands race inside cobra; `go test -race` reproduces it. `Setup` users are unaffected because cobra executes single-threaded. This is documented in the package comment; do not paper over it with a package mutex, which cannot cover cobra's own `Execute`.
- **Do Not Filter pflag Globals From Help:** cobra folds `pflag.CommandLine` into every root's persistent flags (`command.go:1918`), so a flag registered via package-level `pflag.String` and friends really is accepted by the binary — verified by running it. Help must list it; hiding it would document an interface the CLI does not have. `TestRenderersReportPflagGlobals` locks this. Note this is pflag's global set, never the standard library's `flag` package, which cobra does not consult.
- **Never Read `cmd.Flags()` Directly:** Cobra folds persistent and inherited flags into it only via `mergePersistentFlags`, which runs during `Execute` and inside `LocalFlags`/`InheritedFlags`. Reading `Flags()` drops them on any command not yet executed. Use `applicableFlags`, and call it **before** `cmd.UseLine()`: the merge flips `HasAvailableFlags`, so collecting flags afterwards makes the first render differ from every later one.
- **Clipped Agent Output Must Still Parse:** `clipLine` shortens only the value half of a `key: value` line, never a block opener such as `metadata:`, and returns a line in full when its key leaves no room for a readable value. `MaxLineWidth` is a ceiling, not a guarantee, because a truncated key is unidentifiable and may collide with another key.
- **Hermetic Unit Tests:** All unit tests must remain offline, hermetic, and deterministic.

## Gotchas
- **`cmd.SetOut` masks stream bugs in tests.** Cobra's `getOut` returns the writer installed by `SetOut` for *both* `OutOrStdout()` and `OutOrStderr()`, so a test using `SetOut(buf)` passes no matter which stream the code targets. Tests covering stream choice must replace `os.Stdout` / `os.Stderr` with `os.Pipe()`; see `captureStdio` in `tree_test.go`.
- **A pipe is not a terminal.** `GetTerminalWidth` falls back to `term.GetSize(os.Stdout.Fd())`, which fails under `go test`. Tests asserting the fallback must pin `os.Stdout` to a pipe; see `withNonTerminalStdout` in `tree_test.go`.

## Boundaries
- **Always:** Maintain >= 90% statement code coverage across all Go packages.
- **Always:** Run `just test` and `just vet` before committing changes.
- **Always:** Change a test file whenever a code change alters runtime output, and develop red/green — confirm the new test fails before the fix and passes after.
- **Never:** Publish releases, tags, packages, or deployments without explicit user authorization.
