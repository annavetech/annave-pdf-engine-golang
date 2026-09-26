// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"github.com/annavetech/annave-pdf-engine-golang/internal/ast"
)

// TestCsvParser_Parse_QuotedMultiLineCell proves a quoted cell containing
// an embedded newline is parsed as one cell spanning two lines, not split into two rows.
func TestCsvParser_Parse_QuotedMultiLineCell(t *testing.T) {
	// A spreadsheet export shape: header row, then one data row whose
	// "Notes" cell was typed across two lines and quoted by the exporter.
	input := "Name,Notes,City\n" +
		"Alice,\"Line one\nLine two\",NYC\n" +
		"Bob,Fine,LA\n"

	p := &CsvParser{}
	doc, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(doc.Children) != 1 || doc.Children[0].Type != ast.TypeTable {
		t.Fatalf("expected a single table node, got %+v", doc.Children)
	}
	table := doc.Children[0]

	wantHeaders := []string{"Name", "Notes", "City"}
	if !equalStrings(table.Headers, wantHeaders) {
		t.Fatalf("Headers = %#v, want %#v", table.Headers, wantHeaders)
	}

	// Assert exactly one data row, with the quoted cell containing the
	// literal embedded newline.
	if len(table.Rows) != 2 {
		t.Fatalf("expected 2 data rows (the multi-line cell must not split into an extra row), got %d: %#v",
			len(table.Rows), table.Rows)
	}
	wantRow0 := []string{"Alice", "Line one\nLine two", "NYC"}
	if !equalStrings(table.Rows[0], wantRow0) {
		t.Fatalf("Rows[0] = %#v, want %#v", table.Rows[0], wantRow0)
	}
	wantRow1 := []string{"Bob", "Fine", "LA"}
	if !equalStrings(table.Rows[1], wantRow1) {
		t.Fatalf("Rows[1] = %#v, want %#v", table.Rows[1], wantRow1)
	}
}

// TestCsvParser_Parse_NonASCII proves non-ASCII characters survive CSV
// parsing byte-for-byte instead of coming out as mojibake.
func TestCsvParser_Parse_NonASCII(t *testing.T) {
	input := "Name,City,Note\n" +
		"café,Kraków,naïve\n" +
		"田中太郎,東京,日本語のテキスト\n"

	p := &CsvParser{}
	doc, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	table := doc.Children[0]

	wantHeaders := []string{"Name", "City", "Note"}
	if !equalStrings(table.Headers, wantHeaders) {
		t.Fatalf("Headers = %#v, want %#v", table.Headers, wantHeaders)
	}
	if len(table.Rows) != 2 {
		t.Fatalf("expected 2 data rows, got %d: %#v", len(table.Rows), table.Rows)
	}
	wantRow0 := []string{"café", "Kraków", "naïve"}
	if !equalStrings(table.Rows[0], wantRow0) {
		t.Fatalf("Rows[0] = %#v, want %#v (mojibake regression)", table.Rows[0], wantRow0)
	}
	wantRow1 := []string{"田中太郎", "東京", "日本語のテキスト"}
	if !equalStrings(table.Rows[1], wantRow1) {
		t.Fatalf("Rows[1] = %#v, want %#v (mojibake regression)", table.Rows[1], wantRow1)
	}
}

// TestCsvParser_Parse_RowLongerThanHeader proves a data row with more
// fields than the header row keeps every field, extending Headers to match.
func TestCsvParser_Parse_RowLongerThanHeader(t *testing.T) {
	input := "Name,Age\n" +
		"Alice,30,extra-value\n"

	p := &CsvParser{}
	doc, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	table := doc.Children[0]

	wantHeaders := []string{"Name", "Age", ""}
	if !equalStrings(table.Headers, wantHeaders) {
		t.Fatalf("Headers = %#v, want %#v (extended to the widest row, not left at the original 2)", table.Headers, wantHeaders)
	}
	if len(table.Rows) != 1 {
		t.Fatalf("expected 1 data row, got %d: %#v", len(table.Rows), table.Rows)
	}
	wantRow := []string{"Alice", "30", "extra-value"}
	if !equalStrings(table.Rows[0], wantRow) {
		t.Fatalf("Rows[0] = %#v, want %#v: the extra field must survive, not be silently dropped", table.Rows[0], wantRow)
	}
}

// TestCsvParser_Parse_RowShorterThanHeader proves a data row with fewer
// fields than the header row is padded with empty cells, not rejected.
func TestCsvParser_Parse_RowShorterThanHeader(t *testing.T) {
	input := "Name,Age,City\n" +
		"Bob,25\n"

	p := &CsvParser{}
	doc, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	table := doc.Children[0]

	wantHeaders := []string{"Name", "Age", "City"}
	if !equalStrings(table.Headers, wantHeaders) {
		t.Fatalf("Headers = %#v, want %#v", table.Headers, wantHeaders)
	}
	if len(table.Rows) != 1 {
		t.Fatalf("expected 1 data row, got %d: %#v", len(table.Rows), table.Rows)
	}
	wantRow := []string{"Bob", "25", ""}
	if !equalStrings(table.Rows[0], wantRow) {
		t.Fatalf("Rows[0] = %#v, want %#v (padded with an empty trailing cell)", table.Rows[0], wantRow)
	}
}

// TestCsvParser_Parse_RaggedRowsMixedWidths proves a CSV file with rows of
// different widths loses no data and produces a rectangular table.
func TestCsvParser_Parse_RaggedRowsMixedWidths(t *testing.T) {
	input := "Name,Age,City\n" +
		"Short,1\n" +
		"Exact,2,Town\n" +
		"Long,3,Village,extra1,extra2\n"

	p := &CsvParser{}
	doc, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	table := doc.Children[0]

	// The widest row has 5 fields, so Headers must be extended from 3 to 5.
	wantHeaders := []string{"Name", "Age", "City", "", ""}
	if !equalStrings(table.Headers, wantHeaders) {
		t.Fatalf("Headers = %#v, want %#v", table.Headers, wantHeaders)
	}
	if len(table.Rows) != 3 {
		t.Fatalf("expected 3 data rows, got %d: %#v", len(table.Rows), table.Rows)
	}

	wantRows := [][]string{
		{"Short", "1", "", "", ""},
		{"Exact", "2", "Town", "", ""},
		{"Long", "3", "Village", "extra1", "extra2"},
	}
	for i, want := range wantRows {
		if !equalStrings(table.Rows[i], want) {
			t.Fatalf("Rows[%d] = %#v, want %#v", i, table.Rows[i], want)
		}
	}

	// Every row must be exactly as wide as Headers: the renderer indexes
	// columns positionally and relies on this being rectangular.
	for i, row := range table.Rows {
		if len(row) != len(table.Headers) {
			t.Fatalf("Rows[%d] has %d cells, want %d (== len(Headers)): %#v", i, len(row), len(table.Headers), row)
		}
	}
}

// TestCsvParser_CanParse_NonASCIIHeader confirms the detection path (which
// only ever looks at the first line) is equally byte-safe.
func TestCsvParser_CanParse_NonASCIIHeader(t *testing.T) {
	p := &CsvParser{}
	if !p.CanParse([]byte("café,ville\nx,y\n")) {
		t.Fatalf("CanParse() = false, want true for a valid two-column CSV with a non-ASCII header")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
