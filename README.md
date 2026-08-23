# ANNAVE PDF Engine

Convert any document format to PDF with a single API call. No headless browser. No C dependencies. Self-hosted.

* **Author:** Anna Veretennykova · [www.annave.tech](https://www.annave.tech)
* **License:** Apache 2.0

---

## Why this exists

Every available option has a cost that isn't obvious until production:

| Tool | Problem |
|---|---|
| Headless Chrome / Puppeteer | 200 MB binary, high memory, cold-start latency |
| wkhtmltopdf | Abandoned in 2023, requires CGO, no ARM build |
| PDFco / HTML2PDF / similar | Per-call pricing, your documents leave your infrastructure |
| ReportLab (Python) | Python runtime required, not embeddable in Go services |

ANNAVE PDF Engine is a single Go binary (~10 MB with embedded fonts). It takes raw text or structured JSON as input and returns PDF bytes. No runtime dependencies. No external calls. Deploy it once and it runs indefinitely.

---

## Installation

The `annave` CLI converts documents to PDF without running a server.

```bash
# Go — works today
go install github.com/annavetech/annave-pdf-engine-golang/cmd/cli@latest

# Homebrew (macOS, Linux) — once a tagged release exists
brew tap annavetech/annave
brew install annave-pdf-engine
```

```bash
annave pdf convert report.md -o report.pdf
```

---

## Quickstart

```bash
# Start the server
go run cmd/server/main.go

# Convert a Markdown file
curl -X POST http://localhost:5741/convert \
  -F "file=@README.md" \
  -o output.pdf

# Convert a JSON document
curl -X POST http://localhost:5741/convert \
  -H "Content-Type: application/json" \
  -d '{"type":"document","children":[{"type":"heading","level":1,"text":"Hello"},{"type":"paragraph","text":"World."}]}' \
  -o output.pdf
```

---

## Use as a library

The engine is also a Go module, for services that want to convert documents
in-process instead of calling an HTTP endpoint:

```bash
go get github.com/annavetech/annave-pdf-engine-golang
```

```go
import "github.com/annavetech/annave-pdf-engine-golang"

pdf, err := pdfengine.New().Convert(text, pdfengine.FormatMarkdown)
if err != nil {
    var pe *pdfengine.Error
    if errors.As(err, &pe) {
        // pe.Code is a stable, machine-readable identifier, e.g. "ENGINE_ERR_PARSE_FAILED".
    }
    return err
}
```

An `Engine` is safe to reuse across many calls to `Convert`. Pass
`pdfengine.FormatAuto` to detect the format from the content, or override
typography and page margins per call with `pdfengine.WithStyle`.

---

## Supported input formats

| Format | Extension | Notes |
|---|---|---|
| Markdown (GFM) | `.md` | ATX headings, fenced code, tables, inline formatting |
| Plain text | `.txt` | Paragraphs separated by blank lines |
| JSON | `.json` | Document schema or auto-detected structure |
| HTML | `.html`, `.htm` | Sanitised with bluemonday before parsing |
| CSV | `.csv`, `.tsv` | Rendered as a table |
| YAML | `.yaml`, `.yml` | Maps to document structure |
| XML | `.xml` | Element-to-node mapping |
| reStructuredText | `.rst` | Common directives supported |
| Jupyter Notebook | `.ipynb` | Code and markdown cells |
| Word Document | `.docx` | Headings, paragraphs, lists, tables; pure Go, no COM/LibreOffice |
| Raster image | `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp` | Embedded at full page width with correct aspect ratio |

Pass the format explicitly with `?format=md` to skip auto-detection.

---

## API reference

### `POST /convert`

Accepts three content types:

**Multipart form upload:**
```
Content-Type: multipart/form-data
Field: file — the document file
```

**Raw body with format hint:**
```
Content-Type: text/plain
GET parameter: ?format=md
```

**JSON document:**
```
Content-Type: application/json
Body: { "type": "document", "children": [...] }
```

**Success response — 200 OK:**
```
Content-Type: application/pdf
X-Engine-Version: <engine version>
X-Request-Id: <uuid>
Body: <PDF bytes>
```

**Error response — 4xx/5xx:**
```json
{
  "error": {
    "code": "ENGINE_ERR_FILE_TOO_LARGE",
    "stage": "input",
    "message": "File exceeds the maximum allowed size of 5 MB."
  }
}
```

See [`schema/error.v1.schema.json`](schema/error.v1.schema.json) for the full error schema and [`config/messages.yaml`](config/messages.yaml) for all error codes.

### `GET /health`

Returns `200 OK` with `{"status":"ok","version":"<engine version>"}`. Use this as a readiness probe.

---

## JSON document schema

The canonical input format that all parsers produce internally. Send it directly for programmatic use:

```json
{
  "type": "document",
  "version": "1",
  "children": [
    { "type": "heading", "level": 1, "text": "Report Title" },
    { "type": "paragraph", "text": "Summary paragraph." },
    {
      "type": "table",
      "headers": ["Date", "Value"],
      "rows": [["2026-01-01", "42"]]
    }
  ]
}
```

Full schema: [`schema/document.v1.schema.json`](schema/document.v1.schema.json)

---

## Configuration

Edit the YAML files in `config/` and rebuild. No code changes required.

| File | Controls |
|---|---|
| [`config/style.yaml`](config/style.yaml) | Page size, margins, font sizes, line heights, colors |
| [`config/limits.yaml`](config/limits.yaml) | Max file size, max nodes, max pages |
| [`config/messages.yaml`](config/messages.yaml) | All error messages and their codes |

The configuration is embedded in the binary at build time. A custom deployment (different page size, corporate fonts, stricter limits) only requires editing YAML and running `go build`.

---

## Architecture

The engine follows a hexagonal (ports and adapters) structure. The six-stage pipeline is the domain core; everything outside it is an adapter.

```
Delivery adapter (HTTP, gRPC, CLI)
        │
        ▼
  [ Converter port ]
        │
        ▼
┌──────────────────────────────────────────┐
│  Pipeline (domain core)                  │
│                                          │
│  1. Normalise   — clean raw input        │
│  2. Parse       — format → AST           │
│  3. Validate    — AST constraints        │
│  4. Layout      — AST → LayoutBox[]     │
│  5. Paginate    — LayoutBox[] → Page[]  │
│  6. Render      — Page[] → bytes        │
└──────────────────────────────────────────┘
        │
        ▼
  [ Renderer port ]
        │
        ▼
   PDF bytes
```

Adding a new input format: implement `port.DocumentParser` and register it in `internal/parser/registry.go`. Nothing else changes.

Adding a new delivery mechanism (gRPC, CLI, Lambda): implement `port.Converter` and wire it to `engine.Pipeline`. Nothing in the domain core changes.

---

## Security

**Internal token enforcement:**
Set `ANNAVE_INTERNAL_TOKEN` in the environment to require callers to send `X-Internal-Token: <secret>` on every request. Without the variable the engine runs open (suitable for localhost development).

```bash
ANNAVE_INTERNAL_TOKEN=your-secret go run cmd/server/main.go
```

The recommended deployment pattern for browser clients is the Backend-for-Frontend (BFF) approach: the browser calls your server-side API route, which holds the secret and forwards requests to the engine. The secret never reaches the browser.

**Input validation:**
- Maximum file size: 5 MB (configurable)
- Maximum input characters: 500,000
- Maximum AST nodes: 2,000
- Maximum output pages: 100
- HTML sanitisation via bluemonday on all HTML input

---

## Running tests

```bash
# Unit and integration tests
go test ./...

# Single package with verbose output
go test -v ./internal/engine/...

# Fuzz the Markdown parser (run for 30 seconds)
go test -fuzz=FuzzMdParser -fuzztime=30s ./internal/parser/...
```

---

## Building a release binary

```bash
go build -o annave-pdf-engine ./cmd/server
```

The binary embeds all fonts and configuration. No external files are needed at runtime.

For cross-compilation (e.g. Linux on macOS):
```bash
GOOS=linux GOARCH=amd64 go build -o annave-pdf-engine-linux ./cmd/server
```

---

## Use cases

- **SaaS "Export to PDF" feature** — POST structured JSON from your backend, receive PDF bytes. No client-side rendering, no Puppeteer process to manage.
- **Mobile app reports** — iOS or Android app posts structured data directly to the engine (gRPC planned for v2). No platform-specific PDF library needed.
- **Data pipeline output** — CSV exports, Jupyter notebooks, YAML reports → clean PDF for stakeholder review or archival.
- **Technical documentation** — Markdown or RST files → PDF for distribution or print.
- **Self-hosted, zero vendor lock-in** — one binary, no API keys, no per-conversion pricing, runs anywhere Go does.

---

## Third-party attributions

See [NOTICE](NOTICE) for the full list of open-source components included in this project.
