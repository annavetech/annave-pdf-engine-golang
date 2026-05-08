# Changelog

All notable changes to ANNAVE PDF Engine are documented here.
Versioning follows [Semantic Versioning](https://semver.org/).

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