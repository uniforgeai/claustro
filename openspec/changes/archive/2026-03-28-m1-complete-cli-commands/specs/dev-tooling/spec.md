## ADDED Requirements

### Requirement: Makefile for local development

The project SHALL provide a `Makefile` at the repository root with targets for common development operations.

#### Scenario: Build the binary

- **WHEN** the developer runs `make build`
- **THEN** the binary is compiled to `bin/claustro` using `go build -o bin/claustro ./cmd/claustro`

#### Scenario: Run the CLI directly

- **WHEN** the developer runs `make run ARGS="up --name foo"`
- **THEN** the CLI is executed via `go run ./cmd/claustro` with the provided arguments

#### Scenario: Run unit tests

- **WHEN** the developer runs `make test`
- **THEN** `go test ./...` is executed and results are printed

#### Scenario: Run linter

- **WHEN** the developer runs `make lint`
- **THEN** `golangci-lint run` is executed against the codebase

#### Scenario: Clean build artifacts

- **WHEN** the developer runs `make clean`
- **THEN** the `bin/` directory is removed
