<!--
title:       ANNAVE PDF Engine — Contributing Guide
description: How to add a parser, run tests, submit a pull request,
             and what code style is expected.
author:      Anna Veretennykova
website:     www.annave.tech
version:     1.0.4
created:     2026-05-06
updated:     2026-08-23
-->

# Contributing

## Setup

```bash
git clone https://github.com/annavetechhq/604-annave-pdf-engine-oss-api-dev.git
cd 604-annave-pdf-engine-oss-api-dev

# Build
go build ./cmd/server

# Run tests
go test ./...

# Run the server locally (no auth)
go run cmd/server/main.go
```

Go 1.23 or later is required.

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

---

## Code style

- SPDX header on every `.go` file:
  ```go
  // Copyright 2026 Anna Veretennykova
  //
  // SPDX-License-Identifier: Apache-2.0
  ```
- Comments only when the reason is non-obvious — not what the code does, but why it does it that way.
- No `_test.go` file uses mocks for the database, filesystem, or HTTP. The pipeline tests call `Pipeline.Run` directly. The HTTP tests use `httptest.NewRecorder`. No fakes for the PDF renderer — tests assert on the error return, not the PDF content.
- `gofmt` is the formatter. No additional linters are required, but `go vet ./...` must pass.
- Error codes must be added to `config/messages.yaml` before they are used in Go code. Do not hardcode message strings in `.go` files.

---

## How to add an input format

See `docs/ARCHITECTURE.md` for the full walkthrough. Short version:

1. Create `internal/parser/yourformat.go` with a struct implementing `DocumentParser` (two methods: `CanParse`, `Parse`).
2. Add the format constant and extension mappings to `internal/parser/registry.go`.
3. Register the parser in `NewRegistry()` — binary parsers (magic-byte checks) before text parsers in the `ordered` slice.
4. Write a test in `internal/parser/yourformat_test.go`. The test fixture should be a real document from the ANNAVE PDF Engine documentation — not lorem ipsum. This doubles as self-documenting content that people will not delete.
5. Update the `ENGINE_ERR_UNSUPPORTED_FORMAT` message in `config/messages.yaml` to include the new format name.

The HTTP handler, pipeline, and renderer require no changes.

---

## How to add a pipeline stage

The six stages are fixed at the architectural level. Adding a seventh stage requires:

1. Write the stage function in `internal/engine/`.
2. Add a new `EngineStage` constant in `internal/engine/errors.go` and add it to the `stage` enum in `schema/error.v1.schema.json`.
3. Call it in `Pipeline.Run` in `internal/engine/pipeline.go` at the correct position.
4. Write tests in `internal/engine/` that exercise the new stage in isolation and as part of the full pipeline.

---

## Submitting a pull request

1. Fork and create a branch from `main`.
2. Make your changes. `go build ./...` and `go test ./...` must pass.
3. Add a test for any new behaviour. For parsers, use a real fixture from the engine documentation.
4. Open a PR against `main`. Describe what the change does and why, not just what files changed.
5. Do not bump the version in `internal/engine/config.go` — that is done at release time.

---

## Versioning

The engine follows [Semantic Versioning](https://semver.org). The version is set in `internal/engine/config.go`:

```go
const EngineVersion = "1.0.4"
```

- Patch (1.0.x): bug fixes, no API or config key changes
- Minor (1.x.0): new parsers, new config keys (all backward compatible)
- Major (x.0.0): breaking changes to the HTTP API, error code scheme, or AST structure

---

## Reporting bugs

Open an issue at the project repository. Include:
- The input format
- A minimal reproduction (smallest input that triggers the bug)
- The `X-Request-Id` header from the failing response
- The full JSON error body
