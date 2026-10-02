// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/annavetech/pdfengine/internal/ast"
)

type IpynbParser struct{}

// jsonKeySeen records that a key was present without decoding or copying its value.
type jsonKeySeen bool

func (k *jsonKeySeen) UnmarshalJSON([]byte) error {
	*k = true
	return nil
}

// jsonNumberSeen records whether a key's value is a JSON number, from its first byte only.
type jsonNumberSeen bool

func (k *jsonNumberSeen) UnmarshalJSON(b []byte) error {
	*k = len(b) > 0 && (b[0] == '-' || (b[0] >= '0' && b[0] <= '9'))
	return nil
}

func (p *IpynbParser) CanParse(input []byte) bool {
	t := bytes.TrimLeft(input, " \t\r\n")
	if len(t) == 0 || t[0] != '{' {
		return false
	}
	// A notebook needs cells and a numeric nbformat. Key matching is as in Parse; other values are skipped, not built.
	var obj struct {
		Cells    jsonKeySeen    `json:"cells"`
		Nbformat jsonNumberSeen `json:"nbformat"`
	}
	if err := json.Unmarshal(t, &obj); err != nil {
		return false
	}
	return bool(obj.Cells) && bool(obj.Nbformat)
}

func (p *IpynbParser) Parse(input []byte) (*ast.DocumentNode, error) {
	var nb struct {
		Metadata *struct {
			Kernelspec *struct {
				Language string `json:"language"`
			} `json:"kernelspec"`
			LanguageInfo *struct {
				Name string `json:"name"`
			} `json:"language_info"`
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

	if err := json.Unmarshal(input, &nb); err != nil {
		// A type error names the notebook's own struct type; report the field and JSON kind instead.
		var te *json.UnmarshalTypeError
		if errors.As(err, &te) {
			return nil, fmt.Errorf("ipynb: field %q: unexpected JSON %s", te.Field, te.Value)
		}
		return nil, fmt.Errorf("ipynb: %w", err)
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
			doc, _ := (&MdParser{}).Parse([]byte(src))
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
				doc, _ := (&MdParser{}).Parse([]byte(src))
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
