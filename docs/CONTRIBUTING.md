<!--
title:       ANNÁVE PDF Engine: Contributing Guide
description: How to add a parser, run tests, submit a pull request,
             and what code style is expected.
author:      Anna Veretennykova
website:     www.annave.tech
version:     1.2.0
created:     2026-05-06
updated:     2026-08-23
-->

# Contributing

## Setup

```bash
git clone https://github.com/annavetech/pdfengine.git
cd pdfengine

# Build
go build ./cmd/cli

# Run tests
go test ./...

# Convert a document locally
go run ./cmd/cli pdf convert README.md -o output.pdf
```

Go 1.26 or later is required.

---

## Running tests

```bash
# All tests
go test ./...

# Single package with verbose output
go test -v ./internal/engine/...

# Single test function
go test -v -run TestPipeline_Run_Markdown ./internal/engine/...

# Fuzz the Markdown parser for 30 seconds
go test -fuzz=FuzzMdParser -fuzztime=30s ./internal/parser/...

# Race detector
go test -race ./...
```

### Golden-PDF regression test

`internal/engine/golden_test.go` renders `internal/engine/testdata/golden.md`
and byte-compares the result against the committed
`internal/engine/testdata/golden.pdf`. This is what proves a refactor left
rendered output unchanged: a pure change (regex hoisting, loop restructuring)
must produce identical bytes; if it does not, something was transcribed
wrongly.

If a change is meant to alter rendered output, regenerate the golden file
and commit it deliberately:

```bash
UPDATE_GOLDEN=1 go test ./internal/engine/ -run TestPipeline_Run_GoldenPDFMatchesFixture
git diff --stat internal/engine/testdata/golden.pdf   # confirm the change is expected
```

The `UPDATE_GOLDEN` env var guard means this never happens as a side effect
of a normal `go test ./...`.

---

## Code style

- SPDX header on every `.go` file:
  ```go
  // Copyright 2026 Anna Veretennykova
  //
  // SPDX-License-Identifier: Apache-2.0
  ```
- Comments only when the reason is non-obvious: not what the code does, but why it does it that way.
- No `_test.go` file uses mocks for the database or filesystem. The pipeline tests call `Pipeline.Run` directly. No fakes for the PDF renderer: tests assert on the error return, not the PDF content.
- CI requires `gofmt`, `go vet ./...`, `golangci-lint` v2.14.0, `govulncheck` v1.8.0, and `go test ./... -race` to all pass. Run the linter locally with the same pinned version CI uses: `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...`. Run the vuln check with `go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...`.
- Error codes must be added to `config/messages.yaml` before they are used in Go code. Do not hardcode message strings in `.go` files.

---

## How to add an input format

See `docs/ARCHITECTURE.md` for the full walkthrough. Short version:

1. Create `internal/parser/yourformat.go` with a struct implementing `parser.Parser` (two methods: `CanParse`, `Parse`).
2. Add the format constant and extension mappings to `internal/parser/registry.go`.
3. Register the parser in `NewRegistry()`: binary parsers (magic-byte checks) before text parsers in the `ordered` slice.
4. Write a test in `internal/parser/yourformat_test.go`. The test fixture should be a real document from the ANNÁVE PDF Engine documentation, not lorem ipsum. This doubles as self-documenting content that people will not delete.
5. Update the `ENGINE_ERR_UNSUPPORTED_FORMAT` message in `config/messages.yaml` to include the new format name.

The pipeline and renderer require no changes.

---

## How to add a pipeline stage

The six stages are fixed at the architectural level. Adding a seventh stage requires:

1. Write the stage function in `internal/engine/`.
2. Add a new `EngineStage` constant in `internal/engine/errors.go`.
3. Call it in `Pipeline.Run` in `internal/engine/pipeline.go` at the correct position.
4. Write tests in `internal/engine/` that exercise the new stage in isolation and as part of the full pipeline.

---

## Submitting a pull request

1. Fork and create a branch from `main`.
2. Make your changes. `go build ./...` and `go test ./...` must pass.
3. Add a test for any new behaviour. For parsers, use a real fixture from the engine documentation.
4. Open a PR against `main`. Describe what the change does and why, not just what files changed.
5. Do not bump the version in `internal/engine/config.go`; that is done at release time.

---

## Versioning

The engine follows [Semantic Versioning](https://semver.org). The version is set in `internal/engine/config.go`:

```go
const EngineVersion = "1.2.0"
```

- Patch (1.0.x): bug fixes, no API or config key changes
- Minor (1.x.0): new parsers, new config keys (all backward compatible)
- Major (x.0.0): breaking changes to the public Go API, error code scheme, or AST structure

---

## Reporting bugs

Open an issue at the project repository. Include:
- The input format
- A minimal reproduction (smallest input that triggers the bug)
- The full error returned by `Convert` (`Code`, `Stage`, and `Message`)
