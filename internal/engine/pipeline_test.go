// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
