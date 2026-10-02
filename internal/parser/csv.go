// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/annavetech/pdfengine/internal/ast"
)

type CsvParser struct{}

// CanParse reads only the first two records. It claims equal field counts, at least 2, unless the separator is a comma and a split number reads as prose.
// It claims a shorter second record of 2 or more fields when no field starts with a space or tab and, with a comma separator, no number is split at a thousands separator.
func (p *CsvParser) CanParse(input []byte) bool {
	line := csvSniffLine(input)
	if line == "" {
		return false
	}
	r := csv.NewReader(bytes.NewReader(input))
	sep := detectSep(line)
	r.Comma = rune(sep)
	r.FieldsPerRecord = -1
	first, err := r.Read()
	if err != nil || len(first) < 2 {
		return false
	}
	second, err := r.Read()
	if err != nil || len(second) < 2 || len(second) > len(first) {
		return false
	}
	// Prose puts a space after its commas and splits numbers at thousands separators; CSV data rarely does either.
	// Only a comma can be a thousands separator.
	if len(second) == len(first) {
		if sep != ',' {
			return true
		}
		ba, bb := csvThousands(first, true), csvThousands(second, true)
		if !ba && !bb {
			return true
		}
		// A greeting line such as "Dear Anna," then a split number is a letter.
		if len(first) == 2 && first[1] == "" {
			return false
		}
		// A split number reads as prose with a field that starts with a space, after text in one record and split in the other, or ending a sentence.
		return !csvLeadingSpace(first) && !csvLeadingSpace(second) &&
			(!bb || !csvThousands(first, false)) && (!ba || !csvThousands(second, false)) &&
			!csvSentenceEnd(first) && !csvSentenceEnd(second)
	}
	return !csvLeadingSpace(first) && !csvLeadingSpace(second) &&
		(sep != ',' || (!csvThousands(first, true) && !csvThousands(second, true)))
}

// csvThousands reports whether rec splits a number in text at a thousands separator, as in "rose to 1" then "200" or "2" then "345 km2".
// A field holding a comma was quoted, so its pair is skipped. With bare false, only a split after text counts. O(n) over rec, no allocation.
func csvThousands(rec []string, bare bool) bool {
	for i := 0; i+1 < len(rec); i++ {
		f, g := rec[i], rec[i+1]
		if len(g) < 3 || !csvDigit(g[0]) || !csvDigit(g[1]) || !csvDigit(g[2]) || (len(g) > 3 && csvDigit(g[3])) {
			continue
		}
		k := 0
		for k < 4 && k < len(f) && csvDigit(f[len(f)-1-k]) {
			k++
		}
		if k >= 1 && k <= 3 && ((k < len(f) && csvTextBefore(f, len(f)-1-k)) || (bare && len(g) > 3 && !csvDecimal(g) && csvNumberStart(f, k))) &&
			strings.IndexByte(f, ',') < 0 && strings.IndexByte(g, ',') < 0 {
			return true
		}
	}
	return false
}

// csvNumberStart reports whether f, ending in k digits, is only those digits after an optional sign, '$' or '(', as in "1" or "$202".
func csvNumberStart(f string, k int) bool {
	return k == len(f) || (k == len(f)-1 && (f[0] == '-' || f[0] == '+' || f[0] == '$' || f[0] == '('))
}

// csvSentenceEnd reports whether rec splits a number that ends a sentence, as in "1" then "200." or "552?".
func csvSentenceEnd(rec []string) bool {
	for i := 0; i+1 < len(rec); i++ {
		f, g := rec[i], rec[i+1]
		if f == "" || !csvDigit(f[len(f)-1]) || len(g) < 4 || !csvDigit(g[0]) || !csvDigit(g[1]) || !csvDigit(g[2]) {
			continue
		}
		if g[3] == '!' || g[3] == '?' || g[3] == ')' || (g[3] == '.' && (len(g) == 4 || !csvDigit(g[4]))) {
			return true
		}
	}
	return false
}

// csvDecimal reports whether g is three digits, a point and one or more digits, as in "415.15".
func csvDecimal(g string) bool {
	if len(g) < 5 || g[3] != '.' {
		return false
	}
	for j := 4; j < len(g); j++ {
		if !csvDigit(g[j]) {
			return false
		}
	}
	return true
}

// csvTextBefore reports whether f[i], just before a run of digits, is text, as opposed to part of a
// value such as "0.5", ".5", "A1", "2024-01-01", "09:30", "-5", "$5", "SKU-1" or "item_1".
func csvTextBefore(f string, i int) bool {
	b := f[i]
	switch {
	case b|0x20 >= 'a' && b|0x20 <= 'z':
		return false
	case (b == '-' || b == '+' || b == '$' || b == '(' || b == '.') && i == 0:
		return false
	case (b == '.' || b == '/' || b == ':' || b == '-') && i > 0 && csvDigit(f[i-1]):
		return false
	case (b == '-' || b == '_') && csvCodePrefix(f[:i]):
		return false
	}
	return true
}

// csvCodePrefix reports whether s, at most 64 bytes, holds only ASCII letters, digits, '-' and '_'.
func csvCodePrefix(s string) bool {
	if len(s) > 64 {
		return false
	}
	for j := 0; j < len(s); j++ {
		c := s[j]
		if !csvDigit(c) && c != '-' && c != '_' && (c|0x20 < 'a' || c|0x20 > 'z') {
			return false
		}
	}
	return true
}

func csvDigit(b byte) bool { return b >= '0' && b <= '9' }

// csvLeadingSpace reports whether any field in rec starts with a space or tab.
func csvLeadingSpace(rec []string) bool {
	for _, f := range rec {
		if f != "" && (f[0] == ' ' || f[0] == '\t') {
			return true
		}
	}
	return false
}

// Parse reads input as CSV via the standard library's encoding/csv. Ragged
// rows are tolerated: a short row is padded, and a long row is never truncated.
func (p *CsvParser) Parse(input []byte) (*ast.DocumentNode, error) {
	line := csvSniffLine(input)
	if line == "" {
		return &ast.DocumentNode{Type: ast.TypeDocument}, nil
	}
	sep := detectSep(line)

	r := csv.NewReader(bytes.NewReader(input))
	r.Comma = rune(sep)
	r.FieldsPerRecord = -1

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv: %w", err)
	}
	if len(records) == 0 {
		return &ast.DocumentNode{Type: ast.TypeDocument}, nil
	}

	headers := csvTrimFields(records[0])
	dataRecords := records[1:]

	// The table's column count is the width of its widest row, headers
	// included; headers is padded with empty labels to match, not truncated.
	numCols := len(headers)
	for _, rec := range dataRecords {
		if len(rec) > numCols {
			numCols = len(rec)
		}
	}
	for len(headers) < numCols {
		headers = append(headers, "")
	}

	rows := make([][]string, 0, len(dataRecords))
	for _, rec := range dataRecords {
		cells := csvTrimFields(rec)
		for len(cells) < numCols {
			cells = append(cells, "")
		}
		rows = append(rows, cells)
	}
	return &ast.DocumentNode{
		Type:     ast.TypeDocument,
		Children: []ast.Node{{Type: ast.TypeTable, Headers: headers, Rows: rows}},
	}, nil
}

// csvTrimFields trims surrounding whitespace from every field in rec.
func csvTrimFields(rec []string) []string {
	out := make([]string, len(rec))
	for i, f := range rec {
		out[i] = strings.TrimSpace(f)
	}
	return out
}

func detectSep(line string) byte {
	tabs := strings.Count(line, "\t")
	commas := strings.Count(line, ",")
	semis := strings.Count(line, ";")
	if tabs >= commas && tabs >= semis {
		return '\t'
	}
	if semis > commas {
		return ';'
	}
	return ','
}

// csvSniffLine returns the first non-blank line of input, with its trailing
// "\r" (if any) stripped, for separator detection.
func csvSniffLine(input []byte) string {
	start := 0
	for start < len(input) {
		var raw []byte
		if nl := bytes.IndexByte(input[start:], '\n'); nl >= 0 {
			raw = input[start : start+nl]
			start += nl + 1
		} else {
			raw = input[start:]
			start = len(input)
		}
		line := strings.TrimSpace(strings.TrimRight(string(raw), "\r"))
		if line != "" {
			return line
		}
	}
	return ""
}
