// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"archive/zip"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/annavetech/pdfengine/internal/ast"
)

// minimalDocxWithTrailingSpace builds a valid docx (zip) archive whose
// last byte is an ASCII space, by setting the zip comment to a single space.
func minimalDocxWithTrailingSpace(t *testing.T) []byte {
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
	if err := zw.SetComment(" "); err != nil {
		t.Fatalf("set zip comment: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

// minimalPNGWithTrailingSpace builds a valid PNG and appends one extra
// 0x20 byte after it.
func minimalPNGWithTrailingSpace(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	data := buf.Bytes()
	data = append(data, ' ')
	return data
}

// assertGenuineTrapCase fails the test if trailing-whitespace trimming
// would not actually have shortened data.
func assertGenuineTrapCase(t *testing.T, data []byte) {
	t.Helper()
	if len(data) == 0 || data[len(data)-1] != ' ' {
		t.Fatalf("test fixture does not end in a whitespace byte, not a valid trap case")
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == len(data) {
		t.Fatalf("bytes.TrimSpace did not shorten the fixture; not a valid trap case")
	}
}

// TestRegistry_Parse_DocxSurvivesTrailingWhitespaceByte proves a docx
// archive whose last byte is an ASCII space round-trips intact.
func TestRegistry_Parse_DocxSurvivesTrailingWhitespaceByte(t *testing.T) {
	data := minimalDocxWithTrailingSpace(t)
	assertGenuineTrapCase(t, data)

	// Confirm the old unconditional trim would have made this archive unparseable.
	trimmed := bytes.TrimSpace(data)
	if _, err := zip.NewReader(bytes.NewReader(trimmed), int64(len(trimmed))); err == nil {
		t.Fatalf("expected the trimmed archive to be invalid (proving this is a genuine trap case), got no error")
	}

	r := NewRegistry()
	const wantText = "A minimal Word document."

	for _, format := range []InputFormat{FormatDocx, FormatAuto} {
		t.Run(string(format), func(t *testing.T) {
			doc, err := r.Parse(data, format)
			if err != nil {
				t.Fatalf("Parse(format=%s) error: %v", format, err)
			}
			// Assert the actual docx content came through, not just that
			// some node exists.
			found := false
			for _, n := range doc.Children {
				if n.Type == ast.TypeParagraph && n.Text == wantText {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("Parse(format=%s): expected a paragraph with text %q, got %+v", format, wantText, doc.Children)
			}
		})
	}
}

// TestRegistry_Parse_ImageSurvivesTrailingWhitespaceByte proves the same
// property for images: the parsed bytes are not silently shortened.
func TestRegistry_Parse_ImageSurvivesTrailingWhitespaceByte(t *testing.T) {
	data := minimalPNGWithTrailingSpace(t)
	assertGenuineTrapCase(t, data)

	r := NewRegistry()

	for _, format := range []InputFormat{FormatImage, FormatAuto} {
		t.Run(string(format), func(t *testing.T) {
			doc, err := r.Parse(data, format)
			if err != nil {
				t.Fatalf("Parse(format=%s) error: %v", format, err)
			}
			if len(doc.Children) != 1 || doc.Children[0].Type != ast.TypeImage {
				t.Fatalf("Parse(format=%s): expected a single image node, got %+v", format, doc.Children)
			}
			got := doc.Children[0].Data
			if !bytes.Equal(got, data) {
				t.Fatalf("Parse(format=%s): Data was corrupted: got %d bytes (last=%q), want %d bytes (last=%q)",
					format, len(got), lastByte(got), len(data), lastByte(data))
			}
		})
	}
}

func lastByte(b []byte) byte {
	if len(b) == 0 {
		return 0
	}
	return b[len(b)-1]
}

// TestRegistry_Parse_TextFormatsStillTrimmed confirms text formats are
// still trimmed of surrounding whitespace before parsing.
func TestRegistry_Parse_TextFormatsStillTrimmed(t *testing.T) {
	r := NewRegistry()
	doc, err := r.Parse([]byte("  \n  # Heading\n\nBody text.\n  \n"), FormatMd)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(doc.Children) == 0 {
		t.Fatalf("expected parsed content, got no nodes")
	}
	got := doc.Children[0]
	if got.Type != ast.TypeHeading || got.Text != "Heading" {
		t.Fatalf("expected the first node to be a heading with text %q, got type %q text %q",
			"Heading", got.Type, got.Text)
	}
}

// TestRegistry_Parse_UnrecognisedFormatReturnsError proves Registry.Parse
// rejects an unregistered format with *UnsupportedFormatError.
func TestRegistry_Parse_UnrecognisedFormatReturnsError(t *testing.T) {
	r := NewRegistry()
	const bogus = InputFormat("not-a-real-format")
	const src = "# Heading\n\nBody text.\n"

	_, err := r.Parse([]byte(src), bogus)
	if err == nil {
		t.Fatalf("expected an error for an unrecognised format, got nil")
	}
	var ufe *UnsupportedFormatError
	if !errors.As(err, &ufe) {
		t.Fatalf("expected *UnsupportedFormatError, got %T: %v", err, err)
	}
	if ufe.Format != bogus {
		t.Errorf("UnsupportedFormatError.Format = %q, want %q", ufe.Format, bogus)
	}

	doc, err := r.Parse([]byte(src), FormatAuto)
	if err != nil {
		t.Fatalf("Parse(FormatAuto) error: %v", err)
	}
	if len(doc.Children) == 0 || doc.Children[0].Type != ast.TypeHeading {
		t.Fatalf("Parse(FormatAuto): expected auto-detection to still recognise Markdown, got %+v", doc.Children)
	}
}

// TestRegistry_Parse_ImageFormatAliases proves --format png, jpg, jpeg,
// gif and webp are accepted and routed to the same ImageParser as "image".
func TestRegistry_Parse_ImageFormatAliases(t *testing.T) {
	r := NewRegistry()
	withTrailer := minimalPNGWithTrailingSpace(t)
	data := withTrailer[:len(withTrailer)-1] // valid PNG bytes, no trailing junk

	for _, format := range []InputFormat{"png", "jpg", "jpeg", "gif", "webp", FormatImage} {
		t.Run(string(format), func(t *testing.T) {
			if !r.SupportsFormat(format) {
				t.Fatalf("SupportsFormat(%q) = false, want true", format)
			}
			doc, err := r.Parse(data, format)
			if err != nil {
				t.Fatalf("Parse(format=%s) error: %v", format, err)
			}
			if len(doc.Children) != 1 || doc.Children[0].Type != ast.TypeImage {
				t.Fatalf("Parse(format=%s): expected a single image node, got %+v", format, doc.Children)
			}
		})
	}
}

// TestRegistry_Parse_TrulyUnknownFormatStillErrors proves a genuinely
// unknown format name still returns *UnsupportedFormatError.
func TestRegistry_Parse_TrulyUnknownFormatStillErrors(t *testing.T) {
	r := NewRegistry()
	const bogus = InputFormat("bmp")

	if r.SupportsFormat(bogus) {
		t.Fatalf("SupportsFormat(%q) = true, want false", bogus)
	}
	_, err := r.Parse([]byte("whatever"), bogus)
	var ufe *UnsupportedFormatError
	if !errors.As(err, &ufe) {
		t.Fatalf("expected *UnsupportedFormatError, got %T: %v", err, err)
	}
}
