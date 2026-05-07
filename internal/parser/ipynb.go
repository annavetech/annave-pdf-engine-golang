// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/json"
	"strings"

	"annave.tech/pdf-engine/internal/ast"
)

type IpynbParser struct{}

func (p *IpynbParser) CanParse(input string) bool {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(input), &obj); err != nil {
		return false
	}
	_, hasCells := obj["cells"]
	_, hasNbformat := obj["nbformat"]
	return hasCells || hasNbformat
}

func (p *IpynbParser) Parse(input string) (*ast.DocumentNode, error) {
	var nb struct {
		Metadata *struct {
			Kernelspec   *struct{ Language string `json:"language"` } `json:"kernelspec"`
			LanguageInfo *struct{ Name string `json:"name"` }      `json:"language_info"`
		} `json:"metadata"`
		Cells []struct {
			CellType string      `json:"cell_type"`
			Source   interface{} `json:"source"` // string or []string
			Outputs  []struct {
				OutputType string      `json:"output_type"`
				Text       interface{} `json:"text"`
				Data       *struct {
					TextPlain interface{} `json:"text/plain"`
				} `json:"data"`
				Ename  string `json:"ename"`
				Evalue string `json:"evalue"`
			} `json:"outputs"`
		} `json:"cells"`
	}

	if err := json.Unmarshal([]byte(input), &nb); err != nil {
		return &ast.DocumentNode{Type: ast.TypeDocument}, nil
	}

	lang := ""
	if nb.Metadata != nil {
		if nb.Metadata.Kernelspec != nil {
			lang = nb.Metadata.Kernelspec.Language
		}
		if lang == "" && nb.Metadata.LanguageInfo != nil {
			lang = nb.Metadata.LanguageInfo.Name
		}
	}

	var children []ast.Node
	for _, cell := range nb.Cells {
		src := joinSource(cell.Source)

		switch cell.CellType {
		case "markdown":
			doc, _ := (&MdParser{}).Parse(src)
			children = append(children, doc.Children...)

		case "code":
			if src != "" {
				children = append(children, ast.Node{Type: ast.TypeCodeBlock, Language: lang, Text: src})
			}
			for _, out := range cell.Outputs {
				if out.OutputType == "error" {
					text := out.Ename + ": " + out.Evalue
					children = append(children, ast.Node{
						Type:  ast.TypeBlockquote,
						Spans: []ast.InlineSpan{{Kind: ast.SpanCode, Text: strings.TrimSpace(text)}},
					})
					continue
				}
				raw := out.Text
				if raw == nil && out.Data != nil {
					raw = out.Data.TextPlain
				}
				if raw == nil {
					continue
				}
				text := strings.TrimSpace(joinSource(raw))
				if text != "" {
					children = append(children, ast.Node{
						Type:  ast.TypeBlockquote,
						Spans: []ast.InlineSpan{{Kind: ast.SpanText, Text: text}},
					})
				}
			}

		case "raw":
			if src != "" {
				doc, _ := (&MdParser{}).Parse(src)
				children = append(children, doc.Children...)
			}
		}
	}

	return &ast.DocumentNode{Type: ast.TypeDocument, Children: children}, nil
}

func joinSource(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []interface{}:
		var parts []string
		for _, s := range t {
			parts = append(parts, str(s))
		}
		return strings.Join(parts, "")
	}
	return ""
}
