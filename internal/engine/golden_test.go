// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/annavetech/annave-pdf-engine-golang/internal/parser"
)

// goldenMdPath and goldenPdfPath are the fixture and the committed reference
// output for TestPipeline_Run_GoldenPDFMatchesFixture.
const (
	goldenMdPath  = "testdata/golden.md"
	goldenPdfPath = "testdata/golden.pdf"
)

// TestPipeline_Run_GoldenPDFMatchesFixture renders testdata/golden.md and
// compares the result against the committed testdata/golden.pdf. Font
// registration order in NewRenderer used to depend on Go's randomised map
// iteration, which made gopdf's object numbering non-deterministic; that was
// fixed so this comparison is meaningful rather than flaky. A single wrong
// character transcribed while hoisting a regex to package scope changes the
// parsed AST and therefore the rendered bytes, which this test is built to
// catch.
//
// The comparison runs on a normalised form rather than raw file bytes:
// normalizePDF inflates each FlateDecode stream, since Go's compress/flate
// output is not guaranteed byte-identical across Go releases even for
// identical input, and drops the xref table and startxref value, since those
// record physical byte offsets that shift whenever any stream's compressed
// length does. Everything else — object structure, dictionary values,
// decompressed stream content — is compared as-is.
//
// To intentionally update the golden file after a change that is meant to
// alter output, run:
//
//	UPDATE_GOLDEN=1 go test ./internal/engine/ -run TestPipeline_Run_GoldenPDFMatchesFixture
//
// and commit the resulting testdata/golden.pdf. The env var guard means this
// never happens as a side effect of a normal `go test ./...`.
func TestPipeline_Run_GoldenPDFMatchesFixture(t *testing.T) {
	src, err := os.ReadFile(goldenMdPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	p := NewPipeline()
	got, err := p.Run(string(src), parser.FormatMd)
	if err != nil {
		t.Fatalf("pipeline.Run() error: %v", err)
	}

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPdfPath, got, 0o644); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
		t.Logf("updated %s (%d bytes) — review the diff before committing", goldenPdfPath, len(got))
		return
	}

	want, err := os.ReadFile(goldenPdfPath)
	if err != nil {
		t.Fatalf("read golden file: %v (regenerate with UPDATE_GOLDEN=1)", err)
	}

	gotNorm, err := normalizePDF(got)
	if err != nil {
		t.Fatalf("normalise rendered PDF: %v", err)
	}
	wantNorm, err := normalizePDF(want)
	if err != nil {
		t.Fatalf("normalise %s: %v", goldenPdfPath, err)
	}

	if !bytes.Equal(gotNorm, wantNorm) {
		t.Fatal(mismatchReport(gotNorm, wantNorm))
	}
}

// pdfObjectRe matches the start of an indirect PDF object ("N G obj"), used
// to split a PDF file's body into per-object chunks.
var pdfObjectRe = regexp.MustCompile(`(?s)\d+ \d+ obj\r?\n`)

// pdfLengthRe matches a stream dictionary's /Length entry with its numeric
// value, which normalizePDF blanks out since it is a byproduct of
// compression rather than content.
var pdfLengthRe = regexp.MustCompile(`/Length\s+(\d+)`)

// normalizePDF returns a toolchain-independent representation of a rendered
// PDF suitable for comparing against the golden fixture: every FlateDecode
// stream is inflated so that two differently-compressed encodings of the
// same content compare equal, and the xref table plus the startxref value
// are dropped, since both record physical byte offsets that shift whenever
// any stream's compressed length changes and carry no rendering
// information. Everything else in the file — object numbering, dictionary
// keys and values, decompressed stream bytes, non-flate stream bytes — is
// preserved, so a change to rendered content still produces a divergence.
func normalizePDF(data []byte) ([]byte, error) {
	xrefIdx := bytes.LastIndex(data, []byte("\nxref\n"))
	if xrefIdx == -1 {
		return nil, fmt.Errorf("no xref table found")
	}
	body := data[:xrefIdx]

	locs := pdfObjectRe.FindAllIndex(body, -1)
	if len(locs) == 0 {
		return nil, fmt.Errorf("no indirect objects found")
	}

	var out bytes.Buffer
	for i, loc := range locs {
		start := loc[0]
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		normalized, err := normalizePDFObject(body[start:end])
		if err != nil {
			return nil, fmt.Errorf("object at offset %d: %w", start, err)
		}
		out.Write(normalized)
	}
	return out.Bytes(), nil
}

// normalizePDFObject normalises a single indirect object's bytes (from its
// "N G obj" header to just before the next object, or end of file). Objects
// without a stream are returned unchanged. Objects with a stream have their
// /Length value blanked and their stream content inflated when the
// dictionary declares /FlateDecode; other streams (uncompressed, or using a
// filter other than FlateDecode, such as embedded binary font data with no
// filter at all) are carried through byte for byte.
func normalizePDFObject(obj []byte) ([]byte, error) {
	streamIdx := bytes.Index(obj, []byte("stream"))
	if streamIdx == -1 {
		return obj, nil
	}
	header := obj[:streamIdx]

	p := streamIdx + len("stream")
	var eolLen int
	switch {
	case p+1 < len(obj) && obj[p] == '\r' && obj[p+1] == '\n':
		eolLen = 2
	case p < len(obj) && obj[p] == '\n':
		eolLen = 1
	default:
		return nil, fmt.Errorf("missing end-of-line after 'stream' keyword")
	}
	dataStart := p + eolLen

	lm := pdfLengthRe.FindSubmatch(header)
	if lm == nil {
		return nil, fmt.Errorf("stream dictionary has no /Length entry")
	}
	length, err := strconv.Atoi(string(lm[1]))
	if err != nil {
		return nil, fmt.Errorf("parse /Length value: %w", err)
	}
	if dataStart+length > len(obj) {
		return nil, fmt.Errorf("/Length %d exceeds object bounds", length)
	}
	raw := obj[dataStart : dataStart+length]

	content := raw
	if bytes.Contains(header, []byte("/FlateDecode")) {
		r, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("open flate stream: %w", err)
		}
		content, err = io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("inflate stream: %w", err)
		}
		if err := r.Close(); err != nil {
			return nil, fmt.Errorf("close flate stream: %w", err)
		}
	}

	var result bytes.Buffer
	result.Write(pdfLengthRe.ReplaceAll(header, []byte("/Length -")))
	result.WriteString("stream")
	result.Write(obj[p:dataStart])
	result.Write(content)
	result.Write(obj[dataStart+length:])
	return result.Bytes(), nil
}

// mismatchReport locates the first byte at which the normalised got and
// want representations diverge and renders a printable report naming the
// offset and the bytes on each side, rather than letting the test framework
// print two large opaque blobs.
func mismatchReport(got, want []byte) string {
	n := len(got)
	if len(want) < n {
		n = len(want)
	}
	for i := 0; i < n; i++ {
		if got[i] != want[i] {
			lo, hi := i-16, i+16
			if lo < 0 {
				lo = 0
			}
			if hi > n {
				hi = n
			}
			return fmt.Sprintf(
				"rendered PDF does not match %s: first divergence at normalised byte offset %d (got %d bytes, want %d bytes)\n  got:  %q\n  want: %q",
				goldenPdfPath, i, len(got), len(want), got[lo:hi], want[lo:hi],
			)
		}
	}
	return fmt.Sprintf(
		"rendered PDF does not match %s: normalised content identical up to byte %d, but lengths differ: got %d bytes, want %d bytes",
		goldenPdfPath, n, len(got), len(want),
	)
}
