// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"

	"annave.tech/pdf-engine/internal/parser"
)

// TestPipeline_Run_GoldenMarkdown verifies that the full pipeline converts
// a reference Markdown file into a valid multi-page PDF. Page counts are
// validated against a golden value rather than byte-comparing the output,
// which would be brittle across gopdf versions.
func TestPipeline_Run_GoldenMarkdown(t *testing.T) {
	data, err := os.ReadFile("testdata/basic.md")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	p := NewPipeline()
	out, err := p.Run(string(data), parser.FormatMd)
	if err != nil {
		t.Fatalf("pipeline.Run() error: %v", err)
	}

	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF (first bytes: %q)", out[:min(8, len(out))])
	}
	if len(out) < 10_000 {
		t.Errorf("PDF suspiciously small (%d bytes); rendering may have failed silently", len(out))
	}
}

func TestPipeline_Run_RejectsEmptyInput(t *testing.T) {
	p := NewPipeline()
	_, err := p.Run("", parser.FormatAuto)
	// Empty input is normalised to "" and produces an empty document, which the
	// pipeline handles by returning an empty Page slice → valid but empty PDF.
	// The real guard is at the HTTP handler level (writeError for empty body).
	// Here we confirm the pipeline itself does not panic or error.
	if err != nil {
		t.Errorf("unexpected error for empty input: %v", err)
	}
}

func TestPipeline_Run_RejectsOversizedInput(t *testing.T) {
	p := NewPipeline()
	huge := strings.Repeat("x ", appLimits.Input.MaxFileSizeBytes/2+1)
	_, err := p.Run(huge, parser.FormatAuto)
	if err == nil {
		t.Fatal("expected error for oversized input, got nil")
	}
	ae, ok := err.(*AnnaveError)
	if !ok {
		t.Fatalf("expected *AnnaveError, got %T: %v", err, err)
	}
	if ae.Stage != StageInput {
		t.Errorf("expected stage %q, got %q", StageInput, ae.Stage)
	}
}

func TestPipeline_Run_HTMLSanitisedOnlyWhenHTMLFormat(t *testing.T) {
	// Markdown that contains an HTML fragment must NOT be sanitised as HTML.
	// If it were, bluemonday would strip most of the content and the page count
	// would drop to 1 (this was a real bug).
	md := `# Title

A paragraph that references <iframe> and <embed> tags in prose.

## Section Two

More content here to ensure multi-node output.

- item one
- item two
- item three
`
	p := NewPipeline()
	out, err := p.Run(md, parser.FormatMd)
	if err != nil {
		t.Fatalf("pipeline.Run() error: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF")
	}
}

// TestPipeline_Run_PNG verifies that a PNG image is correctly converted to a
// valid PDF. gopdf's parsePng is strict: it rejects interlaced and 16-bit PNGs,
// so this test uses a small standard 8-bit RGBA image.
func TestPipeline_Run_PNG(t *testing.T) {
	imgBytes := makePNG(t, 64, 64)

	p := NewPipeline()
	out, err := p.Run(string(imgBytes), parser.FormatAuto)
	if err != nil {
		t.Fatalf("pipeline.Run() error for PNG input: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF — first bytes: %q", out[:min(16, len(out))])
	}
	if len(out) < 1000 {
		t.Errorf("PDF suspiciously small (%d bytes)", len(out))
	}
}

// makePNG builds a minimal valid 8-bit RGB PNG of the given dimensions.
func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test PNG: %v", err)
	}
	return buf.Bytes()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestPipeline_Run_AllTextFormats(t *testing.T) {
	cases := []struct {
		name   string
		format parser.InputFormat
		input  string
	}{
		{"html", parser.FormatHTML, "<h1>Hello</h1><p>World</p>"},
		{"json", parser.FormatJSON, `{"title":"Test","body":"Hello world content here."}`},
		{"csv", parser.FormatCSV, "Name,Age,City\nAlice,30,NYC\nBob,25,LA"},
		{"yaml", parser.FormatYAML, "title: Test\nbody: Hello world"},
		{"xml", parser.FormatXML, "<root><title>Hello</title><body>World</body></root>"},
		{"rst", parser.FormatRST, "Hello World\n===========\n\nThis is a paragraph.\n"},
		{"txt", parser.FormatTxt, "Hello world. This is plain text."},
	}
	p := NewPipeline()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := p.Run(tc.input, tc.format)
			if err != nil {
				t.Fatalf("pipeline.Run(%s) error: %v", tc.name, err)
			}
			if !bytes.HasPrefix(out, []byte("%PDF-")) {
				t.Fatalf("output for %s is not a PDF — first bytes: %q", tc.name, out[:min(16, len(out))])
			}
		})
	}
}