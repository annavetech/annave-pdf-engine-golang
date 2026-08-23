<!--
title:       ANNAVE PDF Engine — Technical White Paper
description: The problem, the design decisions, the tradeoffs, and the
             performance characteristics of the ANNAVE PDF Engine v1.0.
author:      Anna Veretennykova
website:     www.annave.tech
version:     1.0.5
created:     2026-05-06
updated:     2026-08-23
-->

# Technical White Paper

## Problem statement

Generating PDFs from structured documents is a common requirement across developer tools, reporting systems, and content pipelines. The existing solutions fall into two broad categories:

**Browser-based rendering** (Puppeteer, wkhtmltopdf, WeasyPrint): Launch a headless browser or WebKit engine, render HTML, and print to PDF. These produce visually accurate output because they use real CSS layout engines. The cost is significant: a headless Chrome instance consumes 150–300 MB of RAM at idle, takes 2–5 seconds to cold-start, and requires a separate process per request or a managed pool. Operating one at scale requires infrastructure that is disproportionate to the task.

**Document library approach** (iTextPDF, Apache PDFBox, reportlab): Imperative APIs where you position every element by hand. These are powerful but require substantial code to convert a document into a PDF; there is no concept of "document to PDF" — only "draw text at this position." Integrating a new input format (say, DOCX) requires implementing the full conversion chain in the library's API.

Neither category is well-suited to the use case of "accept a document in any common format, produce a clean PDF, do it fast."

---

## Design goals

1. **Single binary, no external processes.** The engine must run as a single Go binary with no runtime dependencies — no browser, no Java, no Python. This keeps deployment simple: copy the binary, run it.

2. **Zero-dependency parsing.** The DOCX parser uses only `archive/zip` and `encoding/xml` from the standard library. The CSV, YAML, XML, and JSON parsers use only `encoding/csv`, `gopkg.in/yaml.v3`, and `encoding/xml`. The HTML parser uses `golang.org/x/net/html`. No external document processing libraries.

3. **Consistent output.** Every document type produces output styled by the same `config/style.yaml`. A Markdown README and a DOCX specification look identical side by side. This is a deliberate constraint — the engine is not a layout fidelity tool, it is a consistent rendering tool.

4. **Operator-configurable, developer-transparent.** Operators adjust limits and styles by editing YAML files and rebuilding. Developers adding a new parser implement two methods and register the parser in one place. Nothing else changes.

5. **Predictable resource use.** Memory consumption is bounded by the configured limits: max file size, max input chars, max nodes, max pages. A request that produces 100 pages holds those layout boxes in memory until rendering completes, then frees them. There is no unbounded growth path.

---

## Architecture summary

The engine is a hexagonal (ports and adapters) architecture. The domain core is a six-stage pipeline:

```
Normalise → Parse → Validate → Layout → Paginate → Render
```

Each stage is a pure function over the output of the previous stage. The pipeline itself has no knowledge of HTTP, file I/O, or the PDF renderer API. This makes each stage independently testable and replaceable.

The parser registry uses a two-path dispatch: explicit format (O(1) map lookup) or auto-detection (ordered probe, typically resolves in 1–3 checks for common formats). Binary format parsers check magic bytes and are listed first; text format parsers use structural heuristics.

The AST (`internal/ast`) is the data contract between pipeline stages. It has no dependencies on any other internal package. Block node types: document, heading, paragraph, list, table, code block, blockquote, horizontal rule, image. Inline span types: text, bold, italic, bold-italic, code, strikethrough, link.

---

## Tradeoffs

### Layout engine is custom, not CSS

The layout engine computes positions in pixels using font metrics from gopdf and the configured style values. It does not implement CSS. This means:

- No support for `margin: auto` centering, flexbox, grid, or float
- No inheritance of styles from parent elements
- Tables lay out with equal column widths regardless of content

The benefit is that the layout engine is 300 lines of Go with no external dependencies, is fully deterministic, and produces identical output across platforms and Go versions.

For use cases that require CSS layout fidelity (marketing pages, complex reports with precise column layout), a browser-based tool is more appropriate.

### HTML sanitisation before parsing

When the input format is HTML, `bluemonday` strips all disallowed tags and attributes before the HTML parser runs. This is a deliberate security decision: the engine runs as an HTTP server and must not be a vector for stored XSS (if the PDF is later viewed in a browser-based viewer) or for triggering panics in the HTML parser via malformed input.

The cost is that some valid HTML — particularly anything relying on CSS classes, inline styles, or JavaScript — is stripped. For internal, trusted HTML generation pipelines, this can be relaxed by modifying the sanitisation policy in `internal/engine/sanitizer.go`.

### No font subsetting

Embedded fonts (Inter, JetBrains Mono) are included in their entirety in every PDF output. A typical PDF output with both fonts is approximately 300–500 KB before any content. Font subsetting (embedding only the glyphs used in the document) would reduce this, but gopdf v0.36 does not provide subsetting. The tradeoff is larger files in exchange for zero missing-glyph risk.

### No streaming output

The engine renders the complete PDF into memory before writing any bytes to the HTTP response. This means:

- A 500-page document holds its full layout in memory until the PDF is complete
- The client does not receive any bytes until rendering is done
- There is no ability to cancel mid-render if the client disconnects

For the expected use cases (documents up to 100 pages), memory consumption is in the range of 5–50 MB per request. Streaming would require a different PDF generation approach (incremental page writing) and is not planned for v1.

---

## Performance characteristics

These are approximate figures from local benchmarks on an M-series Mac. Production numbers will vary with the document content, page count, and hardware.

| Document | Format | Size | Pages | Time | Peak memory |
|---|---|---|---|---|---|
| Simple README | Markdown | 3 KB | 2 | ~30 ms | ~20 MB |
| Technical spec | Markdown | 80 KB | 45 | ~180 ms | ~45 MB |
| Data export | CSV, 500 rows | 60 KB | 30 | ~120 ms | ~35 MB |
| DOCX report | DOCX | 200 KB | 25 | ~200 ms | ~50 MB |

Memory includes the embedded fonts (~8 MB), the gopdf internal state, and the layout boxes. Peak memory is per-request; it is released when the response is sent.

Concurrent request handling: Go's goroutine model handles concurrent requests naturally. Under concurrent load, peak memory per request multiplies by the number of concurrent requests. At the default 5 MB input limit and 100 page limit, 10 concurrent requests should remain within 1 GB of total RSS.

---

## Security model

The engine is designed to be deployed as an internal service, not a public endpoint. The security model assumes:

1. **Network boundary:** The engine runs behind a gateway (Vercel, Nginx, Cloudflare) that handles TLS, DDoS protection, and IP allowlisting. The engine itself does not terminate TLS.

2. **Token authentication:** When `ANNAVE_INTERNAL_TOKEN` is set, all requests to `/convert` must include the correct `X-Internal-Token` header. The token comparison is a direct string equality check (not timing-safe constant-time comparison) — acceptable for an internal service where the token is long and random, but worth noting.

3. **Input sanitisation:** HTML inputs are sanitised via bluemonday. DOCX inputs are parsed from their ZIP/XML structure — no executable content is read or executed. Image inputs are decoded only for dimension detection; no image processing libraries with known parsing vulnerabilities are used.

4. **No outbound network:** The engine makes no outbound HTTP requests. URL images (e.g. `![alt](https://example.com/image.png)` in Markdown) are rendered as placeholders, not fetched. This prevents server-side request forgery (SSRF).

5. **Size limits:** All inputs are bounded by the limits in `config/limits.yaml`. The HTTP layer enforces a hard ceiling of `max_file_size_bytes + 1 MB` before any parsing begins.

---

## Future directions

- **Inline rich text rendering:** The layout engine computes per-token style information (`MeasuredToken` in `layout.go`), and the renderer tracks horizontal position across tokens and switches gopdf font state mid-line to render bold, italic, and code spans inline — but only for headings and paragraphs. List items and blockquote text carry the same span data through layout, then get flattened to a single font before rendering; tables have no per-cell span data in the AST to begin with. Extending inline rendering to lists and blockquotes, and adding span support to table cells, is what remains.

- **Streaming output:** Write each PDF page to the response as it is rendered, reducing time-to-first-byte for long documents. Requires gopdf support for incremental page writing.
