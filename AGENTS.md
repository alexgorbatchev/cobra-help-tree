---
created_on: 2026-08-24 16:30
last_modified: 2026-08-24 16:30
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
- **Dependencies:** `github.com/spf13/cobra`, `golang.org/x/term`.

## Conventions
- **ASCII Tree Rendering:** Uses Unicode box-drawing branches (`├─ `, `╰─ `, `│  `) with single-line aligned descriptions.
- **Rune Width Alignment:** Computes column width via `utf8.RuneCountInString` to ensure visual alignment in UTF-8 terminals.
- **Terminal Width Clipping:** Automatically detects terminal width via `golang.org/x/term` / `$COLUMNS` and truncates descriptions with `...` before wrapping.
- **Hermetic Unit Tests:** All unit tests must remain offline, hermetic, and deterministic.

## Boundaries
- **Always:** Maintain >= 90% statement code coverage across all Go packages.
- **Always:** Run `just test` and `just vet` before committing changes.
