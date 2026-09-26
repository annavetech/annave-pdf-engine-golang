// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/annavetech/annave-pdf-engine-golang/internal/ast"
)

// buildDocxWithImages returns a minimal, valid .docx archive containing one
// paragraph per entry in imageSizes, each referencing an embedded image of the given size in bytes.
func buildDocxWithImages(t testing.TB, imageSizes []int) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	var paras strings.Builder
	var rels strings.Builder
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := range imageSizes {
		rID := fmt.Sprintf("rId%d", i+1)
		mediaName := fmt.Sprintf("media/image%d.bin", i+1)
		paras.WriteString(fmt.Sprintf(`<w:p><w:drawing><blip embed=%q/></w:drawing></w:p>`, rID))
		rels.WriteString(fmt.Sprintf(
			`<Relationship Id=%q Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target=%q/>`,
			rID, mediaName))
	}
	rels.WriteString(`</Relationships>`)

	docXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` +
		paras.String() + `</w:body></w:document>`

	mustZipWrite(t, zw, "word/document.xml", []byte(docXML))
	mustZipWrite(t, zw, "word/_rels/document.xml.rels", []byte(rels.String()))

	const chunkSize = 1 << 20 // 1 MiB, reused across writes rather than allocated per entry
	pattern := bytes.Repeat([]byte{'A'}, chunkSize)
	for i, size := range imageSizes {
		w, err := zw.Create(fmt.Sprintf("word/media/image%d.bin", i+1))
		if err != nil {
			t.Fatalf("create media entry: %v", err)
		}
		remaining := size
		for remaining > 0 {
			n := chunkSize
			if n > remaining {
				n = remaining
			}
			if _, err := w.Write(pattern[:n]); err != nil {
				t.Fatalf("write media entry: %v", err)
			}
			remaining -= n
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

// buildDocxWithFillerEntries is buildDocxWithImages, plus n additional
// small, unrelated zip entries not referenced by any relationship.
func buildDocxWithFillerEntries(t testing.TB, imageSizes []int, fillerEntries int) []byte {
	t.Helper()
	base := buildDocxWithImages(t, imageSizes)

	zr, err := zip.NewReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatalf("re-open base archive: %v", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open base entry %q: %v", f.Name, err)
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("recreate entry %q: %v", f.Name, err)
		}
		// Copies exactly the entry's declared size.
		if _, err := io.CopyN(w, rc, int64(f.UncompressedSize64)); err != nil { //nolint:gosec // fixed-size copy, size taken from the entry itself
			t.Fatalf("copy entry %q: %v", f.Name, err)
		}
		_ = rc.Close()
	}
	for i := 0; i < fillerEntries; i++ {
		mustZipWrite(t, zw, fmt.Sprintf("word/filler/unused%d.xml", i), []byte("<x/>"))
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close filler archive: %v", err)
	}
	return buf.Bytes()
}

func mustZipWrite(t testing.TB, zw *zip.Writer, name string, data []byte) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create %q: %v", name, err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write %q: %v", name, err)
	}
}

// TestDocxParser_Parse_EntryAtCapSucceeds proves a legitimate embedded
// image exactly at the per-entry cap still converts.
func TestDocxParser_Parse_EntryAtCapSucceeds(t *testing.T) {
	data := buildDocxWithImages(t, []int{int(maxDocxEntryBytes)})

	p := &DocxParser{}
	if !p.CanParse(data) {
		t.Fatalf("CanParse() = false, want true")
	}
	doc, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error: %v, want a successful conversion at exactly the per-entry cap", err)
	}
	if len(doc.Children) != 1 || doc.Children[0].Type != ast.TypeImage {
		t.Fatalf("expected a single image node, got %+v", doc.Children)
	}
	if got := len(doc.Children[0].Data); got != int(maxDocxEntryBytes) {
		t.Fatalf("image Data length = %d, want %d (the per-entry cap)", got, maxDocxEntryBytes)
	}
}

// TestDocxParser_Parse_EntryOverCapFails proves the per-entry cap fires
// with a clear, structured error naming the cap, not a silent truncation.
func TestDocxParser_Parse_EntryOverCapFails(t *testing.T) {
	over := int(maxDocxEntryBytes) + 1024
	data := buildDocxWithImages(t, []int{over})

	p := &DocxParser{}
	_, err := p.Parse(data)
	if err == nil {
		t.Fatalf("Parse() error = nil, want an error for an entry exceeding the %d byte per-entry cap", maxDocxEntryBytes)
	}
	if !strings.Contains(err.Error(), "per-entry cap") {
		t.Fatalf("Parse() error = %q, want it to clearly name the per-entry cap", err.Error())
	}
}

// TestDocxParser_Parse_CumulativeUnderCapSucceeds proves several legitimate
// embedded images, together well under the cumulative cap, still convert.
func TestDocxParser_Parse_CumulativeUnderCapSucceeds(t *testing.T) {
	const perImage = 30 * 1024 * 1024 // 30 MiB × 3 = 90 MiB, under the 150 MiB cumulative cap
	data := buildDocxWithImages(t, []int{perImage, perImage, perImage})

	p := &DocxParser{}
	doc, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error: %v, want a successful conversion under the cumulative cap", err)
	}
	if len(doc.Children) != 3 {
		t.Fatalf("expected 3 image nodes, got %d", len(doc.Children))
	}
	for i, n := range doc.Children {
		if n.Type != ast.TypeImage || len(n.Data) != perImage {
			t.Fatalf("image %d: type=%q len=%d, want type=%q len=%d", i, n.Type, len(n.Data), ast.TypeImage, perImage)
		}
	}
}

// TestDocxParser_Parse_CumulativeOverCapFails proves the cumulative cap
// fires even when every individual entry is under the per-entry cap.
func TestDocxParser_Parse_CumulativeOverCapFails(t *testing.T) {
	const perImage = 40 * 1024 * 1024 // 40 MiB, well under the 50 MiB per-entry cap
	// 4 × 40 MiB = 160 MiB, over the 150 MiB cumulative cap.
	data := buildDocxWithImages(t, []int{perImage, perImage, perImage, perImage})

	p := &DocxParser{}
	_, err := p.Parse(data)
	if err == nil {
		t.Fatalf("Parse() error = nil, want an error once cumulative uncompressed bytes exceed %d", maxDocxTotalBytes)
	}
	if !strings.Contains(err.Error(), "cumulative") {
		t.Fatalf("Parse() error = %q, want it to clearly name the cumulative cap", err.Error())
	}
}

// TestDocxParser_CanParse_DoesNotEnforceCaps confirms CanParse never reads
// entry content, so it is unaffected by the size caps that apply to Parse.
func TestDocxParser_CanParse_DoesNotEnforceCaps(t *testing.T) {
	data := buildDocxWithImages(t, []int{int(maxDocxEntryBytes) + 1024})
	p := &DocxParser{}
	if !p.CanParse(data) {
		t.Fatalf("CanParse() = false, want true: CanParse must not read (and so must not reject) oversized entry content")
	}
}

// BenchmarkDocxParser_Parse_ManyEntries measures Parse cost on an archive
// with many filler entries alongside the images actually referenced.
func BenchmarkDocxParser_Parse_ManyEntries(b *testing.B) {
	imageSizes := make([]int, 20)
	for i := range imageSizes {
		imageSizes[i] = 4096
	}

	for _, fillerEntries := range []int{100, 4000} {
		data := buildDocxWithFillerEntries(b, imageSizes, fillerEntries)
		p := &DocxParser{}
		b.Run(fmt.Sprintf("entries=%d", fillerEntries+2), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := p.Parse(data); err != nil {
					b.Fatalf("Parse() error: %v", err)
				}
			}
		})
	}
}
