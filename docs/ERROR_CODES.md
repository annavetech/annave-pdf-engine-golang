<!--
title:       ANNÁVE PDF Engine — Error Codes Reference
description: Full reference for all ENGINE_ERR_* codes: HTTP status, pipeline stage,
             when each code is triggered, and how to handle it in client code.
author:      Anna Veretennykova
website:     www.annave.tech
version:     1.1.0
created:     2026-05-06
updated:     2026-08-23
-->

# Error Codes Reference

All errors from the convert endpoint follow the same JSON structure:

```json
{
  "error": {
    "code":    "ENGINE_ERR_FILE_TOO_LARGE",
    "stage":   "input",
    "message": "File exceeds the maximum allowed size of 5 MB."
  }
}
```

The `code` field is the machine-readable identifier. The `message` field is human-readable and may change between versions. Parse `code`, not `message`.

The JSON Schema for this response is at `schema/error.v1.schema.json`.

---

## Code pattern

```
ENGINE_(ERR|OK|WARN)_SLUG
```

- `ERR` — an error; the request failed and no PDF was produced
- `OK` — a success event (used in structured logs, not API responses)
- `WARN` — a warning (emitted in structured logs alongside `OK` and `ERR` events)

The slug is the unique identifier. There are no sequence numbers.

---

## Error codes

### `ENGINE_ERR_FILE_TOO_LARGE`
| Field | Value |
|---|---|
| HTTP status | 400 |
| Stage | `input` |
| Config key | `input.max_file_size_bytes` in `config/limits.yaml` |

**When triggered:** The raw request body or uploaded file exceeds the configured byte limit (default 5 MB). The check runs before any parsing.

**Client handling:** Check the size of the file before uploading. If the file is legitimately large, ask the operator to increase `max_file_size_bytes` in `limits.yaml`.

---

### `ENGINE_ERR_EMPTY_INPUT`
| Field | Value |
|---|---|
| HTTP status | 400 |
| Stage | `input` |

**When triggered:** The request body, form field, or uploaded file is empty or contains only whitespace after trimming.

**Client handling:** Validate that the input is non-empty before sending the request.

---

### `ENGINE_ERR_INPUT_TOO_LARGE`
| Field | Value |
|---|---|
| HTTP status | 400 |
| Stage | `input` |
| Config key | `input.max_input_chars` in `config/limits.yaml` |

**When triggered:** After decoding to UTF-8 and normalising line endings, the input string exceeds the configured character limit (default 500,000). This check runs after the byte-size check and after normalisation.

**Client handling:** Split large documents into sections. If the document is genuinely that large, ask the operator to increase `max_input_chars`.

---

### `ENGINE_ERR_UNSUPPORTED_FORMAT`
| Field | Value |
|---|---|
| HTTP status | 400 |
| Stage | `input` |

**When triggered:** The `format` query parameter specifies a format the engine does not recognise. Auto-detection (`format=auto` or no `format` param) never triggers this error — it falls back to plain text.

**Client handling:** Use one of the supported format identifiers: `md`, `html`, `json`, `csv`, `yaml`, `xml`, `rst`, `ipynb`, `docx`, `png`, `jpg`, `gif`, `webp`, `txt`.

---

### `ENGINE_ERR_UNAUTHORIZED`
| Field | Value |
|---|---|
| HTTP status | 401 |
| Stage | `input` |

**When triggered:** The `ANNAVE_INTERNAL_TOKEN` environment variable is set on the server and the request either omits the `X-Internal-Token` header or provides the wrong value.

**Client handling:** Include the correct token in the `X-Internal-Token` request header. In development (token not set), this error never fires.

---

### `ENGINE_ERR_RATE_LIMITED`
| Field | Value |
|---|---|
| HTTP status | 429 |
| Stage | `input` |

**When triggered:** The per-IP request rate exceeds the configured limit. The response includes a `Retry-After` header with the number of seconds to wait.

**Client handling:** Honour the `Retry-After` header. Implement exponential backoff for automated clients.

*Note: rate limiting is configured in `config/server.yaml` and enforced per-IP with a sliding window.*

---

### `ENGINE_ERR_PARSE_FAILED`
| Field | Value |
|---|---|
| HTTP status | 422 |
| Stage | `parser` |

**When triggered:** A parser was selected (by explicit format or auto-detection) but returned an error. Most parsers are lenient — this error typically occurs with binary formats (DOCX, image) that are malformed or truncated.

**Client handling:** Verify the file is not corrupt. For DOCX files, try resaving from the source application. For images, verify the file header.

---

### `ENGINE_ERR_INVALID_DOCUMENT`
| Field | Value |
|---|---|
| HTTP status | 422 |
| Stage | `validation` |

**When triggered:** The parsed document AST fails structural validation — the root node has the wrong type, or the document is nil. This indicates a bug in a parser, not a problem with the user's input.

**Client handling:** Report at the issue tracker. Include the input format and a minimal reproduction.

---

### `ENGINE_ERR_INVALID_NODE`
| Field | Value |
|---|---|
| HTTP status | 422 |
| Stage | `validation` |

**When triggered:** A block-level node has an unknown type, or a required field (text, items, headers, src) is missing. The error message includes the node index. This indicates a parser bug.

**Client handling:** Report at the issue tracker.

---

### `ENGINE_ERR_TOO_MANY_NODES`
| Field | Value |
|---|---|
| HTTP status | 422 |
| Stage | `validation` |
| Config key | `document.max_nodes` in `config/limits.yaml` |

**When triggered:** The parsed document has more block-level nodes than the configured limit (default 2,000). A node is one of: paragraph, heading, list, table, code block, blockquote, horizontal rule, or image.

**Client handling:** Split the document into smaller sections. If the document is legitimately large (technical manual, legal document), ask the operator to increase `max_nodes`.

---

### `ENGINE_ERR_TOO_MANY_PAGES`
| Field | Value |
|---|---|
| HTTP status | 422 |
| Stage | `pagination` |
| Config key | `document.max_pages` in `config/limits.yaml` |

**When triggered:** After layout and pagination, the document produces more pages than the configured limit (default 100). The error message includes the actual page count.

**Client handling:** Split the document. If the document is legitimately long (book-length), ask the operator to increase `max_pages`.

---

### `ENGINE_ERR_RENDERER_INIT`
| Field | Value |
|---|---|
| HTTP status | 500 |
| Stage | `render` |

**When triggered:** The PDF renderer (`gopdf`) failed to initialise. This is a server-side infrastructure problem — usually a missing or corrupt embedded font.

**Client handling:** Retry after a short delay. If the error persists, report it. This should not occur in a correctly built binary.

---

### `ENGINE_ERR_RENDER_FAILED`
| Field | Value |
|---|---|
| HTTP status | 500 |
| Stage | `render` |

**When triggered:** The renderer initialised successfully but failed while writing the PDF. Usually caused by an unexpected node type that reached the render stage without being caught by validation.

**Client handling:** Retry. If the error persists with a specific document, report it with the input format and a minimal reproduction.

---

### `ENGINE_ERR_INTERNAL`
| Field | Value |
|---|---|
| HTTP status | 500 |
| Stage | `render` |

**When triggered:** An unexpected error occurred that did not match any known error type. The error message includes the raw Go error string for diagnosis.

**Client handling:** Report at the issue tracker with the request ID (from the `X-Request-Id` response header) and the full error message.

---

## Success event (log only)

### `ENGINE_OK_CONVERTED`

Not returned in API responses. Emitted as a structured log line when a document is converted successfully. Useful for monitoring and alerting.

---

## Summary table

| Code | HTTP | Stage |
|---|---|---|
| `ENGINE_ERR_FILE_TOO_LARGE` | 400 | input |
| `ENGINE_ERR_EMPTY_INPUT` | 400 | input |
| `ENGINE_ERR_INPUT_TOO_LARGE` | 400 | input |
| `ENGINE_ERR_UNSUPPORTED_FORMAT` | 400 | input |
| `ENGINE_ERR_UNAUTHORIZED` | 401 | input |
| `ENGINE_ERR_RATE_LIMITED` | 429 | input |
| `ENGINE_ERR_PARSE_FAILED` | 422 | parser |
| `ENGINE_ERR_INVALID_DOCUMENT` | 422 | validation |
| `ENGINE_ERR_INVALID_NODE` | 422 | validation |
| `ENGINE_ERR_TOO_MANY_NODES` | 422 | validation |
| `ENGINE_ERR_TOO_MANY_PAGES` | 422 | pagination |
| `ENGINE_ERR_RENDERER_INIT` | 500 | render |
| `ENGINE_ERR_RENDER_FAILED` | 500 | render |
| `ENGINE_ERR_INTERNAL` | 500 | render |
