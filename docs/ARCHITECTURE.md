<!--
title:       ANNÁVE PDF Engine: Architecture
description: Six-stage pipeline overview, the public API and per-Engine
             configuration, package structure, and instructions for adding a new parser.
author:      Anna Veretennykova
website:     www.annave.tech
version:     1.3.0
created:     2026-05-06
updated:     2026-08-23
-->

# Architecture

## Design pattern

The engine is a public Go package (`pdfengine`, at the module root) wrapping a six-stage conversion pipeline (`internal/engine`). The pipeline itself has no knowledge of where its input came from or how its output is delivered: it takes a document and a format, and returns PDF bytes or an error. Two consumers drive it today: the `pdfengine` package (for library callers) and `cmd/cli` (for the `annave` command). Both call straight into `internal/engine`; there is no separate ports-and-adapters interface layer between them and the pipeline, and no runtime-pluggable delivery mechanism: adding a new one (a future MCP server, for example) means writing a new package that imports `internal/engine` or `pdfengine` directly, the same way `cmd/cli` does.

Configuration (style, limits, messages) is resolved once, when a `pdfengine.Engine` or `internal/engine.Pipeline` is constructed, into an immutable `internal/engine.Config`; see `docs/CONFIGURATION.md`. Two `Engine` values can hold two different `Config`s in the same process.

```
┌──────────────────────────────────────────────────────┐
│ Consumers                                            │
│ pdfengine (module root) : public library entry point │
│ cmd/cli/                : cobra CLI                  │
└──────────────────────────────────────────────────────┘
                      │  pdfengine.New(opts...) (*Engine, error)
                      │  (*Engine).Convert(ctx, data, format) ([]byte, error)
                      ▼
┌───────────────────────────────────────────────────────────────────┐
│ Pipeline : internal/engine/                                       │
│ NewPipeline(cfg *Config) *Pipeline                                │
│ (*Pipeline).Run(input string, format InputFormat) ([]byte, error) │
│ Stage 1: Normalise   normalizer.go                                │
│ Stage 2: Parse       parser registry                              │
│ Stage 3: Validate    validator.go                                 │
│ Stage 4: Layout      layout.go                                    │
│ Stage 5: Paginate    paginator.go                                 │
│ Stage 6: Render      renderer.go                                  │
└───────────────────────────────────────────────────────────────────┘
                      │  implements parser.Parser
                      ▼
┌───────────────────────────────────────────────────┐
│ Parsers : internal/parser/                        │
│ md.go, html.go, json.go, csv.go, yaml.go, xml.go, │
│ rst.go, ipynb.go, docx.go, image.go, txt.go       │
└───────────────────────────────────────────────────┘
```

---

## Six pipeline stages

### Stage 1: Normalise (`internal/engine/normalizer.go`)

Input: raw string (from a file, CLI stdin, or a direct library call)

- Detects and strips UTF-8 BOM
- Normalises line endings (CRLF, CR → LF)
- Enforces the Engine's configured `input.max_input_chars` (the embedded `config/limits.yaml` by default, or a caller-supplied file, see `docs/CONFIGURATION.md`)

Output: clean UTF-8 string with LF line endings

### Stage 2: Parse (`internal/parser/`)

Input: normalised string + format hint

The parser registry (`registry.go`) selects the appropriate parser:
- If `format` is explicit and recognised, use `byFormat[format]` directly
- If `format` is `auto`, iterate `ordered []Parser` and call `CanParse` on each until one accepts the input
- Binary parsers (DOCX, image) are listed first in the ordered slice; their `CanParse` checks magic bytes and is O(1)

Each parser implements `parser.Parser` (`internal/parser/interface.go`):
```go
type Parser interface {
    CanParse(input string) bool
    Parse(input string) (*ast.DocumentNode, error)
}
```

Output: `*ast.DocumentNode` (the internal AST)

### Stage 3: Validate (`internal/engine/validator.go`)

Input: `*ast.DocumentNode`

- Verifies the document root type is `"document"`
- Enforces the Engine's configured `document.max_nodes`
- Checks every block node for a valid type and required fields
- Checks every inline span for a valid kind

Output: validated document or `ENGINE_ERR_INVALID_*` / `ENGINE_ERR_TOO_MANY_NODES`

### Stage 4: Layout (`internal/engine/layout.go`)

Input: `*ast.DocumentNode`

Converts the abstract document tree into a flat list of `LayoutBox` values: concrete positioned elements with computed dimensions. At this stage, the engine knows the text column width, font metrics, and wrapping behaviour.

Each `LayoutBox` holds:
- Element type and content
- Computed height in pixels
- Text runs with style information (for inline bold/italic)

Output: `[]LayoutBox`

### Stage 5: Paginate (`internal/engine/paginator.go`)

Input: `[]LayoutBox`

Groups layout boxes into pages that fit within the configured page height minus margins. A box that would overflow the current page is moved to the next page.

Enforces the Engine's configured `document.max_pages`.

Output: `[]Page` where each `Page` holds a slice of `LayoutBox`

### Stage 6: Render (`internal/engine/renderer.go`)

Input: `[]Page`

Drives `gopdf` to produce a PDF byte stream:
- Opens a new PDF page for each `Page`
- For each `LayoutBox`: selects the font and size, sets the fill colour, calls `gopdf.Cell` or equivalent
- For image boxes: embeds the image bytes directly via `gopdf.ImageHolderByBytes`
- Returns the complete PDF as `[]byte`

Output: `[]byte` (PDF)

---

## AST types (`internal/ast/`)

The AST has no dependencies on other internal packages; it is the shared data type passed between pipeline stages.

### Block nodes (`ast.Node`)

| Type constant | `Node.Type` value | Key fields |
|---|---|---|
| `TypeDocument` | `"document"` | `Children []Node` |
| `TypeHeading` | `"heading"` | `Text`, `Level` (1–3), `Spans` |
| `TypeParagraph` | `"paragraph"` | `Text`, `Spans` |
| `TypeList` | `"list"` | `Items []string`, `Ordered bool` |
| `TypeTable` | `"table"` | `Headers []string`, `Rows [][]string` |
| `TypeCodeBlock` | `"code_block"` | `Text`, `Lang` |
| `TypeBlockquote` | `"blockquote"` | `Text`, `Spans` |
| `TypeHR` | `"hr"` | *(no content fields)* |
| `TypeImage` | `"image"` | `Src`, `Alt`, `Data []byte` |

`Data []byte` on image nodes holds the raw image bytes when the image was uploaded as a file. It is excluded from JSON serialisation (`json:"-"`) since it is an in-memory transport field.

### Inline spans (`ast.InlineSpan`)

| Kind | Description |
|---|---|
| `SpanText` | Plain text |
| `SpanBold` | Bold text |
| `SpanItalic` | Italic text |
| `SpanBoldItalic` | Bold and italic |
| `SpanCode` | Inline code |
| `SpanStrike` | Strikethrough |
| `SpanLink` | Hyperlink; `Href` field holds the URL |

---

## Package map

```
github.com/annavetech/pdfengine/
├── cmd/
│   └── cli/             : cobra CLI entry point
├── config/              : Go package; embeds and exports all YAML bytes
├── internal/
│   ├── ast/             : DocumentNode, Node, InlineSpan types
│   ├── engine/          : Pipeline, Config/LoadConfig, and the six pipeline stages
│   └── parser/          : One file per supported input format, behind parser.Parser
└── schema/              : JSON Schema definitions
```

---

## How to add a new input format

1. **Create `internal/parser/yourformat.go`** implementing `parser.Parser`:

```go
package parser

import "github.com/annavetech/pdfengine/internal/ast"

type YourParser struct{}

// CanParse returns true if this parser should handle the given input.
// For text formats: check a distinctive prefix or structural marker.
// For binary formats, check magic bytes: input[0:N] == expectedMagic.
func (p *YourParser) CanParse(input string) bool {
    return len(input) > 4 && input[:4] == "YOUM"
}

func (p *YourParser) Parse(input string) (*ast.DocumentNode, error) {
    doc := &ast.DocumentNode{Type: ast.TypeDocument}
    // ... parse input, populate doc.Children ...
    return doc, nil
}
```

2. **Add the format constant and extension mapping to `registry.go`**:

```go
const FormatYour InputFormat = "your"

// In extToFormat:
"your": FormatYour,

// In NewRegistry().ordered (binary before text parsers):
&YourParser{},

// In NewRegistry().byFormat:
FormatYour: &YourParser{},
```

3. **Add a test in `internal/parser/yourformat_test.go`** using a real fixture from the ANNÁVE PDF Engine documentation as input.

4. **Update `config/messages.yaml`** to include the new format in the `ENGINE_ERR_UNSUPPORTED_FORMAT` message list.

Nothing else changes. The pipeline and renderer are format-agnostic.

---

## Error types (`internal/engine/errors.go`)

All pipeline errors are `*AnnaveError`:

```go
type AnnaveError struct {
    Code    string
    Stage   EngineStage
    Message string
}
```

`EngineStage` values: `input`, `parser`, `validation`, `layout`, `pagination`, `render`.

Create an error with `engine.NewError(code, stage, message)`. The public `pdfengine` package translates it to a `*pdfengine.Error`; callers use `errors.As` to obtain it and branch on `Code`.
