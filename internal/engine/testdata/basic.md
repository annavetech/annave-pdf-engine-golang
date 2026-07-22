# ANNAVE PDF Engine

A Go service that converts documents to PDF in a single HTTP request.
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

| Code | HTTP | Stage |
|---|---|---|
| ENGINE_ERR_FILE_TOO_LARGE | 400 | input |
| ENGINE_ERR_EMPTY_INPUT | 400 | input |
| ENGINE_ERR_PARSE_FAILED | 422 | parser |
| ENGINE_ERR_TOO_MANY_NODES | 422 | validation |
| ENGINE_ERR_TOO_MANY_PAGES | 422 | pagination |
| ENGINE_ERR_INTERNAL | 500 | render |

## Configuration

Edit `config/limits.yaml` to adjust size limits, then rebuild.
Edit `config/style.yaml` to change fonts, sizes, and colours, then rebuild.
Edit `config/server.yaml` for port, CORS, and debug settings.

```yaml
document:
  max_nodes: 2000
  max_pages: 100
```

## Quick start

```bash
go build ./cmd/server
PORT=8080 ./cmd/server

curl -s localhost:5741/convert?format=md \
  -H "Content-Type: text/plain" \
  --data-binary @README.md -o output.pdf
```

> All configuration is embedded at build time. No external files are read at runtime.

---

### Supported formats

Final section confirming the eleven supported input formats and their auto-detection method.
