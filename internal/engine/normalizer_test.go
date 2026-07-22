// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"strings"
	"testing"
)

func TestNormalizeInput(t *testing.T) {
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
			input: "Héllo wörld — こんにちは",
			want:  "Héllo wörld — こんにちは",
		},
		{
			name:    "rejects input exceeding max character count",
			input:   strings.Repeat("x", appLimits.Input.MaxInputChars+1),
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
			got, err := NormalizeInput(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("NormalizeInput() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("NormalizeInput()\ngot:  %q\nwant: %q", got, tc.want)
			}
		})
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
