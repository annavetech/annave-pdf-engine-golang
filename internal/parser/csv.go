// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"

	"annave.tech/pdf-engine/internal/ast"
)

type CsvParser struct{}

func (p *CsvParser) CanParse(input string) bool {
	lines := nonEmptyTrimmedLines(input)
	if len(lines) == 0 {
		return false
	}
	sep := detectSep(lines[0])
	return len(splitCSVLine(lines[0], sep)) >= 2
}

func (p *CsvParser) Parse(input string) (*ast.DocumentNode, error) {
	lines := nonEmptyTrimmedLines(input)
	if len(lines) == 0 {
		return &ast.DocumentNode{Type: ast.TypeDocument}, nil
	}
	sep := detectSep(lines[0])
	headers := splitCSVLine(lines[0], sep)
	var rows [][]string
	for _, l := range lines[1:] {
		cells := splitCSVLine(l, sep)
		for len(cells) < len(headers) {
			cells = append(cells, "")
		}
		rows = append(rows, cells[:len(headers)])
	}
	return &ast.DocumentNode{
		Type: ast.TypeDocument,
		Children: []ast.Node{{Type: ast.TypeTable, Headers: headers, Rows: rows}},
	}, nil
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

func splitCSVLine(line string, sep byte) []string {
	var result []string
	field := ""
	inQuotes := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '"' {
			if inQuotes && i+1 < len(line) && line[i+1] == '"' {
				field += "\""
				i++
			} else {
				inQuotes = !inQuotes
			}
		} else if ch == sep && !inQuotes {
			result = append(result, strings.TrimSpace(field))
			field = ""
		} else {
			field += string(ch)
		}
	}
	result = append(result, strings.TrimSpace(field))
	return result
}

func nonEmptyTrimmedLines(input string) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(input), "\n") {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
