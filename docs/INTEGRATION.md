<!--
title:       ANNÁVE PDF Engine — Integration Guide
description: How to call the convert endpoint from Go, Node.js, Python, Swift,
             and Angular — with working code examples for each.
author:      Anna Veretennykova
website:     www.annave.tech
version:     1.1.0
created:     2026-05-06
updated:     2026-08-23
-->

# Integration Guide

The engine exposes a single HTTP endpoint: `POST /convert`. It accepts a document in any supported format and returns a PDF byte stream.

All examples assume the server is running at `http://localhost:5741`. Replace with your deployment URL in production.

---

## Endpoint reference

### `POST /convert`

**Query parameters:**
- `format` — optional; one of `md`, `html`, `json`, `csv`, `yaml`, `xml`, `rst`, `ipynb`, `docx`, `png`, `jpg`, `gif`, `webp`, `txt`, `auto`. Defaults to `auto` (auto-detection).

**Request formats:**
- Raw body: send the document content as the request body with any `Content-Type`
- Multipart form: field `file` (file upload) or field `text` (text content); optional field `format`
- URL-encoded form: field `text` and optional field `format`

**Success response:**
- HTTP 200
- `Content-Type: application/pdf`
- Body: PDF bytes

**Error response:**
- HTTP 4xx or 5xx
- `Content-Type: application/json`
- Body: `{"error": {"code": "ENGINE_ERR_*", "stage": "...", "message": "..."}}`

**Headers on all responses:**
- `X-Request-Id` — unique identifier for this request (use in bug reports)
- `X-Engine-Version` — engine version string (on successful conversions)

### `GET /health`

Returns `{"status": "ok", "version": "<engine version>"}`. No authentication required.

---

## Go

Direct library use — no HTTP required:

```go
package main

import (
    "os"

    "github.com/annavetech/annave-pdf-engine-golang/internal/engine"
    "github.com/annavetech/annave-pdf-engine-golang/internal/parser"
)

func main() {
    pipeline := engine.NewPipeline()

    markdown := `# Report\n\nFirst paragraph.\n\n## Section\n\nMore text.`
    pdf, err := pipeline.Run(markdown, parser.FormatMd)
    if err != nil {
        panic(err)
    }

    os.WriteFile("output.pdf", pdf, 0644)
}
```

Via HTTP:

```go
package main

import (
    "io"
    "net/http"
    "os"
    "strings"
)

func convertMarkdown(serverURL, markdown string) ([]byte, error) {
    resp, err := http.Post(
        serverURL+"/convert?format=md",
        "text/plain",
        strings.NewReader(markdown),
    )
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    return io.ReadAll(resp.Body)
}

func main() {
    pdf, err := convertMarkdown("http://localhost:5741", "# Hello\n\nWorld.")
    if err != nil {
        panic(err)
    }
    os.WriteFile("output.pdf", pdf, 0644)
}
```

File upload in Go:

```go
func convertFile(serverURL, filePath string) ([]byte, error) {
    data, _ := os.ReadFile(filePath)

    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    part, _ := writer.CreateFormFile("file", filepath.Base(filePath))
    part.Write(data)
    writer.Close()

    resp, err := http.Post(serverURL+"/convert", writer.FormDataContentType(), body)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    return io.ReadAll(resp.Body)
}
```

---

## Node.js

```js
const fs = require('fs');

async function convertMarkdown(serverURL, markdown) {
  const response = await fetch(`${serverURL}/convert?format=md`, {
    method: 'POST',
    headers: { 'Content-Type': 'text/plain' },
    body: markdown,
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(`${error.error.code}: ${error.error.message}`);
  }

  return Buffer.from(await response.arrayBuffer());
}

// File upload
async function convertFile(serverURL, filePath) {
  const form = new FormData();
  form.append('file', new Blob([fs.readFileSync(filePath)]), filePath);

  const response = await fetch(`${serverURL}/convert`, {
    method: 'POST',
    body: form,
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(`${error.error.code}: ${error.error.message}`);
  }

  return Buffer.from(await response.arrayBuffer());
}

// Usage
convertMarkdown('http://localhost:5741', '# Report\n\nContent here.')
  .then(pdf => fs.writeFileSync('output.pdf', pdf))
  .catch(console.error);
```

With a token (production):

```js
async function convertWithToken(serverURL, token, markdown) {
  const response = await fetch(`${serverURL}/convert?format=md`, {
    method: 'POST',
    headers: {
      'Content-Type': 'text/plain',
      'X-Internal-Token': token,
    },
    body: markdown,
  });
  // ...
}
```

---

## Python

```python
import requests

def convert_markdown(server_url: str, markdown: str) -> bytes:
    response = requests.post(
        f"{server_url}/convert",
        params={"format": "md"},
        data=markdown.encode("utf-8"),
        headers={"Content-Type": "text/plain"},
    )
    response.raise_for_status()
    return response.content

def convert_file(server_url: str, file_path: str) -> bytes:
    with open(file_path, "rb") as f:
        response = requests.post(
            f"{server_url}/convert",
            files={"file": (file_path, f)},
        )
    response.raise_for_status()
    return response.content

# Error handling
def convert_safe(server_url: str, markdown: str) -> bytes | None:
    try:
        response = requests.post(
            f"{server_url}/convert?format=md",
            data=markdown.encode("utf-8"),
            headers={"Content-Type": "text/plain"},
        )
        if not response.ok:
            error = response.json()["error"]
            print(f"Engine error {error['code']} at stage {error['stage']}: {error['message']}")
            return None
        return response.content
    except requests.RequestException as e:
        print(f"HTTP error: {e}")
        return None

# Usage
pdf = convert_markdown("http://localhost:5741", "# Title\n\nContent.")
with open("output.pdf", "wb") as f:
    f.write(pdf)
```

---

## Swift

```swift
import Foundation

struct EngineError: Codable {
    struct ErrorBody: Codable {
        let code: String
        let stage: String
        let message: String
    }
    let error: ErrorBody
}

func convertMarkdown(serverURL: URL, markdown: String) async throws -> Data {
    var request = URLRequest(url: serverURL.appendingPathComponent("convert"))
    request.httpMethod = "POST"
    request.setValue("text/plain", forHTTPHeaderField: "Content-Type")
    request.url?.append(queryItems: [URLQueryItem(name: "format", value: "md")])
    request.httpBody = markdown.data(using: .utf8)

    let (data, response) = try await URLSession.shared.data(for: request)
    guard let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode == 200 else {
        let engineError = try JSONDecoder().decode(EngineError.self, from: data)
        throw NSError(
            domain: "AnnavePDFEngine",
            code: (response as? HTTPURLResponse)?.statusCode ?? 0,
            userInfo: [NSLocalizedDescriptionKey: engineError.error.message]
        )
    }
    return data
}

// Usage
Task {
    let serverURL = URL(string: "http://localhost:5741")!
    let markdown = "# Title\n\nFirst paragraph."
    do {
        let pdf = try await convertMarkdown(serverURL: serverURL, markdown: markdown)
        try pdf.write(to: URL(fileURLWithPath: "output.pdf"))
    } catch {
        print("Conversion failed: \(error)")
    }
}
```

---

## Angular (TypeScript)

Service using `HttpClient`:

```typescript
import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders, HttpErrorResponse } from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError } from 'rxjs/operators';

export interface EngineError {
  error: {
    code: string;
    stage: string;
    message: string;
  };
}

@Injectable({ providedIn: 'root' })
export class PdfEngineService {
  private readonly baseUrl = 'http://localhost:5741';

  constructor(private http: HttpClient) {}

  convertMarkdown(markdown: string): Observable<Blob> {
    return this.http.post(
      `${this.baseUrl}/convert?format=md`,
      markdown,
      {
        headers: new HttpHeaders({ 'Content-Type': 'text/plain' }),
        responseType: 'blob',
      }
    ).pipe(catchError(this.handleError));
  }

  convertFile(file: File): Observable<Blob> {
    const form = new FormData();
    form.append('file', file);

    return this.http.post(
      `${this.baseUrl}/convert`,
      form,
      { responseType: 'blob' }
    ).pipe(catchError(this.handleError));
  }

  private handleError(error: HttpErrorResponse): Observable<never> {
    if (error.error instanceof Blob) {
      // Parse the JSON error from the blob response
      error.error.text().then(text => {
        const engineError: EngineError = JSON.parse(text);
        console.error(`Engine error ${engineError.error.code}: ${engineError.error.message}`);
      });
    }
    return throwError(() => error);
  }
}
```

Component usage with inline PDF preview:

```typescript
import { Component } from '@angular/core';
import { PdfEngineService } from './pdf-engine.service';
import { DomSanitizer, SafeResourceUrl } from '@angular/platform-browser';

@Component({
  selector: 'app-converter',
  template: `
    <textarea [(ngModel)]="markdown" rows="10"></textarea>
    <button (click)="convert()">Convert to PDF</button>
    <iframe *ngIf="pdfUrl" [src]="pdfUrl" width="100%" height="600px"></iframe>
  `
})
export class ConverterComponent {
  markdown = '';
  pdfUrl: SafeResourceUrl | null = null;

  constructor(
    private engine: PdfEngineService,
    private sanitizer: DomSanitizer
  ) {}

  convert(): void {
    this.engine.convertMarkdown(this.markdown).subscribe(blob => {
      const url = URL.createObjectURL(blob);
      this.pdfUrl = this.sanitizer.bypassSecurityTrustResourceUrl(url);
    });
  }
}
```

---

## Error handling guide

All clients should parse the `code` field from error responses rather than the `message` string. Message text may change between versions; codes are stable.

| Code | Recommended action |
|---|---|
| `ENGINE_ERR_FILE_TOO_LARGE` | Reject before upload; show file size limit to user |
| `ENGINE_ERR_EMPTY_INPUT` | Validate input before sending |
| `ENGINE_ERR_INPUT_TOO_LARGE` | Split document or show character limit to user |
| `ENGINE_ERR_UNSUPPORTED_FORMAT` | Show list of supported formats |
| `ENGINE_ERR_UNAUTHORIZED` | Check token configuration |
| `ENGINE_ERR_RATE_LIMITED` | Back off; honour `Retry-After` header |
| `ENGINE_ERR_PARSE_FAILED` | Show error message; suggest re-saving source file |
| `ENGINE_ERR_TOO_MANY_NODES` | Split document |
| `ENGINE_ERR_TOO_MANY_PAGES` | Split document |
| `ENGINE_ERR_INTERNAL` | Retry; report with `X-Request-Id` if it persists |
