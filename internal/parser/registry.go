// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"

	"annave.tech/pdf-engine/internal/ast"
)

type InputFormat string

const (
	FormatAuto  InputFormat = "auto"
	FormatTxt   InputFormat = "txt"
	FormatMd    InputFormat = "md"
	FormatJSON  InputFormat = "json"
	FormatHTML  InputFormat = "html"
	FormatCSV   InputFormat = "csv"
	FormatYAML  InputFormat = "yaml"
	FormatXML   InputFormat = "xml"
	FormatRST   InputFormat = "rst"
	FormatIPYNB InputFormat = "ipynb"
	FormatDocx  InputFormat = "docx"
	FormatImage InputFormat = "image"
)

var extToFormat = map[string]InputFormat{
	"json": FormatJSON, "html": FormatHTML, "htm": FormatHTML,
	"md": FormatMd, "markdown": FormatMd, "txt": FormatTxt,
	"csv": FormatCSV, "tsv": FormatCSV,
	"yaml": FormatYAML, "yml": FormatYAML,
	"xml":   FormatXML,
	"rst":   FormatRST,
	"ipynb": FormatIPYNB,
	"docx":  FormatDocx,
	"png":   FormatImage, "jpg": FormatImage, "jpeg": FormatImage,
	"gif": FormatImage, "webp": FormatImage,
}

type Registry struct {
	ordered  []Parser
	byFormat map[InputFormat]Parser
}

func NewRegistry() *Registry {
	return &Registry{
		ordered: []Parser{
			// Binary formats first — fast magic-byte checks, must precede text parsers.
			&DocxParser{},
			&ImageParser{},
			&IpynbParser{},
			&JsonParser{},
			&XmlParser{},
			&HtmlParser{},
			&CsvParser{},
			&YamlParser{},
			&RstParser{},
			&MdParser{},
			&TxtParser{},
		},
		byFormat: map[InputFormat]Parser{
			FormatTxt:   &TxtParser{},
			FormatMd:    &MdParser{},
			FormatJSON:  &JsonParser{},
			FormatHTML:  &HtmlParser{},
			FormatCSV:   &CsvParser{},
			FormatYAML:  &YamlParser{},
			FormatXML:   &XmlParser{},
			FormatRST:   &RstParser{},
			FormatIPYNB: &IpynbParser{},
			FormatDocx:  &DocxParser{},
			FormatImage: &ImageParser{},
		},
	}
}

func (r *Registry) Parse(input string, format InputFormat) (*ast.DocumentNode, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return &ast.DocumentNode{Type: "document"}, nil
	}

	if format != FormatAuto {
		if p, ok := r.byFormat[format]; ok {
			return p.Parse(trimmed)
		}
	}

	for _, p := range r.ordered {
		if p.CanParse(trimmed) {
			return p.Parse(trimmed)
		}
	}

	return &ast.DocumentNode{Type: "document"}, nil
}

func (r *Registry) LooksLikeHTML(input string) bool {
	return (&HtmlParser{}).CanParse(input)
}

func (r *Registry) IsBinaryInput(input string) bool {
	return (&DocxParser{}).CanParse(input) || (&ImageParser{}).CanParse(input)
}

func FormatFromExtension(filename string) InputFormat {
	parts := strings.Split(filename, ".")
	if len(parts) < 2 {
		return FormatAuto
	}
	ext := strings.ToLower(parts[len(parts)-1])
	if f, ok := extToFormat[ext]; ok {
		return f
	}
	return FormatAuto
}
