// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"errors"
	"strings"
	"testing"

	"github.com/annavetech/pdfengine/internal/ast"
)

// nestedXML returns levels nested <l> elements, the root counting as level 1.
func nestedXML(levels int) []byte {
	return []byte(strings.Repeat("<l>", levels) + "x" + strings.Repeat("</l>", levels))
}

func TestXmlParser_Parse_AtDepthLimitParses(t *testing.T) {
	doc, err := (&XmlParser{}).Parse(nestedXML(maxXMLDepth))
	if err != nil {
		t.Fatalf("Parse() error at %d levels: %v", maxXMLDepth, err)
	}
	if len(doc.Children) == 0 || doc.Children[0].Type != ast.TypeHeading || doc.Children[0].Text != "l" {
		t.Fatalf("expected root heading %q, got %+v", "l", doc.Children)
	}
}

func TestXmlParser_Parse_PastDepthLimitReturnsDepthLimitError(t *testing.T) {
	doc, err := (&XmlParser{}).Parse(nestedXML(maxXMLDepth + 1))
	var depthErr *DepthLimitError
	if !errors.As(err, &depthErr) {
		t.Fatalf("Parse() error = %v, doc = %v; want *DepthLimitError", err, doc)
	}
	if depthErr.Format != FormatXML || depthErr.Limit != maxXMLDepth {
		t.Errorf("DepthLimitError = %+v, want Format %q Limit %d", depthErr, FormatXML, maxXMLDepth)
	}
}

func TestXmlParser_Parse_MalformedFallsBackToText(t *testing.T) {
	input := []byte("<root><a>unclosed</root>")
	doc, err := (&XmlParser{}).Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(doc.Children) != 1 || doc.Children[0].Type != ast.TypeParagraph || doc.Children[0].Text != string(input) {
		t.Fatalf("expected plain-text fallback, got %+v", doc.Children)
	}
}
