// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"
)

// TestJsonParser_Parse_InvalidJSONReturnsError checks that malformed JSON
// is reported as an error instead of converting to an empty document.
func TestJsonParser_Parse_InvalidJSONReturnsError(t *testing.T) {
	doc, err := (&JsonParser{}).Parse([]byte(`{"title": "Report", "body": `))
	if err == nil {
		t.Fatalf("Parse returned nil error and document %+v, want an error", doc)
	}
	if doc != nil {
		t.Errorf("Parse returned document %+v alongside the error, want nil", doc)
	}
	if !strings.HasPrefix(err.Error(), "json: ") {
		t.Errorf("error %q lacks the \"json: \" prefix", err.Error())
	}
}

// nestedJSON returns an array nested depth levels deep.
func nestedJSON(depth int) []byte {
	return []byte(strings.Repeat("[", depth) + strings.Repeat("]", depth))
}

// TestJsonParser_CanParse checks that only valid JSON is claimed.
func TestJsonParser_CanParse(t *testing.T) {
	cases := []struct {
		name  string
		input []byte
		want  bool
	}{
		{"object", []byte(`{"title": "Report"}`), true},
		{"array", []byte(`[{"type": "hr"}]`), true},
		{"surrounding whitespace", []byte(" \n{\"a\": 1}\n "), true},
		{"301 levels", nestedJSON(301), true},
		{"markdown link", []byte("[Link](https://example.com) is the intro."), false},
		{"task list", []byte("[ ] todo"), false},
		{"braced word", []byte("{draft} notes"), false},
		{"truncated object", []byte(`{"title": "Report", "body": `), false},
		{"plain text", []byte("Hello world."), false},
		// Valid JSON scalars: only the first-byte check rejects these.
		{"number", []byte("42"), false},
		{"true", []byte("true"), false},
		{"null", []byte("null"), false},
		{"quoted string", []byte(`"Quoted line"`), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (&JsonParser{}).CanParse(tc.input); got != tc.want {
				t.Errorf("CanParse(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
