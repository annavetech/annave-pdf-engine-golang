// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/annavetech/pdfengine/internal/ast"
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
	ordered  []InputFormat
	byFormat map[InputFormat]Parser
}

// NewRegistry returns a Registry whose YAML parser limits aliases to maxInputChars, where each value reached through an alias counts as its length in characters plus one.
func NewRegistry(maxInputChars int) *Registry {
	return &Registry{
		ordered: []InputFormat{
			// Binary formats first: fast magic-byte checks must precede text parsers.
			FormatDocx,
			FormatImage,
			FormatIPYNB,
			FormatJSON,
			FormatXML,
			FormatHTML,
			FormatCSV,
			FormatYAML,
			FormatRST,
			FormatMd,
			FormatTxt,
		},
		byFormat: newByFormat(maxInputChars),
	}
}

// newByFormat builds the format-name-to-parser table, including image
// extension aliases (png, jpg, jpeg, gif, webp) routed to ImageParser.
func newByFormat(maxInputChars int) map[InputFormat]Parser {
	byFormat := map[InputFormat]Parser{
		FormatTxt:   &TxtParser{},
		FormatMd:    &MdParser{},
		FormatJSON:  &JsonParser{},
		FormatHTML:  &HtmlParser{},
		FormatCSV:   &CsvParser{},
		FormatYAML:  &YamlParser{maxAliasChars: maxInputChars},
		FormatXML:   &XmlParser{},
		FormatRST:   &RstParser{},
		FormatIPYNB: &IpynbParser{},
		FormatDocx:  &DocxParser{},
		FormatImage: &ImageParser{},
	}
	for ext, canonical := range extToFormat {
		if canonical == FormatImage {
			byFormat[InputFormat(ext)] = byFormat[FormatImage]
		}
	}
	return byFormat
}

// isBinaryFormat reports whether format is a binary format (image or docx).
// Image extension aliases (png, jpg, jpeg, gif, webp) resolve first.
func isBinaryFormat(format InputFormat) bool {
	if canonical, ok := extToFormat[string(format)]; ok {
		format = canonical
	}
	return format == FormatImage || format == FormatDocx
}

// IsBinaryFormat reports whether format's bytes are binary (an image or a
// Word document), including its image extension aliases.
func (r *Registry) IsBinaryFormat(format InputFormat) bool {
	return isBinaryFormat(format)
}

// UnsupportedFormatError reports that Format names no registered parser.
// FormatAuto never produces this error.
type UnsupportedFormatError struct {
	Format InputFormat
}

func (e *UnsupportedFormatError) Error() string {
	return fmt.Sprintf("unsupported format: %q", string(e.Format))
}

// Parse dispatches input to the parser for format, or auto-detects it when
// format is FormatAuto. An unrecognised format returns *UnsupportedFormatError.
func (r *Registry) Parse(input []byte, format InputFormat) (*ast.DocumentNode, error) {
	if format == FormatAuto {
		format = r.Detect(input)
		if format == FormatAuto {
			return &ast.DocumentNode{Type: "document"}, nil
		}
	}
	p, ok := r.byFormat[format]
	if !ok {
		return nil, &UnsupportedFormatError{Format: format}
	}
	if len(input) == 0 {
		return &ast.DocumentNode{Type: "document"}, nil
	}
	data := input
	if !isBinaryFormat(format) {
		data = bytes.TrimSpace(input)
		if len(data) == 0 {
			return &ast.DocumentNode{Type: "document"}, nil
		}
	}
	return p.Parse(data)
}

// Detect returns the format of the first parser in the ordered list that accepts input.
// It returns FormatAuto for empty input or when no parser matches.
func (r *Registry) Detect(input []byte) InputFormat {
	if len(input) == 0 {
		return FormatAuto
	}
	data := input
	if !r.IsBinaryInput(input) {
		data = bytes.TrimSpace(input)
		if len(data) == 0 {
			return FormatAuto
		}
	}
	for _, f := range r.ordered {
		if r.byFormat[f].CanParse(data) {
			return f
		}
	}
	return FormatAuto
}

// SupportsFormat reports whether format is FormatAuto or names a
// registered parser.
func (r *Registry) SupportsFormat(format InputFormat) bool {
	if format == FormatAuto {
		return true
	}
	_, ok := r.byFormat[format]
	return ok
}

func (r *Registry) IsBinaryInput(input []byte) bool {
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
