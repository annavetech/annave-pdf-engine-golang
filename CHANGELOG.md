# Changelog

All notable changes to ANNÁVE PDF Engine are documented here.
Versioning follows [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

### Changed
- **Breaking:** `Engine.Convert` now takes a `context.Context` and `[]byte` instead of a bare `string`, and drops its per-call `Option` parameter: `func (e *Engine) Convert(ctx context.Context, data []byte, f Format) ([]byte, error)`. `New` now takes the configuration options instead: `func New(opts ...Option) (*Engine, error)`. `New` is fallible (a caller-supplied config file can fail to load or fail validation), so it returns an error instead of the previous panic-free-but-unconfigurable constructor. Every call site (the CLI, the tests, the documentation examples) is updated to the new shape.
- Style and limits are now genuinely per-`Engine`: `pdfengine.New(pdfengine.WithStyleFile(path))` and `pdfengine.New(pdfengine.WithLimitsFile(path))` replace the embedded `config/style.yaml`/`config/limits.yaml` for that `Engine` only, without a rebuild. Two `Engine` values built with different options hold independent configuration in the same process. The library itself never scans a filesystem or reads an environment variable for this: a path is read only when one of these options names it explicitly. `annave pdf convert` gains matching `--style-file`/`--limits-file` flags.
- `README.md`, `docs/ARCHITECTURE.md`, `docs/CONFIGURATION.md`, `docs/CONTRIBUTING.md`, `docs/ERROR_CODES.md`, `docs/USE_CASES.md`, and `docs/WHITEPAPER.md` are updated for the new `Convert`/`New` signatures and to describe the real architecture (see "Removed" below) instead of the ports-and-adapters framing.
- **Breaking (CLI):** `annave pdf convert --format <value>` now rejects a format it doesn't recognise instead of silently falling back to auto-detection. A typo in `--format` used to produce a document parsed under a guess nobody asked for; it now fails with a clear error naming the rejected format.

### Fixed
- `internal/parser.Registry.Parse` silently fell back to auto-detection when asked for a format string it had no parser registered for, instead of reporting the mistake. It now returns an `*UnsupportedFormatError` naming the offending format, which `internal/engine.Pipeline.Run` translates to `ENGINE_ERR_UNSUPPORTED_FORMAT`, a code that has existed in `config/messages.yaml` since before this fix but never had a producer. `FormatAuto` is unaffected: asking for auto-detection always auto-detects. The public `pdfengine.Convert` API cannot trigger this error, since its `Format` type is a closed, validated set that normalises anything unrecognised to auto-detection before reaching this check; today the only caller that can reach it is the CLI's `--format` flag.
- CSV input containing non-ASCII characters (accented Latin, Cyrillic, CJK, anything outside ASCII) came out corrupted (`café` became `cafÃ©`). The hand-rolled field-builder in `internal/parser/csv.go` built each field with `field += string(ch)`, where `ch` was a single byte; Go's `string(byte)` conversion treats that byte as a Unicode code point and re-encodes it as UTF-8, mangling any byte that was part of a multi-byte sequence. `CsvParser` is rewritten on top of the standard library's `encoding/csv`, which never does a byte-to-rune conversion of raw input.
- A quoted CSV cell containing an embedded newline (legal under RFC 4180, and what every spreadsheet export produces for a multi-line cell) was split across two malformed rows, because the previous hand-rolled scanner split the whole input on newlines before any quote handling ran. `encoding/csv.Reader` (RFC 4180-correct, including quoted multi-line fields) replaces it entirely; `docs/DEPENDENCIES.md`'s claim that the CSV parser used `encoding/csv` is now true rather than aspirational.
- `internal/parser/docx.go`'s `Parse` and `CanParse` each independently opened and scanned the same `.docx` zip archive from scratch, and every entry lookup inside `Parse` (`document.xml`, `numbering.xml`, the rels file, every embedded image) was its own linear scan over the archive's file list, an O(entries × images) cost as image count grew. Both now go through a single `docxArchive` type that indexes the archive once into a `map[string]*zip.File`, so every lookup is O(1).
- `.docx` parsing had no limit on how far a compressed zip entry could expand when read, so a small, pathological archive could exhaust memory (a zip bomb). Every entry read from a `.docx` archive is now checked against a 50 MB per-entry uncompressed-size cap and a 150 MB cumulative cap across the whole archive (`internal/parser/docx.go`'s `maxDocxEntryBytes`/`maxDocxTotalBytes`), both named, documented constants. Exceeding either returns a clear error; nothing is silently truncated or silently dropped.

### Removed
- `internal/port`, an unused package of interface definitions with zero importers, is deleted. Its "hexagonal architecture" framing, and every echo of it in `README.md`, `docs/ARCHITECTURE.md`, and `docs/WHITEPAPER.md`, is replaced with a description of what the code actually does: the public `pdfengine` package and the `annave` CLI both call directly into the internal pipeline.
- The `init()` function that loaded `config/limits.yaml`/`config/messages.yaml` into process-wide variables and called `log.Fatalf` on a parse failure (a library must not exit its host process) is gone. Configuration is now loaded explicitly per `Engine` by `LoadConfig`, which returns an ordinary error.
- The hardcoded `DocumentStyle` literal in `internal/engine/style.go` (a duplicate of `config/style.yaml` that the embedded file was never actually loaded to produce) is gone; the embedded `config/style.yaml` is now parsed and validated at `Engine` construction, so it is the file actually in effect.
- `ENGINE_ERR_INTERNAL` is deleted from `config/messages.yaml` and `docs/ERROR_CODES.md`. Nothing in the codebase ever produced it, and no producer was invented to justify keeping it.

### Removed (HTTP layer and its deployment)
- The HTTP server and its deployment mechanics are gone: `internal/api` (handler, middleware, auth, CORS, rate limiting), `cmd/server`, `ui/` (the browser UI), `config/server.yaml`, `Dockerfile`, `docker-compose.yml`, and `railway.toml`. The engine is now a library and a CLI only. The `pdf serve` CLI subcommand is removed along with it.
- `ENGINE_ERR_UNAUTHORIZED` and `ENGINE_ERR_RATE_LIMITED` (both HTTP-only, unreachable without the server) and `ENGINE_ERR_EMPTY_INPUT` (only ever produced by the now-deleted HTTP handler, never by the engine or CLI) are removed from `config/messages.yaml` and `docs/ERROR_CODES.md`.
- `schema/error.v1.schema.json` is removed. It described the JSON body the HTTP handler wrote on error; nothing produces that body anymore. `schema/document.v1.schema.json` (the JSON input format schema) is unaffected and stays.
- `docs/INTEGRATION.md` is removed; it was an HTTP integration guide for five languages. Its one still-relevant piece, direct Go library use, already lives in `README.md`'s "Use as a library" section.

---

## [1.2.0] - 2026-08-23

### Added
- Releases now publish binaries for macOS and Linux on amd64 and arm64 with checksums, and a Homebrew cask in the `annavetech/annave` tap. Install with `brew tap annavetech/annave && brew install annave-pdf-engine`.

### Fixed
- Security: `golang.org/x/image` 0.23.0 → 0.45.0 and `golang.org/x/net` 0.33.0 → 0.58.0. Both were reachable from the code paths that handle uploaded files: image decoding, HTML parsing, and HTML sanitising.
- The Docker image now builds against a Go version that satisfies the module's floor.
- CLI output, log lines, documentation, and schema titles now use the correct ANNÁVE spelling. Environment variable names and `ENGINE_ERR_*` codes are unchanged.

### Changed
- Minimum Go version is now 1.25, raised from 1.23. The `golang.org/x/image` releases that fix the vulnerabilities above require it.
- Release binaries are built with the current Go toolchain, so they carry current standard-library fixes.

### Documentation
- The Go example in the integration guide uses the public `pdfengine` package.
- `README.md` install instructions corrected.

---

## [1.1.0] - 2026-08-23

### Added
- Public Go API at the module root, package `pdfengine`. Every package that did real work lived under `internal/`, so the engine had no importable surface at all outside its own binaries. `pdfengine.New().Convert(text, pdfengine.FormatMarkdown)` delegates inward to the same pipeline the server and CLI use, with its own `Format`, `Style`, `Option` and `Error` types; nothing from `internal/engine` or `internal/parser` is re-exported, so internal changes never force a breaking change on library callers.

---

## [1.0.5] - 2026-08-23

### Added
- `internal/api` test suite: full middleware chain (rate limiting, auth, CORS, size limits, security headers, request ID, logging), the `convert` handler's multipart/urlencoded/raw input paths, and error-stage to HTTP status mapping, including a concurrent rate-limit test run under `-race`. The package had no tests at all; coverage went from 0% to 91.4%.
- Golden-PDF regression test: a markdown fixture, a committed reference PDF, and a byte-compare test in `internal/engine` that reports the diverging byte offset when a parser change alters rendered output instead of a bare "not equal". Regenerate the reference file deliberately with `UPDATE_GOLDEN=1`.

### Fixed
- Rate limiter: the per-IP client map gained an entry on every source address seen and never removed one, so traffic from many rotated addresses grew it without bound. A sweeper now drops entries whose newest request has aged out of the window, and the limiter's state moves into an unexported type so the sweep can be tested directly.
- Renderer: `NewRenderer` registered its six fonts by ranging over a Go map, and map iteration order is randomised on every run. gopdf assigns PDF object numbers in font registration order, so the same document could render to different bytes each time it was converted. Fonts are now registered from a fixed, ordered list, so rendering the same document twice now produces byte-identical output: the golden-PDF test above is what proves it.

### Changed
- Markdown, reStructuredText, and HTML parsing: thirty-five regular expressions were compiled inside function bodies and recompiled on every call, several of them once per line of input. They now compile once at package init instead. Measured: `ParseInline` 6.3× faster with 32.7× less memory, `stripInline` 8.8× faster with 20.6× less memory, and `MdParser.Parse` on a 4,000-item document 8.0× faster, with allocations down from 2,232,765 to 160,042.
- Module path is now `github.com/annavetech/annave-pdf-engine-golang`. The old path, `annave.tech/pdf-engine`, resolves through Go's HTTPS module discovery, which looks for `go-import` metadata at that domain; ANNÁVE TECH never published any, so the module could never actually be fetched under that path. Nothing could have been importing it, which is why this ships as a patch release rather than a major one despite being a module path change. Install the tool directly: `go install github.com/annavetech/annave-pdf-engine-golang/cmd/cli@latest` for the command-line converter, or `.../cmd/server@latest` for the HTTP server.

### Documentation
- Rate limiting, DOCX image extraction, the cobra CLI, inline style spans, and `slog.Warn` logging on render failures were all shipped but still described in the docs as planned. Corrected `README.md`, `docs/ARCHITECTURE.md`, `docs/CONFIGURATION.md`, `docs/CONTRIBUTING.md`, `docs/DEPENDENCIES.md`, `docs/ERROR_CODES.md`, `docs/INTEGRATION.md`, `docs/USE_CASES.md`, and `docs/WHITEPAPER.md` to match the code, fixed a clone URL in the contributing guide that pointed at a repository that does not exist, and reconciled the dependency list with `go.mod`.
- Removed per-document style overrides, the CLI, and rate limiting from the whitepaper's future-directions list: all three are already implemented and documented elsewhere, and the rate-limiting bullet was worded in the present tense, which was incoherent under a "Future directions" heading. Streaming output remains listed because it genuinely isn't implemented. Inline rich text rendering stays listed too, but narrower than before: headings and paragraphs already switch fonts mid-line for bold, italic, and code spans, and what's left is wiring the same span data through for lists and blockquotes and adding it to tables.

---

## [1.0.4] - 2026-05-07

### Fixed
- Renderer: `ImageByHolder` and `ImageHolderByBytes` errors are now logged via `slog.Warn` instead of being silently swallowed, making image rendering failures visible in production logs.

### Added
- Pipeline tests covering all text formats (HTML, JSON, CSV, YAML, XML, RST, TXT) and PNG image input to guard against regressions.

---

## [1.0.3] - 2026-05-07

### Fixed
- Image validation: uploaded binary images (PNG, JPEG, GIF, WebP) carry pixel data in `Data []byte` with no `Src` URL. The validator incorrectly required a non-empty `Src`, rejecting all direct image uploads with `ENGINE_ERR_INVALID_NODE`. The check now accepts a node that has either `Src` or `Data`.
- Image and DOCX parsing via HTTP: `NormalizeInput` strips all bytes below `0x20`, corrupting binary data before it reached the parser and causing `ENGINE_ERR_PARSE_FAILED` for every image and DOCX upload. The pipeline now skips normalization for binary formats (image, DOCX) detected by format hint or magic-byte probe.

---

## [1.0.2] - 2026-05-07

### Fixed
- Table renderer: `SetFillColor` and `SetTextColor` share gopdf's non-stroking color state. After `Cell()` wrote text, the fill color was left as the text color, so every column after the first rendered with a solid dark background. Fill color and text color are now restored before each rectangle and cell draw.
- Table layout: cell text that exceeds column width was silently clipped. Rows now have dynamic height computed from wrapped content; the renderer wraps and stacks lines within each cell.
- Table column width allocation: proportional scaling could compress a column below the width of its longest single word, forcing a mid-word line break. The allocator now locks each column to a word-safe minimum before distributing remaining page width.

### Added
- GitHub Actions CI workflow (`.github/workflows/ci.yml`): runs `go vet`, `go build`, and `go test` on every push and pull request to `main`.

---

## [1.0.1] - 2026-05-07

### Changed
- Renamed `internal/parsers` package to `internal/parser` (singular, consistent with Go stdlib conventions)
- HTML sanitization is now skipped for explicitly typed non-HTML formats; fixes incorrect sanitization of Markdown containing HTML fragments such as `<iframe>` or `<embed>` in paragraph text

### Added
- SPDX file headers across all source files
- Apache 2.0 `LICENSE` and `NOTICE` with third-party attributions

---

## [1.0.0] - 2026-05-06

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