# ANNÁVE PDF Engine

A Go library and CLI that convert documents to PDF.
Supports Markdown, HTML, DOCX, CSV, JSON, YAML, XML, RST, Jupyter Notebooks, and images.

## Pipeline

The engine runs every document through six stages.

- Normalise: strip BOM, unify line endings, enforce character limit
- Parse: select parser by format hint or magic-byte detection
- Validate: check AST structure and enforce node count limit
- Layout: compute element positions using font metrics and page width
- Paginate: group layout boxes into pages within the height limit
- Render: drive gopdf to produce a PDF byte stream

## Error codes

All errors follow the `ENGINE_ERR_SLUG` pattern.

| Code | Stage |
|---|---|
| ENGINE_ERR_FILE_TOO_LARGE | input |
| ENGINE_ERR_PARSE_FAILED | parser |
| ENGINE_ERR_TOO_MANY_NODES | validation |
| ENGINE_ERR_TOO_MANY_PAGES | pagination |
| ENGINE_ERR_RENDER_FAILED | render |

## Configuration

Edit `config/limits.yaml` to adjust size limits, then rebuild.
Edit `config/style.yaml` to change fonts, sizes, and colours, then rebuild.

```yaml
document:
  max_nodes: 2000
  max_pages: 100
```

## Quick start

```bash
go build ./cmd/cli
annave pdf convert README.md -o output.pdf
```

> All configuration is embedded at build time. No external files are read at runtime.

---

### Supported formats

Final section confirming the eleven supported input formats and their auto-detection method.
