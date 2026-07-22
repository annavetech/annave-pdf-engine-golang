// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"regexp"
	"strings"

	"annave.tech/pdf-engine/internal/ast"
)

var (
	yamlKeyValue = regexp.MustCompile(`^[a-zA-Z0-9_"'][^:]*\s*:`)
	yamlTopList  = regexp.MustCompile(`^- `)
)

type YamlParser struct{}

func (p *YamlParser) CanParse(input string) bool {
	t := strings.TrimSpace(input)
	return strings.HasPrefix(t, "---") ||
		yamlKeyValue.MatchString(t) ||
		yamlTopList.MatchString(t)
}

func (p *YamlParser) Parse(input string) (*ast.DocumentNode, error) {
	lines := stripYamlComments(strings.Split(input, "\n"))
	nodes := parseYamlBlock(lines, 0)
	return &ast.DocumentNode{Type: ast.TypeDocument, Children: nodes}, nil
}

func stripYamlComments(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		// strip inline comments naively (good enough for Phase 1)
		if idx := strings.Index(l, " #"); idx >= 0 {
			l = l[:idx]
		}
		out = append(out, strings.TrimRight(l, " \t\r"))
	}
	return out
}

func parseYamlBlock(lines []string, baseIndent int) []ast.Node {
	var nodes []ast.Node
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "---" || trimmed == "..." {
			i++
			continue
		}
		indent := indentOf(line)
		if indent < baseIndent {
			break
		}

		// Sequence item
		if strings.HasPrefix(trimmed, "- ") {
			result, next := parseYamlSequence(lines, i, indent)
			nodes = append(nodes, result)
			i = next
			continue
		}

		// Mapping entry key: value
		colonIdx := findYamlColon(trimmed)
		if colonIdx != -1 {
			key := strings.Trim(trimmed[:colonIdx], `'"`)
			value := strings.TrimSpace(trimmed[colonIdx+1:])

			// Inline list [a, b, c]
			if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
				items := strings.Split(value[1:len(value)-1], ",")
				var cleanItems []string
				for _, it := range items {
					s := strings.TrimSpace(it)
					if s != "" {
						cleanItems = append(cleanItems, s)
					}
				}
				nodes = append(nodes, ast.Node{Type: ast.TypeHeading, Level: 2, Text: key, Spans: []ast.InlineSpan{{Kind: ast.SpanText, Text: key}}})
				if len(cleanItems) > 0 {
					var itemSpans [][]ast.InlineSpan
					for _, it := range cleanItems {
						itemSpans = append(itemSpans, ParseInline(it))
					}
					nodes = append(nodes, ast.Node{Type: ast.TypeList, Ordered: false, Items: cleanItems, ItemSpans: itemSpans})
				}
				i++
				continue
			}

			// Block scalar
			if value == "|" || value == ">" {
				nodes = append(nodes, ast.Node{Type: ast.TypeHeading, Level: 2, Text: key, Spans: []ast.InlineSpan{{Kind: ast.SpanText, Text: key}}})
				text, next := parseYamlBlockScalar(lines, i+1, indent+1, value == ">")
				if text != "" {
					nodes = append(nodes, ast.Node{Type: ast.TypeParagraph, Text: text, Spans: ParseInline(text)})
				}
				i = next
				continue
			}

			// Inline scalar
			if value != "" {
				nodes = append(nodes, ast.Node{Type: ast.TypeHeading, Level: 2, Text: key, Spans: []ast.InlineSpan{{Kind: ast.SpanText, Text: key}}})
				nodes = append(nodes, ast.Node{Type: ast.TypeParagraph, Text: value, Spans: ParseInline(value)})
				i++
				continue
			}

			// Empty value — collect child lines
			nodes = append(nodes, ast.Node{Type: ast.TypeHeading, Level: 2, Text: key, Spans: []ast.InlineSpan{{Kind: ast.SpanText, Text: key}}})
			var childLines []string
			j := i + 1
			for j < len(lines) {
				cl := lines[j]
				if strings.TrimSpace(cl) == "" {
					childLines = append(childLines, cl)
					j++
					continue
				}
				if indentOf(cl) > indent {
					childLines = append(childLines, cl)
					j++
				} else {
					break
				}
			}
			if len(childLines) > 0 {
				children := parseYamlBlock(childLines, indent+1)
				if tbl := tryAsTable(children); tbl != nil {
					nodes = append(nodes, *tbl)
				} else {
					nodes = append(nodes, children...)
				}
			}
			i = j
			continue
		}

		nodes = append(nodes, ast.Node{Type: ast.TypeParagraph, Text: trimmed, Spans: ParseInline(trimmed)})
		i++
	}
	return nodes
}

func parseYamlSequence(lines []string, start, indent int) (ast.Node, int) {
	var items []string
	i := start
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			i++
			continue
		}
		if indentOf(lines[i]) < indent {
			break
		}
		if !strings.HasPrefix(t, "- ") {
			break
		}
		items = append(items, strings.TrimPrefix(t, "- "))
		i++
	}
	var itemSpans [][]ast.InlineSpan
	for _, it := range items {
		itemSpans = append(itemSpans, ParseInline(it))
	}
	return ast.Node{Type: ast.TypeList, Ordered: false, Items: items, ItemSpans: itemSpans}, i
}

func parseYamlBlockScalar(lines []string, start, minIndent int, fold bool) (string, int) {
	var parts []string
	i := start
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" || indentOf(line) >= minIndent {
			parts = append(parts, strings.TrimSpace(line))
			i++
		} else {
			break
		}
	}
	var text string
	if fold {
		text = strings.Join(parts, " ")
		text = strings.Join(strings.Fields(text), " ")
	} else {
		text = strings.Join(parts, "\n")
	}
	return strings.TrimSpace(text), i
}

func tryAsTable(nodes []ast.Node) *ast.Node {
	if len(nodes) < 2 || len(nodes)%2 != 0 {
		return nil
	}
	var rows [][]string
	for i := 0; i < len(nodes); i += 2 {
		if nodes[i].Type != ast.TypeHeading || nodes[i+1].Type != ast.TypeParagraph {
			return nil
		}
		rows = append(rows, []string{nodes[i].Text, nodes[i+1].Text})
	}
	n := ast.Node{Type: ast.TypeTable, Headers: []string{"Key", "Value"}, Rows: rows}
	return &n
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func findYamlColon(s string) int {
	inQ := false
	var q byte
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if !inQ && (ch == '"' || ch == '\'') {
			inQ = true
			q = ch
		} else if inQ && ch == q {
			inQ = false
		} else if !inQ && ch == ':' {
			if i+1 >= len(s) || s[i+1] == ' ' {
				return i
			}
		}
	}
	return -1
}
