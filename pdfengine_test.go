// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package pdfengine_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	pdfengine "github.com/annavetech/annave-pdf-engine-golang"
)

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

	e := pdfengine.New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pdf, err := e.Convert(tc.input, tc.f)
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
	e := pdfengine.New()
	pdf, err := e.Convert(string(minimalDocx(t)), pdfengine.FormatWord)
	if err != nil {
		t.Fatalf("Convert(word) error: %v", err)
	}
	if !bytes.HasPrefix(pdf, pdfPrefix) {
		t.Fatal("Convert(word): output is not a PDF")
	}
}

func TestConvert_Image(t *testing.T) {
	e := pdfengine.New()
	pdf, err := e.Convert(string(minimalPNG(t)), pdfengine.FormatImage)
	if err != nil {
		t.Fatalf("Convert(image) error: %v", err)
	}
	if !bytes.HasPrefix(pdf, pdfPrefix) {
		t.Fatal("Convert(image): output is not a PDF")
	}
}

func TestConvert_AutoDetection(t *testing.T) {
	e := pdfengine.New()
	pdf, err := e.Convert("# Heading\n\nA paragraph.", pdfengine.FormatAuto)
	if err != nil {
		t.Fatalf("Convert(auto) error: %v", err)
	}
	if !bytes.HasPrefix(pdf, pdfPrefix) {
		t.Fatal("Convert(auto): output is not a PDF")
	}
}

func TestConvert_StyleOptionTakesEffect(t *testing.T) {
	e := pdfengine.New()
	input := "# Heading\n\nA paragraph of body text."

	base, err := e.Convert(input, pdfengine.FormatMarkdown)
	if err != nil {
		t.Fatalf("Convert(base) error: %v", err)
	}

	fontSize := 48.0
	styled, err := e.Convert(input, pdfengine.FormatMarkdown, pdfengine.WithStyle(pdfengine.Style{
		Heading1: &pdfengine.TextStyle{FontSize: &fontSize},
	}))
	if err != nil {
		t.Fatalf("Convert(styled) error: %v", err)
	}

	if bytes.Equal(base, styled) {
		t.Fatal("WithStyle produced identical output to the default style")
	}
}

func TestConvert_InvalidInputReturnsInspectableError(t *testing.T) {
	e := pdfengine.New()
	huge := strings.Repeat("x", 8*1024*1024) // exceeds the default 5 MB input limit

	_, err := e.Convert(huge, pdfengine.FormatText)
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
	e := pdfengine.New()
	if _, err := e.Convert("", pdfengine.FormatAuto); err != nil {
		t.Errorf("Convert(\"\") returned an error, want nil: %v", err)
	}
}

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
