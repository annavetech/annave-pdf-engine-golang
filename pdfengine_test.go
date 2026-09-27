// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package pdfengine_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	pdfengine "github.com/annavetech/annave-pdf-engine-golang"
)

// mustNewEngine builds an Engine with opts, failing the test immediately on
// error.
func mustNewEngine(t *testing.T, opts ...pdfengine.Option) *pdfengine.Engine {
	t.Helper()
	e, err := pdfengine.New(opts...)
	if err != nil {
		t.Fatalf("pdfengine.New() error: %v", err)
	}
	return e
}

// pdfPrefix is the byte sequence every valid PDF starts with.
var pdfPrefix = []byte("%PDF-")

func TestConvert_AllFormats(t *testing.T) {
	cases := []struct {
		name  string
		input string
		f     pdfengine.Format
	}{
		{"markdown", "# ANNÁVE PDF Engine\n\nConverts documents to PDF.", pdfengine.FormatMarkdown},
		{"text", "This is a plain text report.\n\nIt has two paragraphs.", pdfengine.FormatText},
		{"json", `{"type":"document","children":[{"type":"heading","level":1,"text":"Report"}]}`, pdfengine.FormatJSON},
		{"html", "<h1>Report</h1><p>A paragraph.</p>", pdfengine.FormatHTML},
		{"csv", "Name,Role\nAnna,Engineer", pdfengine.FormatCSV},
		{"yaml", "title: Report\nbody: A single paragraph.", pdfengine.FormatYAML},
		{"xml", "<root><title>Report</title><body>A paragraph.</body></root>", pdfengine.FormatXML},
		{"rst", "Report\n======\n\nA paragraph.\n", pdfengine.FormatRST},
		{"notebook", `{"cells":[{"cell_type":"markdown","source":["# Report"]}],"nbformat":4}`, pdfengine.FormatNotebook},
	}

	e := mustNewEngine(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pdf, err := e.Convert(context.Background(), []byte(tc.input), tc.f)
			if err != nil {
				t.Fatalf("Convert(%s) error: %v", tc.name, err)
			}
			if !bytes.HasPrefix(pdf, pdfPrefix) {
				t.Fatalf("Convert(%s): output is not a PDF", tc.name)
			}
		})
	}
}

func TestConvert_Word(t *testing.T) {
	e := mustNewEngine(t)
	pdf, err := e.Convert(context.Background(), minimalDocx(t), pdfengine.FormatWord)
	if err != nil {
		t.Fatalf("Convert(word) error: %v", err)
	}
	if !bytes.HasPrefix(pdf, pdfPrefix) {
		t.Fatal("Convert(word): output is not a PDF")
	}
}

func TestConvert_Image(t *testing.T) {
	e := mustNewEngine(t)
	pdf, err := e.Convert(context.Background(), minimalPNG(t), pdfengine.FormatImage)
	if err != nil {
		t.Fatalf("Convert(image) error: %v", err)
	}
	if !bytes.HasPrefix(pdf, pdfPrefix) {
		t.Fatal("Convert(image): output is not a PDF")
	}
}

func TestConvert_AutoDetection(t *testing.T) {
	e := mustNewEngine(t)
	pdf, err := e.Convert(context.Background(), []byte("# Heading\n\nA paragraph."), pdfengine.FormatAuto)
	if err != nil {
		t.Fatalf("Convert(auto) error: %v", err)
	}
	if !bytes.HasPrefix(pdf, pdfPrefix) {
		t.Fatal("Convert(auto): output is not a PDF")
	}
}

// TestConvert_StyleOptionTakesEffect proves WithStyle, applied at
// construction, changes rendered output.
func TestConvert_StyleOptionTakesEffect(t *testing.T) {
	input := []byte("# Heading\n\nA paragraph of body text.")

	defaultEngine := mustNewEngine(t)
	base, err := defaultEngine.Convert(context.Background(), input, pdfengine.FormatMarkdown)
	if err != nil {
		t.Fatalf("Convert(base) error: %v", err)
	}

	fontSize := 48.0
	styledEngine := mustNewEngine(t, pdfengine.WithStyle(pdfengine.Style{
		Heading1: &pdfengine.TextStyle{FontSize: &fontSize},
	}))
	styled, err := styledEngine.Convert(context.Background(), input, pdfengine.FormatMarkdown)
	if err != nil {
		t.Fatalf("Convert(styled) error: %v", err)
	}

	if bytes.Equal(base, styled) {
		t.Fatal("WithStyle produced identical output to the default style")
	}
}

// TestNew_TwoEnginesHoldIndependentConfig proves two Engine values built
// from different style files produce different output for the same input.
func TestNew_TwoEnginesHoldIndependentConfig(t *testing.T) {
	dir := t.TempDir()
	stylePath := filepath.Join(dir, "style.yaml")
	if err := os.WriteFile(stylePath, []byte(narrowMarginStyleYAML), 0o600); err != nil {
		t.Fatalf("write style fixture: %v", err)
	}

	defaultEngine := mustNewEngine(t)
	customEngine := mustNewEngine(t, pdfengine.WithStyleFile(stylePath))

	input := []byte("# Heading\n\nA paragraph of body text.")
	base, err := defaultEngine.Convert(context.Background(), input, pdfengine.FormatMarkdown)
	if err != nil {
		t.Fatalf("Convert(default) error: %v", err)
	}
	custom, err := customEngine.Convert(context.Background(), input, pdfengine.FormatMarkdown)
	if err != nil {
		t.Fatalf("Convert(custom) error: %v", err)
	}

	if bytes.Equal(base, custom) {
		t.Fatal("two Engines built from different style files produced identical output")
	}
}

// TestNew_MalformedStyleFileReturnsError proves a broken caller-supplied
// config file surfaces as an ordinary error from New, never a killed process.
func TestNew_MalformedStyleFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	stylePath := filepath.Join(dir, "style.yaml")
	if err := os.WriteFile(stylePath, []byte("not: [valid, yaml"), 0o600); err != nil {
		t.Fatalf("write style fixture: %v", err)
	}

	_, err := pdfengine.New(pdfengine.WithStyleFile(stylePath))
	if err == nil {
		t.Fatal("expected an error for a malformed style file, got nil")
	}
}

// TestNew_NoOptionsTouchesNoFilesystemPath proves New() with no options
// succeeds even when the working directory has no config files.
func TestNew_NoOptionsTouchesNoFilesystemPath(t *testing.T) {
	empty := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(empty); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore Chdir: %v", err)
		}
	}()

	if _, err := pdfengine.New(); err != nil {
		t.Fatalf("New() error with an empty working directory: %v", err)
	}
}

func TestConvert_InvalidInputReturnsInspectableError(t *testing.T) {
	e := mustNewEngine(t)
	huge := bytes.Repeat([]byte("x"), 8*1024*1024) // exceeds the default 5 MB input limit

	_, err := e.Convert(context.Background(), huge, pdfengine.FormatText)
	if err == nil {
		t.Fatal("expected an error for oversized input, got nil")
	}

	var pe *pdfengine.Error
	if !errors.As(err, &pe) {
		t.Fatalf("errors.As(err, *pdfengine.Error) failed for: %v", err)
	}
	if pe.Code == "" {
		t.Error("Error.Code is empty")
	}
	if pe.Stage != pdfengine.StageInput {
		t.Errorf("Error.Stage = %q, want %q", pe.Stage, pdfengine.StageInput)
	}
}

func TestConvert_EmptyInput(t *testing.T) {
	e := mustNewEngine(t)
	if _, err := e.Convert(context.Background(), nil, pdfengine.FormatAuto); err != nil {
		t.Errorf("Convert(nil) returned an error, want nil: %v", err)
	}
}

func TestConvert_CancelledContext(t *testing.T) {
	e := mustNewEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.Convert(ctx, []byte("# Heading"), pdfengine.FormatMarkdown)
	if err == nil {
		t.Fatal("expected an error for an already-cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected errors.Is(err, context.Canceled), got: %v", err)
	}
}

// narrowMarginStyleYAML is a complete, valid config/style.yaml with a
// distinctly different heading1 font size than the embedded default.
const narrowMarginStyleYAML = `
page:
  width_px:         794
  height_px:        1123
  margin_x_px:      56
  margin_top_px:    48
  margin_bottom_px: 48

fonts:
  sans: '"Inter", sans-serif'
  mono: '"JetBrains Mono", monospace'

text_styles:
  heading1:
    font_family:    sans
    font_size_px:   60
    font_weight:    "800"
    font_style:     normal
    line_height:    1.1
    margin_bottom_px: 20
    color:          "#1d1d1f"
  heading2:
    font_family:    sans
    font_size_px:   20
    font_weight:    "700"
    font_style:     normal
    line_height:    1.2
    margin_bottom_px: 16
    color:          "#1d1d1f"
  heading3:
    font_family:    sans
    font_size_px:   15
    font_weight:    "600"
    font_style:     normal
    line_height:    1.3
    margin_bottom_px: 12
    color:          "#1d1d1f"
  paragraph:
    font_family:    sans
    font_size_px:   13
    font_weight:    "400"
    font_style:     normal
    line_height:    1.65
    margin_bottom_px: 12
    color:          "#3a3a3c"
  code:
    font_family:    mono
    font_size_px:   11
    font_weight:    "400"
    font_style:     normal
    line_height:    1.6
    margin_bottom_px: 12
    color:          "#1d1d1f"
  blockquote:
    font_family:    sans
    font_size_px:   13
    font_weight:    "400"
    font_style:     italic
    line_height:    1.65
    margin_bottom_px: 12
    color:          "#3a3a3c"
`

// minimalDocx builds the smallest ZIP archive the Word parser accepts: a
// word/document.xml containing one paragraph.
func minimalDocx(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("create document.xml: %v", err)
	}
	const body = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>A minimal Word document.</w:t></w:r></w:p>
  </w:body>
</w:document>`
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatalf("write document.xml: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

// minimalPNG builds a small standard 8-bit RGBA PNG.
func minimalPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return buf.Bytes()
}
