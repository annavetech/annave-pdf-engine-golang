<!--
title:       ANNÁVE PDF Engine — Dependencies
description: Every direct and indirect dependency, exact pinned version,
             what each does, and what breaks if you change the version.
author:      Anna Veretennykova
website:     www.annave.tech
version:     1.2.0
created:     2026-05-06
updated:     2026-08-23
-->

# Dependencies

Go version: **1.25** (minimum). The engine uses `log/slog` (added in 1.21) and pattern-based HTTP routing in `net/http` (added in 1.22); the 1.25 floor itself comes from `golang.org/x/image` v0.45.0, which is required to clear a reachable memory-allocation vulnerability (GO-2026-6222) and declares `go 1.25.0`.

All dependencies are listed in `go.mod`. Run `go mod verify` to confirm checksums match `go.sum`.

---

## Direct dependencies

### `github.com/signintech/gopdf v0.36.0`

**Must be exactly v0.36.0 or higher within the v0 series.**

PDF generation. The engine calls `pdf.GetBytesPdfReturnErr()` to obtain the PDF bytes as a Go error-returning function. This API was introduced in v0.36.0; earlier versions provide only `GetBytesPdf()` which panics on error.

| Version range | Result |
|---|---|
| < v0.36.0 | Compile error — `GetBytesPdfReturnErr` undefined |
| v0.36.0 | Confirmed working |
| v0.37+ | Unknown — compile succeeds; check PDF rendering output against the golden tests |

Upgrade path: run `go test ./internal/engine/...` after any gopdf upgrade. The engine test suite exercises rendering across all node types. A rendering regression will typically appear as a misplaced element or a panic.

---

### `github.com/microcosm-cc/bluemonday v1.0.27`

HTML sanitisation. Applied to input only when the format is `html` (explicit or auto-detected). Strips script tags, iframes, event handlers, and disallowed attributes before the HTML parser runs.

| Version range | Result |
|---|---|
| < v1.0.27 | Unknown — API is stable; older versions should work but are not tested |
| v1.0.27 | Confirmed working |
| v1.1+ | Unknown — check if the `UGCPolicy()` API is unchanged |

**Do not remove this dependency.** Without sanitisation, the HTML parser would expose the Go process to malformed HTML that triggers panics in the golang.org/x/net/html tokeniser.

---

### `golang.org/x/image v0.23.0`

Image format support for WebP decoding. The standard library `image` package handles PNG, JPEG, and GIF; WebP requires this extended package.

Blank-imported in `internal/parser/image.go`:
```go
import _ "golang.org/x/image/webp"
```

| Version range | Result |
|---|---|
| Any v0.x | Should work — the `webp` package has a stable decode API |
| v0.23.0 | Confirmed working |

---

### `golang.org/x/net v0.33.0`

HTML parsing, used by `bluemonday` (indirect requirement). Also provides the `html` tokeniser that the HTML parser adapter uses directly.

| Version range | Result |
|---|---|
| Any v0.x | Should work |
| v0.33.0 | Confirmed working |

---

### `github.com/spf13/cobra v1.10.2`

Used directly by `cmd/cli` for the CLI command tree (`pdf`, `pdf convert`, `pdf serve`). The HTTP server (`cmd/server`) and the core engine (`internal/engine`) do not depend on cobra.

| Version range | Result |
|---|---|
| Any v1.x | Should work — cobra's command API is stable |
| v1.10.2 | Confirmed working |

---

## Indirect dependencies

These are required by direct dependencies but not imported directly by the engine.

| Package | Version | Required by |
|---|---|---|
| `github.com/aymerick/douceur` | v0.2.0 | bluemonday — CSS parsing for sanitisation |
| `github.com/gorilla/css` | v1.0.1 | douceur — CSS tokeniser |
| `github.com/inconshreveable/mousetrap` | v1.1.0 | cobra — Windows "don't double-click" detection |
| `github.com/phpdave11/gofpdi` | v1.0.14-0.20211212211723-1f10f9844311 | gopdf — PDF import |
| `github.com/pkg/errors` | v0.8.1 | gopdf — error wrapping |
| `github.com/spf13/pflag` | v1.0.9 | cobra — POSIX/GNU-style flag parsing |
| `golang.org/x/text` | v0.21.0 | golang.org/x/net — text encoding |
| `gopkg.in/yaml.v3` | v3.0.1 | engine config loading |

---

## Standard library usage

The following standard library packages are used in ways that require the minimum Go version:

| Package | Minimum Go version | Usage |
|---|---|---|
| `log/slog` | 1.21 | Structured logging throughout |
| `net/http` (pattern routing) | 1.22 | `"POST /convert"` method-prefixed route registration |
| `archive/zip` | 1.0 | DOCX parsing (DOCX is a ZIP archive) |
| `encoding/xml` | 1.0 | DOCX body XML parsing |
| `image` | 1.0 | PNG/JPEG/GIF decode config for dimensions |
| `crypto/rand` | 1.0 | Request ID generation |

---

## Adding a dependency

Before adding a new dependency:

1. Check if the standard library already provides what you need. The DOCX parser uses `archive/zip` + `encoding/xml` with zero external packages; the CSV parser uses `encoding/csv`. Most common tasks are covered.

2. If an external package is needed, prefer packages with no transitive dependencies or very few. The current dependency tree is intentionally shallow.

3. Run `go mod tidy` after adding and verify `go.sum` is updated.

4. Add an entry to this file with the version, what it does, and what breaks if it changes.
