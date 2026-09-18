# Minimum statement coverage the package must keep, enforced by `just coverage`
min_coverage := "90"

# Default recipe: list available tasks
default:
    @just --list

# Run the example CLI in human mode, e.g. `just run user create --help`
run *args:
    go run ./examples/demo {{args}}

# Run the example CLI in agent mode, e.g. `just run-ai user create --help`
run-ai *args:
    AGENT=1 go run ./examples/demo {{args}}

# Run all unit tests
test:
    go test -v -cover ./...

# Run the tests and fail when statement coverage drops below the floor
coverage:
    #!/usr/bin/env bash
    set -euo pipefail
    # Every test runs, including the example CLI's, but the floor is a statement
    # about the library: -coverpkg measures the non-main packages only, so an
    # example program can never dilute or inflate the number.
    # A quadrupled brace is just's escape for the literal brace pair a Go
    # template needs; just interpolates inside shebang recipes too.
    packages="$(go list -f '{{{{if not (eq .Name "main")}}{{{{.ImportPath}}{{{{end}}' ./... | paste -sd, -)"
    go test -race -coverpkg="${packages}" -coverprofile=coverage.out ./...
    total="$(go tool cover -func=coverage.out | awk '/^total:/ { print substr($3, 1, length($3) - 1) }')"
    if awk "BEGIN { exit ({{min_coverage}} <= ${total}) ? 0 : 1 }"; then
        echo "statement coverage ${total}% meets the {{min_coverage}}% floor"
    else
        echo "statement coverage ${total}% is below the {{min_coverage}}% floor" >&2
        exit 1
    fi

# Run every static check, the tests, and the coverage floor
check:
    just lint
    just coverage

# Format Go source code
fmt:
    go fmt ./...

# Check formatting, vet, and module hygiene
lint:
    #!/usr/bin/env bash
    set -euo pipefail
    unformatted="$(gofmt -l .)"
    if [ -n "${unformatted}" ]; then
        echo "gofmt is not clean, run 'just fmt':" >&2
        echo "${unformatted}" >&2
        exit 1
    fi
    go vet ./...
    go mod tidy -diff
