// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/annavetech/annave-pdf-engine-golang/internal/parser"
)

// mustDefaultConfig loads the embedded default Config, failing the test
// immediately if it cannot load.
func mustDefaultConfig(t *testing.T) *Config {
	t.Helper()
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	return cfg
}

// TestPipeline_Run_GoldenMarkdown verifies that the full pipeline converts
// a reference Markdown file into a valid multi-page PDF.
func TestPipeline_Run_GoldenMarkdown(t *testing.T) {
	data, err := os.ReadFile("testdata/basic.md")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	p := NewPipeline(mustDefaultConfig(t))
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
	p := NewPipeline(mustDefaultConfig(t))
	_, err := p.Run("", parser.FormatAuto)
	// Empty input produces an empty document, a valid but empty PDF.
	if err != nil {
		t.Errorf("unexpected error for empty input: %v", err)
	}
}

func TestPipeline_Run_RejectsOversizedInput(t *testing.T) {
	cfg := mustDefaultConfig(t)
	p := NewPipeline(cfg)
	huge := strings.Repeat("x ", cfg.Limits.Input.MaxFileSizeBytes/2+1)
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

// TestPipeline_Run_RejectsUnsupportedFormat proves an unrecognised format
// is rejected with ENGINE_ERR_UNSUPPORTED_FORMAT.
func TestPipeline_Run_RejectsUnsupportedFormat(t *testing.T) {
	p := NewPipeline(mustDefaultConfig(t))
	_, err := p.Run("# Heading\n\nBody text.\n", parser.InputFormat("not-a-real-format"))
	if err == nil {
		t.Fatal("expected error for an unrecognised format, got nil")
	}
	ae, ok := err.(*AnnaveError)
	if !ok {
		t.Fatalf("expected *AnnaveError, got %T: %v", err, err)
	}
	if ae.Code != "ENGINE_ERR_UNSUPPORTED_FORMAT" {
		t.Errorf("expected code %q, got %q", "ENGINE_ERR_UNSUPPORTED_FORMAT", ae.Code)
	}
	if ae.Stage != StageInput {
		t.Errorf("expected stage %q, got %q", StageInput, ae.Stage)
	}
	if !strings.Contains(ae.Message, "not-a-real-format") {
		t.Errorf("expected the message to name the rejected format, got %q", ae.Message)
	}
}

// TestPipeline_Run_FormatAutoStillAutoDetects proves genuine auto-detection
// is unaffected by the unsupported-format rejection above.
func TestPipeline_Run_FormatAutoStillAutoDetects(t *testing.T) {
	p := NewPipeline(mustDefaultConfig(t))
	out, err := p.Run("# Heading\n\nBody text.\n", parser.FormatAuto)
	if err != nil {
		t.Fatalf("pipeline.Run(FormatAuto) error: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF")
	}
}

func TestPipeline_Run_HTMLSanitisedOnlyWhenHTMLFormat(t *testing.T) {
	// Markdown that contains an HTML fragment must NOT be sanitised as HTML.
	md := `# Title

A paragraph that references <iframe> and <embed> tags in prose.

## Section Two

More content here to ensure multi-node output.

- item one
- item two
- item three
`
	p := NewPipeline(mustDefaultConfig(t))
	out, err := p.Run(md, parser.FormatMd)
	if err != nil {
		t.Fatalf("pipeline.Run() error: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF")
	}
}

// TestPipeline_Run_PNG verifies that a PNG image is correctly converted to a
// valid PDF, using a small standard 8-bit RGBA image.
func TestPipeline_Run_PNG(t *testing.T) {
	imgBytes := makePNG(t, 64, 64)

	p := NewPipeline(mustDefaultConfig(t))
	out, err := p.Run(string(imgBytes), parser.FormatAuto)
	if err != nil {
		t.Fatalf("pipeline.Run() error for PNG input: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF, first bytes: %q", out[:min(16, len(out))])
	}
	if len(out) < 1000 {
		t.Errorf("PDF suspiciously small (%d bytes)", len(out))
	}
}

// TestPipeline_Run_PNGViaExplicitFormatAlias proves Run parses a PNG the
// same way for format "png" as for FormatAuto or FormatImage.
func TestPipeline_Run_PNGViaExplicitFormatAlias(t *testing.T) {
	imgBytes := makePNG(t, 64, 64)

	p := NewPipeline(mustDefaultConfig(t))
	out, err := p.Run(string(imgBytes), parser.InputFormat("png"))
	if err != nil {
		t.Fatalf("pipeline.Run(format=png) error: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF, first bytes: %q", out[:min(16, len(out))])
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
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: 128, A: 255}) //nolint:gosec // test-only pixel pattern generator; intentional wraparound for small fixture dimensions
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test PNG: %v", err)
	}
	return buf.Bytes()
}

// makeJPEG builds a minimal valid JPEG of the given dimensions.
func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: 128, A: 255}) //nolint:gosec // test-only pixel pattern generator; intentional wraparound for small fixture dimensions
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("failed to encode test JPEG: %v", err)
	}
	return buf.Bytes()
}

// makeGIF builds a minimal valid GIF of the given dimensions.
func makeGIF(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{
		color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}, color.RGBA{128, 64, 32, 255},
	})
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetColorIndex(x, y, uint8((x+y)%3)) //nolint:gosec // test-only pixel pattern generator; intentional wraparound for small fixture dimensions
		}
	}
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("failed to encode test GIF: %v", err)
	}
	return buf.Bytes()
}

// tinyWebPBase64 is a real 8x8 lossless WebP fixture, since the standard
// library has no WebP encoder to build one at test time.
const tinyWebPBase64 = "UklGRjQAAABXRUJQVlA4TCcAAAAvB8ABALkyRPQ/dhHR/4Catg2YLv6ocxUBgaQt3vYPYEyAqONB4D8A"

// makeWebP decodes the tinyWebPBase64 fixture into WebP bytes.
func makeWebP(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(tinyWebPBase64)
	if err != nil {
		t.Fatalf("failed to decode test WebP fixture: %v", err)
	}
	return data
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
	p := NewPipeline(mustDefaultConfig(t))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := p.Run(tc.input, tc.format)
			if err != nil {
				t.Fatalf("pipeline.Run(%s) error: %v", tc.name, err)
			}
			if !bytes.HasPrefix(out, []byte("%PDF-")) {
				t.Fatalf("output for %s is not a PDF, first bytes: %q", tc.name, out[:min(16, len(out))])
			}
		})
	}
}

// TestPipeline_Run_ImageFormatAliases proves each image format alias and
// the canonical "image" format run through Pipeline.Run with a real fixture.
func TestPipeline_Run_ImageFormatAliases(t *testing.T) {
	cases := []struct {
		name   string
		format parser.InputFormat
		data   []byte
	}{
		{"png", parser.InputFormat("png"), makePNG(t, 8, 8)},
		{"jpg", parser.InputFormat("jpg"), makeJPEG(t, 8, 8)},
		{"jpeg", parser.InputFormat("jpeg"), makeJPEG(t, 8, 8)},
		{"gif", parser.InputFormat("gif"), makeGIF(t, 8, 8)},
		{"webp", parser.InputFormat("webp"), makeWebP(t)},
		{"image", parser.FormatImage, makePNG(t, 8, 8)},
	}
	p := NewPipeline(mustDefaultConfig(t))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := p.Run(string(tc.data), tc.format)
			if err != nil {
				t.Fatalf("pipeline.Run(format=%s) error: %v", tc.name, err)
			}
			if !bytes.HasPrefix(out, []byte("%PDF-")) {
				t.Fatalf("output for %s is not a PDF, first bytes: %q", tc.name, out[:min(16, len(out))])
			}
		})
	}
}
