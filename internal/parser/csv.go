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

func (p *CsvParser) CanParse(input []byte) bool {
	line := csvSniffLine(input)
	if line == "" {
		return false
	}
	r := csv.NewReader(strings.NewReader(line))
	r.Comma = rune(detectSep(line))
	r.FieldsPerRecord = -1
	fields, err := r.Read()
	return err == nil && len(fields) >= 2
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
