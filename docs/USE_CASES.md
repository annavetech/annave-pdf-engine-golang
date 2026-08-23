<!--
title:       ANNAVE PDF Engine — Use Cases
description: Eight practical use cases with real request and response examples.
             No marketing language — just what works and what to watch for.
author:      Anna Veretennykova
website:     www.annave.tech
version:     1.1.0
created:     2026-05-06
updated:     2026-08-23
-->

# Use Cases

Eight practical scenarios with working request examples and notes on edge cases.

---

## 1. Markdown documentation to PDF

**Scenario:** A CLI tool or CI pipeline converts a Markdown README or technical specification to a PDF for distribution.

```bash
curl -s http://localhost:5741/convert?format=md \
  -H "Content-Type: text/plain" \
  --data-binary @README.md \
  -o output.pdf
```

Or via file upload (auto-detects format from `.md` extension):

```bash
curl -s http://localhost:5741/convert \
  -F "file=@README.md" \
  -o output.pdf
```

**What to watch:**
- Markdown with embedded HTML fragments (`<div>`, `<iframe>`) — the engine sanitises HTML only when the format is explicitly `html` or when auto-detection identifies the input as HTML. Markdown with HTML fragments is not sanitised; the fragments are treated as literal text in the paragraph.
- GFM tables are fully supported. GitHub-specific extensions (task lists, footnotes) are parsed as plain text.
- Fenced code blocks preserve the language hint in the AST (`Lang` field) but the renderer does not yet apply syntax highlighting.

---

## 2. CSV data table to PDF

**Scenario:** Export a spreadsheet or database query result as a formatted PDF table.

```bash
curl -s http://localhost:5741/convert?format=csv \
  -H "Content-Type: text/plain" \
  --data-binary @report.csv \
  -o report.pdf
```

Example input (`report.csv`):

```csv
Name,Role,Status
Anna Veretennykova,Lead,Active
PDF Engine,Service,Running
```

**What to watch:**
- The first row is treated as column headers.
- Very wide tables (many columns) may produce columns too narrow to read at A4 width. Split into multiple narrower tables in the source document.
- TSV (tab-separated) files are auto-detected correctly by extension (`.tsv`) and by content (tab delimiter heuristic).

---

## 3. DOCX upload to PDF

**Scenario:** Accept a DOCX file uploaded from a browser and return a PDF.

```bash
curl -s http://localhost:5741/convert \
  -F "file=@document.docx" \
  -o document.pdf
```

From a browser form:

```html
<form action="http://localhost:5741/convert" method="post" enctype="multipart/form-data">
  <input type="file" name="file" accept=".docx">
  <button type="submit">Convert</button>
</form>
```

**What to watch:**
- DOCX support covers: headings (Heading1–Heading6, Title, Subtitle styles), paragraphs, bold/italic/strikethrough runs, ordered and unordered lists (reads `numbering.xml`), tables.
- Embedded images in DOCX are extracted by resolving the relationship ID to the image bytes and rendered in the PDF.
- Password-protected DOCX files cannot be parsed — `ENGINE_ERR_PARSE_FAILED` is returned.
- The DOCX parser has no external dependencies — it reads the ZIP/XML structure directly.

---

## 4. Jupyter Notebook to PDF

**Scenario:** Convert a `.ipynb` notebook for sharing or archiving.

```bash
curl -s http://localhost:5741/convert \
  -F "file=@analysis.ipynb" \
  -o analysis.pdf
```

**What to watch:**
- Code cells are rendered as code blocks using the monospace font.
- Markdown cells are fully parsed.
- Cell output (stdout, stderr, display_data) is not included — only cell source content is converted.
- Large notebooks with many cells may hit `document.max_nodes`. Increase the limit in `config/limits.yaml` if needed.

---

## 5. HTML report to PDF

**Scenario:** A server-side template renders an HTML report, which is then converted to PDF.

```bash
curl -s http://localhost:5741/convert?format=html \
  -H "Content-Type: text/html" \
  --data-binary @report.html \
  -o report.pdf
```

**What to watch:**
- The engine sanitises HTML input via `bluemonday` before parsing. Allowed tags: standard text elements (p, h1–h6, ul, ol, li, table, pre, code, blockquote, strong, em, a, img, hr). Script tags, iframes, and style attributes are stripped.
- CSS is not applied. Inline styles, class attributes, and external stylesheets have no effect on the PDF output. The engine's own `config/style.yaml` controls all styling.
- Complex HTML layouts (grid, flexbox, floats) are not rendered as laid out — the engine extracts the text content and formats it as a document.

---

## 6. REST API with token authentication

**Scenario:** The engine is deployed as an internal service behind an API gateway. Only authorised callers may use it.

Set `ANNAVE_INTERNAL_TOKEN` to a strong random secret:

```bash
ANNAVE_INTERNAL_TOKEN=my-secret go run cmd/server/main.go
```

All requests must include the token:

```bash
curl -s http://localhost:5741/convert?format=md \
  -H "Content-Type: text/plain" \
  -H "X-Internal-Token: my-secret" \
  --data-binary @README.md \
  -o output.pdf
```

Without the token:

```
HTTP 401
{"error":{"code":"ENGINE_ERR_UNAUTHORIZED","stage":"input","message":"Missing or invalid internal token."}}
```

**What to watch:**
- If `ANNAVE_INTERNAL_TOKEN` is empty or unset, token enforcement is disabled. This is intentional for local development. Never run without a token in production.
- The token must match exactly, including case and any trailing whitespace. Generate a strong token with `openssl rand -hex 32`.

---

## 7. Large document with pagination

**Scenario:** Convert a 50-page technical specification or legal document.

```bash
curl -s http://localhost:5741/convert?format=md \
  -H "Content-Type: text/plain" \
  --data-binary @spec.md \
  -o spec.pdf
```

If the document exceeds the default limits:

```json
{
  "error": {
    "code": "ENGINE_ERR_TOO_MANY_PAGES",
    "stage": "pagination",
    "message": "Document produced 147 pages; the maximum is 100. Reduce the document size or increase max_pages in config/limits.yaml."
  }
}
```

Increase the limit in `config/limits.yaml`:

```yaml
document:
  max_pages: 500
```

Then rebuild and redeploy.

---

## 8. Image to single-page PDF

**Scenario:** Wrap a PNG or JPEG in a PDF page, for archiving or consistent delivery format.

```bash
curl -s http://localhost:5741/convert \
  -F "file=@diagram.png" \
  -o diagram.pdf
```

Or raw body:

```bash
curl -s http://localhost:5741/convert?format=png \
  -H "Content-Type: application/octet-stream" \
  --data-binary @diagram.png \
  -o diagram.pdf
```

**What to watch:**
- Supported formats: PNG, JPEG, GIF, WebP
- The image is embedded at its original dimensions, scaled to fit within the page's text column width if it would overflow
- Very large images (dimensions in thousands of pixels) will be scaled down; quality depends on the original resolution
- The format is auto-detected from magic bytes, so the file extension does not need to be correct
