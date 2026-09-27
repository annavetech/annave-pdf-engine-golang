<!--
title:       ANNÁVE PDF Engine: Error Codes Reference
description: Full reference for all ENGINE_ERR_* codes: pipeline stage,
             when each code is triggered, and how to handle it in client code.
author:      Anna Veretennykova
website:     www.annave.tech
version:     1.3.0
created:     2026-05-06
updated:     2026-08-23
-->

# Error Codes Reference

Every error `Convert` returns can be unwrapped into a `*pdfengine.Error` with `errors.As`:

```go
e, err := pdfengine.New()
if err != nil {
    // A config-loading failure; see the "New" section below.
}

pdf, err := e.Convert(context.Background(), []byte(text), pdfengine.FormatMarkdown)
if err != nil {
    var pe *pdfengine.Error
    if errors.As(err, &pe) {
        // pe.Code is a stable, machine-readable identifier, e.g. "ENGINE_ERR_PARSE_FAILED".
        // pe.Stage is the pipeline stage that produced it.
        // pe.Message is human-readable and may change between versions.
    }
}
```

`New` itself can also fail: a caller-supplied `WithStyleFile`/`WithLimitsFile` path that is missing, not valid YAML, or fails structural validation. That error is a plain Go error, not a `*pdfengine.Error`: it happens before any document is converted, so it has no pipeline stage to report.

The `Code` field is the machine-readable identifier. The `Message` field is human-readable and may change between versions. Branch on `Code`, not on `Message`.

---

## Code pattern

```
ENGINE_(ERR|OK|WARN)_SLUG
```

- `ERR`: an error; the request failed and no PDF was produced
- `OK`: a success event (used in structured logs, not returned as an error)
- `WARN`: a warning (emitted in structured logs alongside `OK` and `ERR` events)

The slug is the unique identifier. There are no sequence numbers.

---

## Error codes

### `ENGINE_ERR_FILE_TOO_LARGE`
| Field | Value |
|---|---|
| Stage | `input` |
| Config key | `input.max_file_size_bytes` in `config/limits.yaml` |

**When triggered:** The input exceeds the configured byte limit (default 5 MB). The check runs before any parsing.

**Client handling:** Check the size of the file before conversion. If the file is legitimately large, increase `max_file_size_bytes` in `limits.yaml`.

---

### `ENGINE_ERR_INPUT_TOO_LARGE`
| Field | Value |
|---|---|
| Stage | `input` |
| Config key | `input.max_input_chars` in `config/limits.yaml` |

**When triggered:** After decoding to UTF-8 and normalising line endings, the input string exceeds the configured character limit (default 500,000). This check runs after the byte-size check and after normalisation.

**Client handling:** Split large documents into sections. If the document is genuinely that large, increase `max_input_chars`.

---

### `ENGINE_ERR_UNSUPPORTED_FORMAT`
| Field | Value |
|---|---|
| Stage | `input` |

**When triggered:** The requested format name does not match any registered parser (for example, the CLI's `--format` flag was given a value that isn't one of the supported format identifiers). The error names the format string that was rejected. `FormatAuto` (or omitting `--format`) never triggers this error: asking for auto-detection always auto-detects, falling back to plain text only if no format-specific pattern matches, which is a successful conversion, not an error. The public `pdfengine.Convert` API cannot trigger this error at all: its `Format` type is a closed, validated set, and any value outside that set is normalised to `FormatAuto` before reaching this check; only internal callers of `internal/parser`/`internal/engine` directly (today, that means the CLI's `--format` flag) can name a format specific enough to be rejected.

**Client handling:** Use one of the supported format identifiers: `md`, `html`, `json`, `csv`, `yaml`, `xml`, `rst`, `ipynb`, `docx`, `png`, `jpg`, `gif`, `webp`, `txt`.

---

### `ENGINE_ERR_PARSE_FAILED`
| Field | Value |
|---|---|
| Stage | `parser` |

**When triggered:** A parser was selected (by explicit format or auto-detection) but returned an error. Most parsers are lenient: this error typically occurs with binary formats (DOCX, image) that are malformed or truncated.

**Client handling:** Verify the file is not corrupt. For DOCX files, try resaving from the source application. For images, verify the file header.

---

### `ENGINE_ERR_INVALID_DOCUMENT`
| Field | Value |
|---|---|
| Stage | `validation` |

**When triggered:** The parsed document AST fails structural validation: the root node has the wrong type, or the document is nil. This indicates a bug in a parser, not a problem with the user's input.

**Client handling:** Report at the issue tracker. Include the input format and a minimal reproduction.

---

### `ENGINE_ERR_INVALID_NODE`
| Field | Value |
|---|---|
| Stage | `validation` |

**When triggered:** A block-level node has an unknown type, or a required field (text, items, headers, src) is missing. The error message includes the node index. This indicates a parser bug.

**Client handling:** Report at the issue tracker.

---

### `ENGINE_ERR_TOO_MANY_NODES`
| Field | Value |
|---|---|
| Stage | `validation` |
| Config key | `document.max_nodes` in `config/limits.yaml` |

**When triggered:** The parsed document has more block-level nodes than the configured limit (default 2,000). A node is one of: paragraph, heading, list, table, code block, blockquote, horizontal rule, or image.

**Client handling:** Split the document into smaller sections. If the document is legitimately large (technical manual, legal document), increase `max_nodes`.

---

### `ENGINE_ERR_TOO_MANY_PAGES`
| Field | Value |
|---|---|
| Stage | `pagination` |
| Config key | `document.max_pages` in `config/limits.yaml` |

**When triggered:** After layout and pagination, the document produces more pages than the configured limit (default 100). The error message includes the actual page count.

**Client handling:** Split the document. If the document is legitimately long (book-length), increase `max_pages`.

---

### `ENGINE_ERR_RENDERER_INIT`
| Field | Value |
|---|---|
| Stage | `render` |

**When triggered:** The PDF renderer (`gopdf`) failed to initialise. This is usually a missing or corrupt embedded font.

**Client handling:** Retry. If the error persists, report it. This should not occur in a correctly built binary.

---

### `ENGINE_ERR_RENDER_FAILED`
| Field | Value |
|---|---|
| Stage | `render` |

**When triggered:** The renderer initialised successfully but failed while writing the PDF. Usually caused by an unexpected node type that reached the render stage without being caught by validation.

**Client handling:** Retry. If the error persists with a specific document, report it with the input format and a minimal reproduction.

---

## Success event (log only)

### `ENGINE_OK_CONVERTED`

Emitted as a structured log line when a document is converted successfully. Useful for monitoring and alerting.

---

## Summary table

| Code | Stage |
|---|---|
| `ENGINE_ERR_FILE_TOO_LARGE` | input |
| `ENGINE_ERR_INPUT_TOO_LARGE` | input |
| `ENGINE_ERR_UNSUPPORTED_FORMAT` | input |
| `ENGINE_ERR_PARSE_FAILED` | parser |
| `ENGINE_ERR_INVALID_DOCUMENT` | validation |
| `ENGINE_ERR_INVALID_NODE` | validation |
| `ENGINE_ERR_TOO_MANY_NODES` | validation |
| `ENGINE_ERR_TOO_MANY_PAGES` | pagination |
| `ENGINE_ERR_RENDERER_INIT` | render |
| `ENGINE_ERR_RENDER_FAILED` | render |
