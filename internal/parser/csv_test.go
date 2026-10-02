// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"github.com/annavetech/pdfengine/internal/ast"
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

// TestCsvParser_CanParse_NonASCIIHeader confirms the detection path, which
// reads the first two records, is equally byte-safe.
func TestCsvParser_CanParse_NonASCIIHeader(t *testing.T) {
	p := &CsvParser{}
	if !p.CanParse([]byte("café,ville\nx,y\n")) {
		t.Fatalf("CanParse() = false, want true for a valid two-column CSV with a non-ASCII header")
	}
}

// TestCsvParser_CanParse checks that CSV needs two records of at least 2 fields, the second no longer than the first, and no prose signs.
func TestCsvParser_CanParse(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"header and row", "Name,Role\nAnna,Engineer", true},
		{"tabs", "Name\tRole\nAnna\tEngineer", true},
		{"semicolons", "Name;Role\nAnna;Engineer", true},
		{"quoted multi-line cell", "Name,Notes\nAnna,\"one\ntwo\"", true},
		{"CRLF with blank line", "a,b\r\n\r\nc,d\r\n", true},
		{"bracketed list", "[a, b, c]", false},
		{"bracketed mixed list", "[1, 2, x]", false},
		{"one line of prose", "Hello, world.", false},
		{"header only", "Name,Role", false},
		{"prose then plain line", "Hello, world.\nGoodbye.", false},
		{"different field counts", "Hi, Anna, and team.\nThanks, see you.", false},
		{"short second row", "Event,Time,Notes\nStandup,09:00", true},
		{"short second row of two", "Name,Role,Team\nAnna,Engineer", true},
		{"short second row, semicolons", "a;b;c\nd;e", true},
		{"short second row, tabs", "a\tb\tc\nd\te", true},
		{"short second row of one field", "Event,Time,Notes\nStandup", false},
		{"long second row", "Thanks, see you.\nHi, Anna, and team.", false},
		{"prose without spaces, longer first", "Hi,Anna, and team.\nThanks,see you.", false},
		{"short prose row with a space, plain header", "a,b,c\nHello, world.", false},
		{"short row with a leading tab", "a,b,c\nd,\te", false},
		{"thousands separators in prose", "Revenue rose to 1,200,000 in 2024\nand to 950,000 in 2023.", false},
		{"thousands separators, short second line", "Totals were 12,500,000\nthen 9,000", false},
		{"thousands separators with units", "Population: 1,234,567\nArea: 2,345 km2", false},
		{"thousands separator in the second record only", "Item,Amount,Note\nabout 1,250 units", false},
		{"short numeric row", "id,count,total\n1,200", true},
		{"short numeric row of three-digit fields", "a,b,c,d\n100,200,300", true},
		{"short row, decimal first field", "x,y,z\n0.5,100", true},
		{"short row, code first field", "sku,qty,price\nA1,100", true},
		{"short row, negative first field", "id,q,r\n-5,100", true},
		{"short row, plus sign first field", "id,q,r\n+5,100", true},
		{"short row, currency first field", "id,v,w\n$5,100", true},
		{"short row, open paren first field", "id,v,w\n(5,100", true},
		{"short row, date first field", "date,val,n\n2024-01-01,100", true},
		{"short row, slash date first field", "date,val,n\n01/02,100", true},
		{"short row, time first field", "t,v,n\n09:30,100", true},
		{"short row, word first field", "name,qty,price\nApple,100", true},
		{"short row, first field ending in a bracket", "name,qty,price\nTea (green),100", true},
		{"short row, four-digit number before the separator", "In 2024,500,x\nnext,row", true},
		{"short row, four-digit group", "Total 1,2000,x\nnext,row", true},
		{"short row, two digits then text", "a,b,c\nTotal 1,20x", true},
		{"thousands separator in the first record only", "Sales hit 1,200,000\nthen,flat", false},
		{"thousands separator then a unit", "a,b,c\n1,200 kg", false},
		{"thousands separator after a dollar sign mid-field", "Costs were $1,200,000\nup,down", false},
		{"thousands separator after a hyphen mid-field", "Low was -1,200,x\nnext,row", false},
		{"short row, code with a hyphen", "sku,qty,price\nSKU-1,100", true},
		{"short row, code with an underscore", "name,qty,price\nitem_1,100", true},
		{"short row, mixed code prefix", "id,qty,price\nab-9_x-1,100", true},
		{"short row, host and port", "host,port,x\nweb-01,443", true},
		{"short row, semicolons, text before a number", "a;b;c\nPack of 6;100", true},
		{"short row, tabs, code with a hyphen", "a\tb\tc\nSKU-1\t100", true},
		{"short row, tabs, text before a number", "a\tb\tc\nPack of 6\t100", true},
		{"short row, decimal point first", "x,y,z\n.5,100", true},
		{"short row, 64-byte code prefix", "a,b,c\naaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-1,100", true},
		{"short row, 65-byte code prefix", "a,b,c\naaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-1,100", false},
		{"thousands separator after approx.", "Population was approx.1,200,000\nthen,flat", false},
		{"thousands separator after c.", "a,b,c\nc.1,200", false},
		{"thousands separator after No.", "a,b,c\nNo.1,500", false},
		{"short row, dated label with spaces", "event,qty,x\nLaunch 2024-01-05,100", true},
		{"thousands separator after a hyphenated word", "We hit pre-1,200,000\nthen pre-950,000", false},
		{"thousands separator after an underscore mid-field", "see item_1,200,000\nand item_950,000", false},
		{"thousands separator after a word and a space", "Down -1,200,000\nthen -950,000", false},
		{"equal counts, thousands separators in prose", "We sold 1,234 units\nand 5,678 more.", false},
		{"equal counts, signed amounts in prose", "It cost -1,200\nthen -950,000", false},
		{"equal counts, amounts after a colon", "Budget: 4,000,000\nspent 3,999,000", false},
		{"equal counts, address after an id", "id,address\n1,234 Main St", true},
		{"equal counts, word then number", "room,size\nRoom 213,605", true},
		{"equal counts, no header, currency and decimals", "561,$202,783.46\n2,$718,447.94", true},
		{"equal counts, no header, quoted thousands then a decimal", "\"923,552\",843.05\n\"130,757\",687.10", true},
		{"equal counts, no header, only the first row splits", "Room 213,605\nHall B,700", true},
		{"equal counts, semicolons, word then number", "Room 213;605\nRoom 214;610", true},
		{"short row, quoted comma before a number", "a,b,c\n\"x,1\",200", true},
		{"short row, quoted thousands then a number", "amount,qty,x\n\"1,200\",300", true},
		{"short row, number then quoted thousands", "id,qty,x\n7,\"100,000\"", true},
		{"short row, quoted thousands then a decimal", "note,amount,x\n\"Paid 90,761\",450.87", true},
		{"short row, number then a decimal", "id,qty,price\n464,415.15", true},
		{"short row, code then a decimal", "sku,qty,price\nSKU-168,907.75", true},
		{"short row, currency then a decimal", "id,v,w\n$202,783.46", true},
		{"short row, date then a decimal", "a,b,c,d\nAnna,2024-11-14,229.38", true},
		{"thousands separator then a decimal and a unit", "a,b,c\n1,200.5 kg", false},
		{"thousands separator then a decimal and text", "a,b,c\n1,200.5 or so", false},
		{"short row, quoted name with a comma before a number", "name,amount,note\n\"Smith, J 2\",100", true},
		{"equal counts, prose then a split number", "Hello, world.\nWe sold 1,234", false},
		{"equal counts, spaces after commas", "Name, Role\nAnna, Engineer", true},
		{"equal counts, split after text then a bare split", "spent 948,083 km\n$541,320.45 more.", false},
		{"equal counts, bare split then a split after text", "724,974 km\nspent 802,603 more.", false},
		{"equal counts, prose then a bare split", "Hi, all.\n616,442 people", false},
		{"equal counts, split number then prose", "We sold 1,234\nHello, world.", false},
		{"equal counts, no header, addresses", "1,234 Main St\n2,567 Oak Ave", true},
		{"short row, number then a one-place decimal", "id,qty,price\n464,415.1", true},
		{"thousands separator then square metres", "a,b,c\n1,200m2", false},
		{"thousands separator at the end of a sentence", "a,b,c\n1,200.", false},
		{"thousands separator, sentence end, then a number", "a,b,c\n1,200. 5", false},
		{"thousands separator then a decimal at the end of a sentence", "a,b,c\n1,200.50.", false},
		{"short row, time then an address", "a,b,c\n21:57,550 Oak St", true},
		{"short row, host then an address", "a,b,c\nweb-19,229 Lake St", true},
		{"short row, decimal then an address", "a,b,c\n62239.95,936 Elm St", true},
		{"short row, date then an address", "a,b,c\n2024-10-02,101 Cedar St", true},
		{"thousands separator after a dollar sign then a unit", "a,b,c\n$1,200 kg", false},
		{"thousands separator after a minus sign then a street", "a,b,c\n-154,664 Lake St", false},
		{"equal counts, greeting line then a split number", "Dear Anna,\nThanks for the 1,200 reports.", false},
		{"equal counts, short greeting then a split number", "Hi team,\nWe shipped 1,200 orders.", false},
		{"short row, bare thousands separator in the first record", "1,234,567 visitors\nthen,flat", false},
		{"long second row without spaces", "a,b\nc,d,e", false},
		{"equal counts, number groups ending a question", "8,362,552?\n6,985,042?", false},
		{"equal counts, number groups ending a sentence", "1,234,567.\n2,345,678.", false},
		{"thousands separator after a plus sign then a unit", "a,b,c\n+1,200 kg", false},
		{"thousands separator after an open paren then a unit", "a,b,c\n(1,200 kg)", false},
		{"short row, code then a weight", "sku,qty,x\nA1,250 kg", true},
		{"equal counts, empty header name then a split-looking row", "name,,note\nRoom 213,605,x", true},
		{"equal counts, number groups ending an exclamation", "1,234,567!\n2,345,678!", false},
		{"equal counts, number groups in parens", "(1,234,567)\n(2,345,678)", false},
		{"equal counts, no header, addresses then decimals", "1,234 Main St,7,120.50\n2,567 Oak Ave,8,130.75", true},
		{"equal counts, number groups then a sentence", "1,234,567. Then more\n2,345,678. Now less", false},
		{"equal counts, sentence end in the second record only", "1,234,567 visitors\n2,345,678.", false},
		{"equal counts, sentence end in the first record only", "1,234,567.\n2,345,678 visitors", false},
		{"equal counts, address then a note ending in a mark", "id,address,note,score\n1,234 Main St,great,100!", true},
		{"equal counts, address then a code led by a letter", "id,address,unit,code\n1,234 Main St,4,B12)", true},
		{"equal counts, address then a code ending in a letter", "id,address,unit,code\n1,234 Main St,4,12B)", true},
		{"equal counts, address then a code with a middle letter", "id,address,unit,code\n1,234 Main St,4,1B2)", true},
		{"short row, signed decimal then an address", "a,b,c\n-0.5,250 Oak St", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (&CsvParser{}).CanParse([]byte(tc.input)); got != tc.want {
				t.Errorf("CanParse(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
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
