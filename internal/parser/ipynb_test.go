// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"
)

// TestIpynbParser_CanParse checks that only a JSON object with a cells key and a numeric nbformat is claimed.
func TestIpynbParser_CanParse(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"cells and nbformat", `{"cells": [], "nbformat": 4}`, true},
		{"nbformat first", `{"nbformat": 4, "nbformat_minor": 5, "cells": []}`, true},
		{"null cells", `{"cells": null, "nbformat": 4}`, true},
		{"key case as in Parse", `{"Cells": 1, "NBFORMAT": 4}`, true},
		{"leading whitespace", " \n{\"cells\": [], \"nbformat\": 4}", true},
		{"cells only", `{"cells": []}`, false},
		{"nbformat only", `{"nbformat": 4}`, false},
		{"mixed-case cells only", `{"title":"Q3 report","Cells":[1]}`, false},
		{"nbformat string", `{"cells": [], "nbformat": "4"}`, false},
		{"nbformat null", `{"cells": [], "nbformat": null}`, false},
		{"nbformat object", `{"cells": [], "nbformat": {"v": 4}}`, false},
		{"other object", `{"a": 1}`, false},
		{"nested key only", `{"a": {"cells": [], "nbformat": 4}}`, false},
		{"array", `[{"cells": 1, "nbformat": 4}]`, false},
		{"truncated", `{"cells": [], "nbformat": 4`, false},
		{"markdown", "# Title\n\nBody.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (&IpynbParser{}).CanParse([]byte(tc.input)); got != tc.want {
				t.Errorf("CanParse(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestIpynbParser_CanParse_DoesNotBuildValues checks that allocations do not grow with the object size.
func TestIpynbParser_CanParse_DoesNotBuildValues(t *testing.T) {
	input := []byte(`{"items": [` + strings.Repeat(`{"id": 1, "tags": ["a", "b"]},`, 5000) + `{}]}`)
	allocs := testing.AllocsPerRun(5, func() { (&IpynbParser{}).CanParse(input) })
	if allocs > 20 {
		t.Errorf("CanParse made %v allocations on a %d-byte object, want at most 20", allocs, len(input))
	}
}

// TestIpynbParser_Parse_UndecodableReturnsError checks that a notebook Parse cannot decode is an error, not an empty document.
func TestIpynbParser_Parse_UndecodableReturnsError(t *testing.T) {
	inputs := []string{
		`{"cells": [{"cell_type": "markdown", "source": "# T"}], "nbformat": 4, "metadata": "x"}`,
		`{"cells": [1, 2], "nbformat": 4}`,
		`{"cells": "text", "nbformat": 4}`,
	}
	r := NewRegistry(testMaxInputChars)
	for _, in := range inputs {
		if _, err := (&IpynbParser{}).Parse([]byte(in)); err == nil || !strings.HasPrefix(err.Error(), "ipynb: ") {
			t.Errorf("Parse(%q) error = %v, want an error starting with %q", in, err, "ipynb: ")
		}
		for _, f := range []InputFormat{FormatAuto, FormatIPYNB} {
			if _, err := r.Parse([]byte(in), f); err == nil {
				t.Errorf("Registry.Parse(%q, %q) error = nil, want an error", in, f)
			}
		}
	}
	if _, err := (&IpynbParser{}).Parse([]byte(`{"cells": "text", "nbformat": 4}`)); err == nil || err.Error() != `ipynb: field "cells": unexpected JSON string` {
		t.Errorf("Parse() error = %v, want the field and JSON kind", err)
	}
	if _, err := r.Parse([]byte("not json"), FormatIPYNB); err == nil {
		t.Error(`Registry.Parse("not json", "ipynb") error = nil, want an error`)
	}
}
