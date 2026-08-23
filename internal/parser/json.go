// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/json"
	"strings"

	"github.com/annavetech/annave-pdf-engine-golang/internal/ast"
)

type JsonParser struct{}

func (p *JsonParser) CanParse(input string) bool {
	t := strings.TrimSpace(input)
	return strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[")
}

func (p *JsonParser) Parse(input string) (*ast.DocumentNode, error) {
	var raw interface{}
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return &ast.DocumentNode{Type: ast.TypeDocument}, nil
	}
	return normalizeJSON(raw), nil
}

func normalizeJSON(raw interface{}) *ast.DocumentNode {
	switch v := raw.(type) {
	case map[string]interface{}:
		if t, ok := v["type"].(string); ok && t == "document" {
			if ver, ok := v["version"].(string); ok {
				_ = ver // schema version noted but not enforced
			}
			if children, ok := v["children"].([]interface{}); ok {
				var nodes []ast.Node
				for _, c := range children {
					if n := normalizeJSONNode(c); n != nil {
						nodes = append(nodes, *n)
					}
				}
				return &ast.DocumentNode{Type: ast.TypeDocument, Children: nodes}
			}
		}
		return extractFromObject(v)
	case []interface{}:
		var nodes []ast.Node
		for _, c := range v {
			if n := normalizeJSONNode(c); n != nil {
				nodes = append(nodes, *n)
			}
		}
		return &ast.DocumentNode{Type: ast.TypeDocument, Children: nodes}
	}
	return &ast.DocumentNode{Type: ast.TypeDocument}
}

func normalizeJSONNode(raw interface{}) *ast.Node {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	typ, _ := m["type"].(string)

	switch typ {
	case "heading":
		text := strings.TrimSpace(str(m["text"]))
		if text == "" {
			return nil
		}
		level := int(num(m["level"]))
		if level < 1 {
			level = 1
		}
		if level > 3 {
			level = 3
		}
		spans := parseJSONSpans(m["spans"], text)
		return &ast.Node{Type: ast.TypeHeading, Level: level, Text: text, Spans: spans}

	case "paragraph":
		text := strings.TrimSpace(str(m["text"]))
		if text == "" {
			return nil
		}
		spans := parseJSONSpans(m["spans"], text)
		return &ast.Node{Type: ast.TypeParagraph, Text: text, Spans: spans}

	case "list":
		ordered, _ := m["ordered"].(bool)
		var items []string
		if arr, ok := m["items"].([]interface{}); ok {
			for _, it := range arr {
				s := strings.TrimSpace(str(it))
				if s != "" {
					items = append(items, s)
				}
			}
		}
		if len(items) == 0 {
			return nil
		}
		var itemSpans [][]ast.InlineSpan
		if arr, ok := m["itemSpans"].([]interface{}); ok {
			for i, s := range arr {
				fallback := ""
				if i < len(items) {
					fallback = items[i]
				}
				itemSpans = append(itemSpans, parseJSONSpans(s, fallback))
			}
		} else {
			for _, it := range items {
				itemSpans = append(itemSpans, ParseInline(it))
			}
		}
		return &ast.Node{Type: ast.TypeList, Ordered: ordered, Items: items, ItemSpans: itemSpans}

	case "table":
		var headers []string
		if arr, ok := m["headers"].([]interface{}); ok {
			for _, h := range arr {
				headers = append(headers, strings.TrimSpace(str(h)))
			}
		}
		var rows [][]string
		if arr, ok := m["rows"].([]interface{}); ok {
			for _, r := range arr {
				if row, ok := r.([]interface{}); ok {
					var cells []string
					for _, c := range row {
						cells = append(cells, strings.TrimSpace(str(c)))
					}
					rows = append(rows, cells)
				}
			}
		}
		return &ast.Node{Type: ast.TypeTable, Headers: headers, Rows: rows}

	case "code-block", "code":
		text := strings.TrimSpace(str(first(m["text"], m["code"])))
		if text == "" {
			return nil
		}
		lang := str(first(m["language"], m["lang"]))
		return &ast.Node{Type: ast.TypeCodeBlock, Language: lang, Text: text}

	case "blockquote":
		text := strings.TrimSpace(str(m["text"]))
		spans := parseJSONSpans(m["spans"], text)
		if len(spans) == 0 {
			return nil
		}
		return &ast.Node{Type: ast.TypeBlockquote, Spans: spans}

	case "hr":
		return &ast.Node{Type: ast.TypeHR}

	case "image":
		src := strings.TrimSpace(str(first(m["src"], m["url"])))
		if src == "" {
			return nil
		}
		return &ast.Node{Type: ast.TypeImage, Alt: str(m["alt"]), Src: src}
	}

	// Heuristic fallback for foreign JSON
	if title := str(first(m["title"], m["heading"])); title != "" {
		t := strings.TrimSpace(title)
		return &ast.Node{Type: ast.TypeHeading, Level: 1, Text: t, Spans: ParseInline(t)}
	}
	if body := str(first(m["text"], m["content"], m["body"], m["description"])); body != "" {
		t := strings.TrimSpace(body)
		return &ast.Node{Type: ast.TypeParagraph, Text: t, Spans: ParseInline(t)}
	}
	return nil
}

func extractFromObject(obj map[string]interface{}) *ast.DocumentNode {
	var children []ast.Node
	if t := strings.TrimSpace(str(first(obj["title"], obj["name"]))); t != "" {
		children = append(children, ast.Node{Type: ast.TypeHeading, Level: 1, Text: t, Spans: ParseInline(t)})
	}
	if s := strings.TrimSpace(str(first(obj["subtitle"], obj["summary"]))); s != "" {
		children = append(children, ast.Node{Type: ast.TypeHeading, Level: 2, Text: s, Spans: ParseInline(s)})
	}
	if b := strings.TrimSpace(str(first(obj["text"], obj["content"], obj["body"], obj["description"]))); b != "" {
		children = append(children, ast.Node{Type: ast.TypeParagraph, Text: b, Spans: ParseInline(b)})
	}
	return &ast.DocumentNode{Type: ast.TypeDocument, Children: children}
}

func parseJSONSpans(raw interface{}, fallback string) []ast.InlineSpan {
	if arr, ok := raw.([]interface{}); ok {
		var spans []ast.InlineSpan
		for _, s := range arr {
			if m, ok := s.(map[string]interface{}); ok {
				text := strings.TrimSpace(str(m["text"]))
				if text == "" {
					continue
				}
				kind := str(m["kind"])
				if kind == "" {
					kind = ast.SpanText
				}
				spans = append(spans, ast.InlineSpan{Kind: kind, Text: text, Href: str(m["href"])})
			}
		}
		if len(spans) > 0 {
			return spans
		}
	}
	return ParseInline(fallback)
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func num(v interface{}) float64 {
	if v == nil {
		return 0
	}
	n, _ := v.(float64)
	return n
}

func first(vals ...interface{}) interface{} {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}
