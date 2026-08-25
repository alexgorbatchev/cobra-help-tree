# Default recipe: list available tasks
default:
    @just --list

# Run all unit tests
test:
    go test -v -cover ./...

# Run static analysis and tests
check:
    go vet ./...
    go test -v ./...

# Format Go source code
fmt:
    go fmt ./...

# Run linter
lint:
    go vet ./...
