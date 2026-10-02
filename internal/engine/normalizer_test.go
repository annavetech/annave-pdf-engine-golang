// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"strings"
	"testing"

	"github.com/annavetech/pdfengine/internal/ast"
	"github.com/annavetech/pdfengine/internal/parser"
)

func TestNormalizeInput(t *testing.T) {
	cfg := mustDefaultConfig(t)
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "normalises CRLF to LF",
			input: "line one\r\nline two\r\nline three",
			want:  "line one\nline two\nline three",
		},
		{
			name:  "normalises bare CR to LF",
			input: "line one\rline two",
			want:  "line one\nline two",
		},
		{
			name:  "collapses three consecutive blank lines to one",
			input: "a\n\n\n\nb",
			want:  "a\n\nb",
		},
		{
			name:  "preserves single blank line",
			input: "a\n\nb",
			want:  "a\n\nb",
		},
		{
			name:  "strips leading and trailing whitespace",
			input: "  \n  hello  \n  ",
			want:  "hello",
		},
		{
			name:  "strips ASCII control characters",
			input: "hello\x01\x02world",
			want:  "helloworld",
		},
		{
			name:  "preserves tabs",
			input: "col1\tcol2\tcol3",
			want:  "col1\tcol2\tcol3",
		},
		{
			name:  "passes through normal unicode",
			input: "Héllo wörld … こんにちは",
			want:  "Héllo wörld … こんにちは",
		},
		{
			name:  "strips a leading UTF-8 BOM",
			input: "\uFEFF# Title",
			want:  "# Title",
		},
		{
			name:    "rejects input exceeding max character count",
			input:   strings.Repeat("x", cfg.Limits.Input.MaxInputChars+1),
			wantErr: true,
		},
		{
			name:  "returns empty string for whitespace-only input",
			input: "   \n\n\t  ",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeInput(tc.input, cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("NormalizeInput() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("NormalizeInput()\ngot:  %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// TestNormalizeInput_BOMRoundTrip proves a BOM-prefixed file parses with no stray leading character.
func TestNormalizeInput_BOMRoundTrip(t *testing.T) {
	cfg := mustDefaultConfig(t)
	reg := parser.NewRegistry(cfg.Limits.Input.MaxInputChars)

	md, err := NormalizeInput("\uFEFF# Title\n\nBody text.", cfg)
	if err != nil {
		t.Fatalf("NormalizeInput(md) error: %v", err)
	}
	doc, err := reg.Parse([]byte(md), parser.FormatMd)
	if err != nil {
		t.Fatalf("Registry.Parse(md) error: %v", err)
	}
	if len(doc.Children) == 0 || doc.Children[0].Type != ast.TypeHeading || doc.Children[0].Text != "Title" {
		t.Errorf("Markdown first node = %+v, want heading %q", doc.Children, "Title")
	}

	csv, err := NormalizeInput("\uFEFFName,City\nAnn,Tallinn", cfg)
	if err != nil {
		t.Fatalf("NormalizeInput(csv) error: %v", err)
	}
	doc, err = reg.Parse([]byte(csv), parser.FormatCSV)
	if err != nil {
		t.Fatalf("Registry.Parse(csv) error: %v", err)
	}
	if len(doc.Children) != 1 || doc.Children[0].Type != ast.TypeTable ||
		len(doc.Children[0].Headers) == 0 || doc.Children[0].Headers[0] != "Name" {
		t.Errorf("CSV = %+v, want a table whose first header is %q", doc.Children, "Name")
	}
}

func TestSanitizeHTML(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		mustKeep []string // substrings that must appear in output
		mustDrop []string // substrings that must NOT appear in output
	}{
		{
			name:     "keeps structural heading tags",
			input:    "<h1>Title</h1><p>Body text.</p>",
			mustKeep: []string{"Title", "Body text."},
		},
		{
			name:     "strips script tags",
			input:    "<p>Safe</p><script>alert('xss')</script>",
			mustKeep: []string{"Safe"},
			mustDrop: []string{"script", "alert"},
		},
		{
			name:     "strips onclick attributes",
			input:    `<p onclick="evil()">Text</p>`,
			mustKeep: []string{"Text"},
			mustDrop: []string{"onclick", "evil"},
		},
		{
			name:     "preserves href on anchor tags",
			input:    `<a href="https://annave.tech">link</a>`,
			mustKeep: []string{"link", "annave.tech"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeHTML(tc.input)
			for _, s := range tc.mustKeep {
				if !strings.Contains(got, s) {
					t.Errorf("SanitizeHTML() output missing %q\noutput: %s", s, got)
				}
			}
			for _, s := range tc.mustDrop {
				if strings.Contains(got, s) {
					t.Errorf("SanitizeHTML() output contains forbidden %q\noutput: %s", s, got)
				}
			}
		})
	}
}
