// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package ast

// InlineSpan kinds
const (
	SpanText       = "text"
	SpanBold       = "bold"
	SpanItalic     = "italic"
	SpanBoldItalic = "bold-italic"
	SpanCode       = "code"
	SpanStrike     = "strike"
	SpanLink       = "link"
)

type InlineSpan struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	Href string `json:"href,omitempty"`
}

// Node types
const (
	TypeHeading    = "heading"
	TypeParagraph  = "paragraph"
	TypeList       = "list"
	TypeTable      = "table"
	TypeCodeBlock  = "code-block"
	TypeBlockquote = "blockquote"
	TypeHR         = "hr"
	TypeImage      = "image"
	TypeDocument   = "document"
)

type HeadingNode struct {
	Type  string       `json:"type"`  // "heading"
	Level int          `json:"level"` // 1|2|3
	Text  string       `json:"text"`
	Spans []InlineSpan `json:"spans,omitempty"`
}

type ParagraphNode struct {
	Type  string       `json:"type"` // "paragraph"
	Text  string       `json:"text"`
	Spans []InlineSpan `json:"spans,omitempty"`
}

type ListNode struct {
	Type      string         `json:"type"` // "list"
	Ordered   bool           `json:"ordered"`
	Items     []string       `json:"items"`
	ItemSpans [][]InlineSpan `json:"itemSpans,omitempty"`
}

type TableNode struct {
	Type    string     `json:"type"` // "table"
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

type CodeBlockNode struct {
	Type     string `json:"type"` // "code-block"
	Language string `json:"language"`
	Text     string `json:"text"`
}

type BlockquoteNode struct {
	Type  string       `json:"type"` // "blockquote"
	Spans []InlineSpan `json:"spans"`
}

type HRNode struct {
	Type string `json:"type"` // "hr"
}

type ImageNode struct {
	Type          string  `json:"type"` // "image"
	Alt           string  `json:"alt"`
	Src           string  `json:"src"`
	NaturalWidth  float64 `json:"naturalWidth,omitempty"`
	NaturalHeight float64 `json:"naturalHeight,omitempty"`
}

// Node is the discriminated union of all block types.
type Node struct {
	// Common
	Type string `json:"type"`

	// HeadingNode
	Level int          `json:"level,omitempty"`
	Text  string       `json:"text,omitempty"`
	Spans []InlineSpan `json:"spans,omitempty"`

	// ListNode
	Ordered     bool           `json:"ordered,omitempty"`
	Items       []string       `json:"items,omitempty"`
	ItemSpans   [][]InlineSpan `json:"itemSpans,omitempty"`
	ItemIndents []int          `json:"itemIndents,omitempty"` // ilvl per item; 0 = top level

	// TableNode
	Headers []string   `json:"headers,omitempty"`
	Rows    [][]string `json:"rows,omitempty"`

	// CodeBlockNode
	Language string `json:"language,omitempty"`

	// ImageNode
	Alt           string  `json:"alt,omitempty"`
	Src           string  `json:"src,omitempty"`
	NaturalWidth  float64 `json:"naturalWidth,omitempty"`
	NaturalHeight float64 `json:"naturalHeight,omitempty"`

	// Data holds in-memory binary payload (e.g. image bytes from a file upload).
	// Never JSON-serialised — only set by binary-format parsers at runtime.
	Data []byte `json:"-"`
}

type DocumentNode struct {
	Type     string `json:"type"` // "document"
	Children []Node `json:"children"`
}
