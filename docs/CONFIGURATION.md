<!--
title:       ANNAVE PDF Engine — Configuration Reference
description: Every configuration key across limits.yaml, style.yaml, server.yaml,
             and messages.yaml — with valid values, defaults, and what breaks at extremes.
author:      Anna Veretennykova
website:     www.annave.tech
version:     1.0.0
created:     2026-05-06
updated:     2026-05-06
-->

# Configuration Reference

The engine uses four YAML files in `config/`. All are embedded into the binary at build time via `//go:embed`. There are no external file reads at runtime.

**To change a value:** edit the YAML file, then rebuild with `go build ./cmd/server`.

Environment variables override specific server settings at runtime without a rebuild — see `config/server.yaml` for which keys have env var overrides.

---

## `config/limits.yaml`

Controls input and output size limits. Exceeding any limit returns an `ENGINE_ERR_*` code; no partial output is produced.

### `input.max_file_size_bytes`

| Field | Value |
|---|---|
| Default | `5242880` (5 MB) |
| Unit | bytes |
| Error on exceed | `ENGINE_ERR_FILE_TOO_LARGE` |

Maximum size of a single uploaded file or raw request body. The HTTP layer enforces a slightly higher ceiling (this value + 1 MB) to account for multipart form encoding overhead.

| Size | Bytes | Notes |
|---|---|---|
| 1 MB | 1048576 | Rejects most DOCX files with images |
| 2 MB | 2097152 | Covers plain DOCX and most Markdown |
| 5 MB | 5242880 | Default — covers most real-world documents |
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

500,000 characters is roughly 300–400 pages of 13px body text at A4.

| Value | Notes |
|---|---|
| 100,000 | Short reports, invoices |
| 500,000 | Default |
| 2,000,000 | Technical manuals, legal docs — watch memory under concurrent load |

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
| 2,000 | Default — covers most real-world documents |
| 10,000 | Very large technical documentation |

### `document.max_pages`

| Field | Value |
|---|---|
| Default | `100` |
| Unit | PDF pages |
| Error on exceed | `ENGINE_ERR_TOO_MANY_PAGES` |

Maximum number of PDF pages per request. 100 pages at A4 / 13px body / 1.65 line height is approximately 10,000 words.

| Value | Notes |
|---|---|
| 20 | Short reports — enforce page budget |
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

## `config/server.yaml`

Controls the HTTP server, CORS, and optional features. Environment variables override YAML values at startup — no rebuild needed.

### `server.port`

| Field | Value |
|---|---|
| Default | `5741` |
| Env var override | `PORT` |

TCP port the server listens on. Most PaaS platforms (Vercel, Railway, Render) set `PORT` automatically.

### `server.debug`

| Field | Value |
|---|---|
| Default | `false` |
| Env var override | `ANNAVE_DEBUG=true` |

When `true`, sets the slog level to `DEBUG`. Do not enable in production — debug logs may include request content.

### `cors.allowed_origin`

| Field | Value |
|---|---|
| Default | `"*"` |
| Env var override | `ANNAVE_CORS_ORIGIN` |

The value of the `Access-Control-Allow-Origin` response header. Use `"*"` for public APIs or local development. Use your specific domain (`https://www.annave.tech`) in production.

### `schema.base_url`

| Field | Value |
|---|---|
| Default | `https://www.annave.tech/pdf-engine/schema` |

Base URL for JSON Schema `$id` fields. Change this if you fork the engine and host your own schema registry. Informational only — the engine does not fetch schemas at runtime.

### `rate_limit.requests_per_minute`

| Field | Value |
|---|---|
| Default | `0` (disabled) |

Per-IP sliding-window rate limit. 0 disables rate limiting. Not yet wired up in v1.0.0 — use an upstream gateway (Nginx, Vercel Edge, Cloudflare) for rate limiting in production.

---

## `config/messages.yaml`

Defines all user-facing error and success messages. Keys are error codes; values are message templates with `{placeholder}` interpolation.

**Operators can customise messages** by editing this file and rebuilding. For example, to add a support email to the `ENGINE_ERR_INTERNAL` message:

```yaml
errors:
  ENGINE_ERR_INTERNAL: "An internal error occurred. Contact support@example.com with request ID {request_id}."
```

Do not rename keys — the engine looks up messages by error code. Do not remove keys — a missing key falls back to `"unknown error code: ENGINE_ERR_*"`.

---

## Environment variables summary

| Variable | Config key overridden | Default |
|---|---|---|
| `PORT` | `server.port` | `5741` |
| `ANNAVE_INTERNAL_TOKEN` | *(no YAML equivalent)* | `""` (auth disabled) |
| `ANNAVE_DEBUG` | `server.debug` | `false` |
| `ANNAVE_CORS_ORIGIN` | `cors.allowed_origin` | `"*"` |
