// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package pdfengine

import "github.com/annavetech/annave-pdf-engine-golang/internal/parser"

// Format selects which parser converts the input, or requests
// auto-detection.
type Format string

const (
	// FormatAuto detects the format from the content itself: magic bytes
	// for binary formats (Word documents, raster images), structural
	// markers (an ATX heading, a JSON object, an RST-style underline) for
	// text formats. Falls back to plain text if nothing matches.
	FormatAuto Format = "auto"

	// FormatMarkdown parses GitHub-flavoured Markdown: ATX headings,
	// bold, italic, inline code, strikethrough, links, images, fenced
	// code blocks, blockquotes, ordered and unordered lists, tables, and
	// thematic breaks. Extension: .md
	FormatMarkdown Format = "md"

	// FormatText treats the input as plain text, splitting paragraphs on
	// blank lines. Extension: .txt
	FormatText Format = "txt"

	// FormatJSON parses the document schema described by
	// schema/document.v1.schema.json. Extension: .json
	FormatJSON Format = "json"

	// FormatHTML parses HTML and sanitises it before layout. Extensions:
	// .html, .htm
	FormatHTML Format = "html"

	// FormatCSV renders each row as a table row; the first row is
	// treated as the header. Extensions: .csv, .tsv
	FormatCSV Format = "csv"

	// FormatYAML maps top-level keys and lists to document structure.
	// Extensions: .yaml, .yml
	FormatYAML Format = "yaml"

	// FormatXML maps elements to document nodes. Extension: .xml
	FormatXML Format = "xml"

	// FormatRST parses reStructuredText: headings, directives, field
	// lists, literal blocks, bullet and ordered lists. Extension: .rst
	FormatRST Format = "rst"

	// FormatNotebook parses Jupyter notebook code and markdown cells.
	// Extension: .ipynb
	FormatNotebook Format = "ipynb"

	// FormatWord parses Word documents: headings, paragraphs, lists, and
	// tables. Pure Go — no external converter is invoked. Extension:
	// .docx
	FormatWord Format = "docx"

	// FormatImage embeds a raster image at full page width, preserving
	// aspect ratio. Extensions: .png, .jpg, .jpeg, .gif, .webp
	FormatImage Format = "image"
)

// toInternal translates a Format to the parser registry's own format type.
// Written as an explicit switch rather than a cast so that a future change
// to the internal string values cannot silently change this package's
// public contract.
func (f Format) toInternal() parser.InputFormat {
	switch f {
	case FormatMarkdown:
		return parser.FormatMd
	case FormatText:
		return parser.FormatTxt
	case FormatJSON:
		return parser.FormatJSON
	case FormatHTML:
		return parser.FormatHTML
	case FormatCSV:
		return parser.FormatCSV
	case FormatYAML:
		return parser.FormatYAML
	case FormatXML:
		return parser.FormatXML
	case FormatRST:
		return parser.FormatRST
	case FormatNotebook:
		return parser.FormatIPYNB
	case FormatWord:
		return parser.FormatDocx
	case FormatImage:
		return parser.FormatImage
	default:
		return parser.FormatAuto
	}
}
