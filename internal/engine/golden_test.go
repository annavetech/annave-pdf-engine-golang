// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"fmt"
	"os"
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
// byte-compares the result against the committed testdata/golden.pdf. Font
// registration order in NewRenderer used to depend on Go's randomised map
// iteration, which made gopdf's object numbering non-deterministic; that was
// fixed so this comparison is meaningful rather than flaky. A single wrong
// character transcribed while hoisting a regex to package scope changes the
// parsed AST and therefore the rendered bytes, which this test is built to
// catch.
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

	if !bytes.Equal(got, want) {
		t.Fatal(mismatchReport(got, want))
	}
}

// mismatchReport locates the first byte at which got and want diverge and
// renders a printable report naming the offset and the bytes on each side,
// rather than letting the test framework print two large opaque blobs.
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
				"rendered PDF does not match %s: first divergence at byte offset %d (got %d bytes, want %d bytes)\n  got:  %q\n  want: %q",
				goldenPdfPath, i, len(got), len(want), got[lo:hi], want[lo:hi],
			)
		}
	}
	return fmt.Sprintf(
		"rendered PDF does not match %s: identical up to byte %d, but lengths differ: got %d bytes, want %d bytes",
		goldenPdfPath, n, len(got), len(want),
	)
}
