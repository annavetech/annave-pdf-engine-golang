# Changelog

All notable changes to ANNAVE PDF Engine are documented here.
Versioning follows [Semantic Versioning](https://semver.org/).

---

## [1.0.5] — 2026-08-23

### Added
- `internal/api` test suite: full middleware chain (rate limiting, auth, CORS, size limits, security headers, request ID, logging), the `convert` handler's multipart/urlencoded/raw input paths, and error-stage to HTTP status mapping, including a concurrent rate-limit test run under `-race`. The package had no tests at all; coverage went from 0% to 91.4%.
- Golden-PDF regression test: a markdown fixture, a committed reference PDF, and a byte-compare test in `internal/engine` that reports the diverging byte offset when a parser change alters rendered output instead of a bare "not equal". Regenerate the reference file deliberately with `UPDATE_GOLDEN=1`.

### Fixed
- Rate limiter: the per-IP client map gained an entry on every source address seen and never removed one, so traffic from many rotated addresses grew it without bound. A sweeper now drops entries whose newest request has aged out of the window, and the limiter's state moves into an unexported type so the sweep can be tested directly.
- Renderer: `NewRenderer` registered its six fonts by ranging over a Go map, and map iteration order is randomised on every run. gopdf assigns PDF object numbers in font registration order, so the same document could render to different bytes each time it was converted. Fonts are now registered from a fixed, ordered list, so rendering the same document twice now produces byte-identical output — the golden-PDF test above is what proves it.

### Changed
- Markdown, reStructuredText, and HTML parsing: thirty-five regular expressions were compiled inside function bodies and recompiled on every call, several of them once per line of input. They now compile once at package init instead. Measured: `ParseInline` 6.3× faster with 32.7× less memory, `stripInline` 8.8× faster with 20.6× less memory, and `MdParser.Parse` on a 4,000-item document 8.0× faster, with allocations down from 2,232,765 to 160,042.
- Module path is now `github.com/annavetech/annave-pdf-engine-golang`. The old path, `annave.tech/pdf-engine`, resolves through Go's HTTPS module discovery, which looks for `go-import` metadata at that domain; ANNÁVE TECH never published any, so the module could never actually be fetched under that path. Nothing could have been importing it, which is why this ships as a patch release rather than a major one despite being a module path change. Install the tool directly: `go install github.com/annavetech/annave-pdf-engine-golang/cmd/cli@latest` for the command-line converter, or `.../cmd/server@latest` for the HTTP server.

### Documentation
- Rate limiting, DOCX image extraction, the cobra CLI, inline style spans, and `slog.Warn` logging on render failures were all shipped but still described in the docs as planned. Corrected `README.md`, `docs/ARCHITECTURE.md`, `docs/CONFIGURATION.md`, `docs/CONTRIBUTING.md`, `docs/DEPENDENCIES.md`, `docs/ERROR_CODES.md`, `docs/INTEGRATION.md`, `docs/USE_CASES.md`, and `docs/WHITEPAPER.md` to match the code, fixed a clone URL in the contributing guide that pointed at a repository that does not exist, and reconciled the dependency list with `go.mod`.
- Removed per-document style overrides, the CLI, and rate limiting from the whitepaper's future-directions list — all three are already implemented and documented elsewhere, and the rate-limiting bullet was worded in the present tense, which was incoherent under a "Future directions" heading. Streaming output remains listed because it genuinely isn't implemented. Inline rich text rendering stays listed too, but narrower than before: headings and paragraphs already switch fonts mid-line for bold, italic, and code spans, and what's left is wiring the same span data through for lists and blockquotes and adding it to tables.

---

## [1.0.4] — 2026-05-07

### Fixed
- Renderer: `ImageByHolder` and `ImageHolderByBytes` errors are now logged via `slog.Warn` instead of being silently swallowed, making image rendering failures visible in production logs.

### Added
- Pipeline tests covering all text formats (HTML, JSON, CSV, YAML, XML, RST, TXT) and PNG image input to guard against regressions.

---

## [1.0.3] — 2026-05-07

### Fixed
- Image validation: uploaded binary images (PNG, JPEG, GIF, WebP) carry pixel data in `Data []byte` with no `Src` URL. The validator incorrectly required a non-empty `Src`, rejecting all direct image uploads with `ENGINE_ERR_INVALID_NODE`. The check now accepts a node that has either `Src` or `Data`.
- Image and DOCX parsing via HTTP: `NormalizeInput` strips all bytes below `0x20`, corrupting binary data before it reached the parser and causing `ENGINE_ERR_PARSE_FAILED` for every image and DOCX upload. The pipeline now skips normalization for binary formats (image, DOCX) detected by format hint or magic-byte probe.

---

## [1.0.2] — 2026-05-07

### Fixed
- Table renderer: `SetFillColor` and `SetTextColor` share gopdf's non-stroking color state. After `Cell()` wrote text, the fill color was left as the text color, so every column after the first rendered with a solid dark background. Fill color and text color are now restored before each rectangle and cell draw.
- Table layout: cell text that exceeds column width was silently clipped. Rows now have dynamic height computed from wrapped content; the renderer wraps and stacks lines within each cell.
- Table column width allocation: proportional scaling could compress a column below the width of its longest single word, forcing a mid-word line break. The allocator now locks each column to a word-safe minimum before distributing remaining page width.

### Added
- GitHub Actions CI workflow (`.github/workflows/ci.yml`): runs `go vet`, `go build`, and `go test` on every push and pull request to `main`.

---

## [1.0.1] — 2026-05-07

### Changed
- Renamed `internal/parsers` package to `internal/parser` (singular, consistent with Go stdlib conventions)
- HTML sanitization is now skipped for explicitly typed non-HTML formats; fixes incorrect sanitization of Markdown containing HTML fragments such as `<iframe>` or `<embed>` in paragraph text

### Added
- SPDX file headers across all source files
- Apache 2.0 `LICENSE` and `NOTICE` with third-party attributions

---

## [1.0.0] — 2026-05-06

### Added
- Initial release
- Six-stage document-to-PDF pipeline: normalise → parse → validate → layout → paginate → render
- Nine input format parsers: Markdown (GFM), plain text, JSON, HTML, CSV, YAML, XML, reStructuredText, Jupyter Notebook
- A4 PDF output at 96 DPI with embedded Inter and JetBrains Mono fonts
- Accurate per-glyph text measurement using `golang.org/x/image/font/opentype`
- Smart page breaking: keep-with-next headings, orphan protection, row-split tables, line-split paragraphs and code blocks
- HTTP API: `POST /convert` (multipart, JSON body, raw text), `GET /health`
- CORS and request size limit middleware
- Debug CLI tool at `cmd/debug`