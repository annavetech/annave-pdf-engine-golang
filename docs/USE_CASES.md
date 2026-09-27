<!--
title:       ANNÁVE PDF Engine: Use Cases
description: Seven practical use cases with real library and CLI examples.
             No marketing language, just what works and what to watch for.
author:      Anna Veretennykova
website:     www.annave.tech
version:     1.3.1
created:     2026-05-06
updated:     2026-08-23
-->

# Use Cases

Seven practical scenarios with working examples and notes on edge cases.

---

## 1. Markdown documentation to PDF

**Scenario:** A CLI tool or CI pipeline converts a Markdown README or technical specification to a PDF for distribution.

```bash
annave pdf convert README.md -o output.pdf
```

Or as a library call, with the format given explicitly:

```go
e, err := pdfengine.New()
pdf, err := e.Convert(context.Background(), []byte(markdown), pdfengine.FormatMarkdown)
```

**What to watch:**
- Markdown with embedded HTML fragments (`<div>`, `<iframe>`): the engine sanitises HTML only when the format is explicitly `html` or when auto-detection identifies the input as HTML. Markdown with HTML fragments is not sanitised; the fragments are treated as literal text in the paragraph.
- GFM tables are fully supported. GitHub-specific extensions (task lists, footnotes) are parsed as plain text.
- Fenced code blocks preserve the language hint in the AST (`Lang` field) but the renderer does not yet apply syntax highlighting.

---

## 2. CSV data table to PDF

**Scenario:** Export a spreadsheet or database query result as a formatted PDF table.

```bash
annave pdf convert report.csv -o report.pdf
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

## 3. DOCX to PDF

**Scenario:** Convert a Word document to PDF as part of a document-processing pipeline.

```bash
annave pdf convert document.docx -o document.pdf
```

**What to watch:**
- DOCX support covers: headings (Heading1–Heading6, Title, Subtitle styles), paragraphs, bold/italic/strikethrough runs, ordered and unordered lists (reads `numbering.xml`), tables.
- Embedded images in DOCX are extracted by resolving the relationship ID to the image bytes and rendered in the PDF.
- Password-protected DOCX files cannot be parsed; `ENGINE_ERR_PARSE_FAILED` is returned.
- The DOCX parser has no external dependencies; it reads the ZIP/XML structure directly.

---

## 4. Jupyter Notebook to PDF

**Scenario:** Convert a `.ipynb` notebook for sharing or archiving.

```bash
annave pdf convert analysis.ipynb -o analysis.pdf
```

**What to watch:**
- Code cells are rendered as code blocks using the monospace font.
- Markdown cells are fully parsed.
- Cell output (stdout, stderr, display_data) is not included; only cell source content is converted.
- Large notebooks with many cells may hit `document.max_nodes`. Increase the limit in `config/limits.yaml` if needed.

---

## 5. HTML report to PDF

**Scenario:** A server-side template renders an HTML report, which is then converted to PDF.

```bash
annave pdf convert report.html -o report.pdf
```

Or, from a Go service that already has the HTML in memory:

```go
e, err := pdfengine.New()
pdf, err := e.Convert(context.Background(), []byte(html), pdfengine.FormatHTML)
```

**What to watch:**
- The engine sanitises HTML input via `bluemonday` before parsing. Allowed tags: standard text elements (p, h1–h6, ul, ol, li, table, pre, code, blockquote, strong, em, a, img, hr). Script tags, iframes, and style attributes are stripped.
- CSS is not applied. Inline styles, class attributes, and external stylesheets have no effect on the PDF output. The engine's own `config/style.yaml` controls all styling.
- Complex HTML layouts (grid, flexbox, floats) are not rendered as laid out; the engine extracts the text content and formats it as a document.

---

## 6. Large document with pagination

**Scenario:** Convert a 50-page technical specification or legal document.

```bash
annave pdf convert spec.md -o spec.pdf
```

If the document exceeds the default limits, `Convert` returns an error:

```
[pagination/ENGINE_ERR_TOO_MANY_PAGES] Document produced 147 pages; the maximum is 100. Reduce the document size or increase max_pages in config/limits.yaml.
```

Increase the limit in `config/limits.yaml`:

```yaml
document:
  max_pages: 500
```

Then rebuild.

---

## 7. Image to single-page PDF

**Scenario:** Wrap a PNG or JPEG in a PDF page, for archiving or consistent delivery format.

```bash
annave pdf convert diagram.png -o diagram.pdf
```

**What to watch:**
- Supported formats: PNG, JPEG, GIF, WebP
- The image is embedded at its original dimensions, scaled to fit within the page's text column width if it would overflow
- Very large images (dimensions in thousands of pixels) will be scaled down; quality depends on the original resolution
- The format is auto-detected from magic bytes, so the file extension does not need to be correct
