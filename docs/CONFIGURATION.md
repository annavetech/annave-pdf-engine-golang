<!--
title:       ANNÁVE PDF Engine: Configuration Reference
description: Every configuration key across limits.yaml, style.yaml,
             and messages.yaml, with valid values, defaults, and what breaks at extremes.
author:      Anna Veretennykova
website:     www.annave.tech
version:     1.3.1
created:     2026-05-06
updated:     2026-08-23
-->

# Configuration Reference

The engine uses three YAML files in `config/`. All three are embedded into the binary at build time via `//go:embed`, and every `Engine` uses those embedded values by default.

**To change the embedded default:** edit the YAML file, then rebuild with `go build ./cmd/cli`.

**To change `style.yaml` or `limits.yaml` without rebuilding:** pass your own file to `pdfengine.New` (`pdfengine.New(pdfengine.WithStyleFile(path))`, `pdfengine.New(pdfengine.WithLimitsFile(path))`, or both together), or, from the CLI, `annave pdf convert --style-file path` / `--limits-file path`. The library itself never reads a file unless one of these options names it explicitly: there is no implicit directory convention, no environment-variable lookup, and no filesystem scan inside the library. A caller that wants a discovery convention of its own (an environment variable, a fixed path, a flag) resolves that path itself and passes the result to one of these options. `config/messages.yaml` has no such override; every `Engine` uses the embedded messages.

Two `Engine` values built with different `WithStyleFile`/`WithLimitsFile` options hold genuinely independent configuration in the same process; nothing here is a single process-wide setting.

---

## `config/limits.yaml`

Controls input and output size limits. Exceeding any limit returns an `ENGINE_ERR_*` code; no partial output is produced.

### `input.max_file_size_bytes`

| Field | Value |
|---|---|
| Default | `5242880` (5 MB) |
| Unit | bytes |
| Error on exceed | `ENGINE_ERR_FILE_TOO_LARGE` |

Maximum size of a single input document.

| Size | Bytes | Notes |
|---|---|---|
| 1 MB | 1048576 | Rejects most DOCX files with images |
| 2 MB | 2097152 | Covers plain DOCX and most Markdown |
| 5 MB | 5242880 | Default: covers most real-world documents |
| 10 MB | 10485760 | Allows large CSV exports |
| 20 MB | 20971520 | Allows large Jupyter notebooks |
| 50 MB | 52428800 | Watch memory usage under concurrent load |

### `input.max_input_chars`

| Field | Value |
|---|---|
| Default | `500000` |
| Unit | Unicode characters |
| Error on exceed | `ENGINE_ERR_INPUT_TOO_LARGE` |

Maximum length of the normalised input string, applied after UTF-8 decode and line ending normalisation.

The same limit bounds YAML aliases: the content they expand to may total at most this many characters, where each value reached through an alias counts as its length in characters plus one. Past it, the conversion fails with `ENGINE_ERR_PARSE_FAILED`.

500,000 characters is roughly 300–400 pages of 13px body text at A4.

| Value | Notes |
|---|---|
| 100,000 | Short reports, invoices |
| 500,000 | Default |
| 2,000,000 | Technical manuals, legal docs; watch memory under concurrent load |

### `document.max_nodes`

| Field | Value |
|---|---|
| Default | `2000` |
| Unit | block-level nodes |
| Error on exceed | `ENGINE_ERR_TOO_MANY_NODES` |

Maximum number of block-level nodes in a parsed document. One node = one paragraph, heading, list, table, code block, blockquote, horizontal rule, or image. A list counts as one node regardless of item count; a table counts as one node regardless of row count.

| Value | Notes |
|---|---|
| 500 | Short reports, invoices |
| 2,000 | Default: covers most real-world documents |
| 10,000 | Very large technical documentation |

### `document.max_pages`

| Field | Value |
|---|---|
| Default | `100` |
| Unit | PDF pages |
| Error on exceed | `ENGINE_ERR_TOO_MANY_PAGES` |

Maximum number of PDF pages the engine will produce for a single conversion. 100 pages at A4 / 13px body / 1.65 line height is approximately 10,000 words.

| Value | Notes |
|---|---|
| 20 | Short reports: enforce page budget |
| 100 | Default |
| 500 | Book-length documents |

---

## `config/style.yaml`

Controls all visual output. All sizes are in CSS pixels at 96 DPI. 1 px = 0.2646 mm = 0.75 pt.

### `page`

| Key | Default | Notes |
|---|---|---|
| `width_px` | 794 | A4 width (210 mm). Letter = 816 px |
| `height_px` | 1123 | A4 height (297 mm). Letter = 1056 px |
| `margin_x_px` | 56 | Left and right margin, equal. Text column = width − (2 × margin) |
| `margin_top_px` | 48 | Top margin (~12.7 mm) |
| `margin_bottom_px` | 48 | Bottom margin (~12.7 mm) |

Text column width at defaults: 794 − (2 × 56) = 682 px.

### `fonts`

| Key | Default | Notes |
|---|---|---|
| `sans` | Inter (then system fallbacks) | Used for all prose text and headings |
| `mono` | JetBrains Mono (then system fallbacks) | Used for code blocks and inline code |

Both fonts are embedded in the binary. The fallback list applies only if gopdf cannot find the primary family.

### `text_styles`

Each entry applies to one block element type. Six styles are defined: `heading1`, `heading2`, `heading3`, `paragraph`, `code`, `blockquote`.

Common fields for each style:

| Field | Type | Notes |
|---|---|---|
| `font_family` | `"sans"` or `"mono"` | References the `fonts` keys above |
| `font_size_px` | integer | Text size in CSS pixels at 96 DPI |
| `font_weight` | `"400"`, `"600"`, `"700"`, `"800"` | Regular, semibold, bold, extrabold |
| `font_style` | `"normal"` or `"italic"` | |
| `line_height` | float | Multiplier on `font_size_px`. 1.65 at 13px = 21.45px between baselines |
| `margin_bottom_px` | integer | Space below each block of this type |
| `color` | `"#rrggbb"` | Must be 7-character hex |

Default values:

| Style | font_size_px | font_weight | line_height | color |
|---|---|---|---|---|
| `heading1` | 28 | 800 | 1.1 | #1d1d1f |
| `heading2` | 20 | 700 | 1.2 | #1d1d1f |
| `heading3` | 15 | 600 | 1.3 | #1d1d1f |
| `paragraph` | 13 | 400 | 1.65 | #3a3a3c |
| `code` | 11 | 400 | 1.6 | #1d1d1f |
| `blockquote` | 13 | 400 | 1.65 | #3a3a3c |

`blockquote` also sets `font_style: italic`.

---

## `config/messages.yaml`

Defines all user-facing error and success messages. Keys are error codes; values are message templates with `{placeholder}` interpolation.

**Operators can customise messages** by editing this file and rebuilding. For example, to add a support email to the `ENGINE_ERR_RENDER_FAILED` message:

```yaml
errors:
  ENGINE_ERR_RENDER_FAILED: "PDF rendering failed: {detail}. Contact support@example.com."
```

Do not rename keys: the engine looks up messages by error code. Do not remove keys: a missing key falls back to `"unknown error code: ENGINE_ERR_*"`.
