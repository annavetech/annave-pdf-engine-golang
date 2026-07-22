// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/xml"
	"strings"

	"annave.tech/pdf-engine/internal/ast"
)

type XmlParser struct{}

func (p *XmlParser) CanParse(input string) bool {
	t := strings.TrimSpace(input)
	return (strings.HasPrefix(t, "<?xml") || (strings.HasPrefix(t, "<") && !strings.HasPrefix(t, "<!"))) &&
		strings.Contains(t, ">")
}

func (p *XmlParser) Parse(input string) (*ast.DocumentNode, error) {
	root, err := parseXMLRoot(input)
	if err != nil {
		return fallbackDoc(input), nil
	}
	var children []ast.Node
	children = append(children, ast.Node{
		Type:  ast.TypeHeading,
		Level: 1,
		Text:  root.XMLName.Local,
		Spans: []ast.InlineSpan{{Kind: ast.SpanText, Text: root.XMLName.Local}},
	})
	if tbl := xmlAttrsToTable(root.Attrs); tbl != nil {
		children = append(children, *tbl)
	}
	xmlWalkChildren(root.Children, 2, &children)
	return &ast.DocumentNode{Type: ast.TypeDocument, Children: children}, nil
}

type xmlNode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr
	Children []xmlNode
	Content  string
}

func (n *xmlNode) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	n.XMLName = start.Name
	n.Attrs = start.Attr
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var child xmlNode
			if err := d.DecodeElement(&child, &t); err == nil {
				n.Children = append(n.Children, child)
			}
		case xml.CharData:
			n.Content += string(t)
		case xml.EndElement:
			return nil
		}
	}
	return nil
}

func parseXMLRoot(input string) (*xmlNode, error) {
	var root xmlNode
	if err := xml.Unmarshal([]byte(input), &root); err != nil {
		return nil, err
	}
	return &root, nil
}

func xmlWalkChildren(nodes []xmlNode, depth int, out *[]ast.Node) {
	for _, child := range nodes {
		tag := child.XMLName.Local
		text := strings.TrimSpace(child.Content)
		if len(child.Children) == 0 {
			if text != "" {
				*out = append(*out, ast.Node{
					Type:  ast.TypeParagraph,
					Text:  tag + ": " + text,
					Spans: []ast.InlineSpan{{Kind: ast.SpanBold, Text: tag + ": "}, {Kind: ast.SpanText, Text: text}},
				})
			} else {
				level := depth
				if level > 3 {
					level = 3
				}
				*out = append(*out, ast.Node{
					Type:  ast.TypeHeading,
					Level: level,
					Text:  tag,
					Spans: []ast.InlineSpan{{Kind: ast.SpanText, Text: tag}},
				})
			}
		} else {
			level := depth
			if level > 3 {
				level = 3
			}
			*out = append(*out, ast.Node{
				Type:  ast.TypeHeading,
				Level: level,
				Text:  tag,
				Spans: []ast.InlineSpan{{Kind: ast.SpanText, Text: tag}},
			})
			if tbl := xmlAttrsToTable(child.Attrs); tbl != nil {
				*out = append(*out, *tbl)
			}
			if depth < 6 {
				xmlWalkChildren(child.Children, depth+1, out)
			} else if text != "" {
				*out = append(*out, ast.Node{Type: ast.TypeParagraph, Text: text, Spans: ParseInline(text)})
			}
		}
	}
}

func xmlAttrsToTable(attrs []xml.Attr) *ast.Node {
	if len(attrs) == 0 {
		return nil
	}
	rows := make([][]string, len(attrs))
	for i, a := range attrs {
		rows[i] = []string{a.Name.Local, a.Value}
	}
	n := ast.Node{Type: ast.TypeTable, Headers: []string{"Attribute", "Value"}, Rows: rows}
	return &n
}

func fallbackDoc(input string) *ast.DocumentNode {
	t := strings.TrimSpace(input)
	return &ast.DocumentNode{
		Type:     ast.TypeDocument,
		Children: []ast.Node{{Type: ast.TypeParagraph, Text: t, Spans: []ast.InlineSpan{{Kind: ast.SpanText, Text: t}}}},
	}
}
