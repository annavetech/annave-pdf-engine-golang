# ANNÁVE PDF Engine

Convert any document format to PDF from Go code or the command line. No headless browser. No C dependencies. Self-hosted.

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

ANNÁVE PDF Engine is a single Go binary (~10 MB with embedded fonts). It takes raw text or structured JSON as input and returns PDF bytes. No runtime dependencies. No external calls. Deploy it once and it runs indefinitely.

---

## Installation

The `annave` CLI converts documents to PDF without running a server.

```bash
# Go: works today
go install github.com/annavetech/pdfengine/cmd/cli@latest

# Homebrew (macOS, Linux): once a tagged release exists
brew tap annavetech/annave
brew install annave-pdf-engine
```

```bash
annave pdf convert report.md -o report.pdf
```

---

## Use as a library

The engine is also a Go module, for services that want to convert documents
in-process:

```bash
go get github.com/annavetech/pdfengine
```

```go
import "github.com/annavetech/pdfengine"

e, err := pdfengine.New()
if err != nil {
    return err
}

pdf, err := e.Convert(context.Background(), []byte(text), pdfengine.FormatMarkdown)
if err != nil {
    var pe *pdfengine.Error
    if errors.As(err, &pe) {
        // pe.Code is a stable, machine-readable identifier, e.g. "ENGINE_ERR_PARSE_FAILED".
    }
    return err
}
```

An `Engine` is safe to reuse across many calls to `Convert`. Pass `pdfengine.FormatAuto` to detect the format from the content, or override typography and page margins when building the `Engine` with `pdfengine.WithStyle`, `pdfengine.WithStyleFile`, or `pdfengine.WithLimitsFile`. Configuration is fixed per `Engine`, not per call, so two `Engine` values can hold two different configurations in the same process.

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

Pass the format explicitly with `pdfengine.FormatMarkdown` (library) or `--format md` (CLI) to skip auto-detection.

---

## JSON document schema

The canonical input format that all parsers produce internally. Pass it directly as `pdfengine.FormatJSON` input for programmatic use:

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

| File | Controls |
|---|---|
| [`config/style.yaml`](config/style.yaml) | Page size, margins, font sizes, line heights, colors |
| [`config/limits.yaml`](config/limits.yaml) | Max file size, max nodes, max pages |
| [`config/messages.yaml`](config/messages.yaml) | All error messages and their codes |

All three are embedded in the binary at build time; editing one and running `go build` changes the default for every `Engine` that does not override it.

`style.yaml` and `limits.yaml` can also be replaced per `Engine`, at runtime, without a rebuild: `pdfengine.New(pdfengine.WithStyleFile(path), pdfengine.WithLimitsFile(path))`, or `annave pdf convert --style-file path --limits-file path` from the CLI. The library never reads a file unless one of these options names it explicitly; see [`docs/CONFIGURATION.md`](docs/CONFIGURATION.md).

---

## Architecture

The public `pdfengine` package and the `annave` CLI both call straight into a six-stage internal pipeline (`internal/engine`). The pipeline has no knowledge of where its input came from or how its output is delivered.

```
pdfengine.New(opts...) (*Engine, error)
(*Engine).Convert(ctx, data, format) ([]byte, error)
        │
        ▼
┌──────────────────────────────────────────┐
│  Pipeline                                │
│                                          │
│  1. Normalise   : clean raw input        │
│  2. Parse       : format → AST           │
│  3. Validate    : AST constraints        │
│  4. Layout      : AST → LayoutBox[]      │
│  5. Paginate    : LayoutBox[] → Page[]   │
│  6. Render      : Page[] → bytes         │
└──────────────────────────────────────────┘
        │
        ▼
   PDF bytes
```

Adding a new input format: implement `parser.Parser` and register it in `internal/parser/registry.go`. Nothing else changes. See `docs/ARCHITECTURE.md` for the full package map and configuration model.

---

## Security

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
go build -o annave ./cmd/cli
```

The binary embeds all fonts and configuration. No external files are needed at runtime.

For cross-compilation (e.g. Linux on macOS):
```bash
GOOS=linux GOARCH=amd64 go build -o annave-linux ./cmd/cli
```

---

## Use cases

- **SaaS "Export to PDF" feature**: call the library with structured JSON from your backend, get PDF bytes back. No client-side rendering, no Puppeteer process to manage.
- **Mobile app reports**: the backend serving a mobile app embeds the library, or shells out to the CLI, to generate PDFs. No platform-specific PDF library needed.
- **Data pipeline output**: CSV exports, Jupyter notebooks, YAML reports → clean PDF for stakeholder review or archival.
- **Technical documentation**: Markdown or RST files → PDF for distribution or print.
- **Self-hosted, zero vendor lock-in**: one binary, no API keys, no per-conversion pricing, runs anywhere Go does.

---

## Third-party attributions

See [NOTICE](NOTICE) for the full list of open-source components included in this project.
