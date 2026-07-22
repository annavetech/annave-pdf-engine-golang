// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"fmt"

	"annave.tech/pdf-engine/internal/ast"
)

var validBlockTypes = map[string]bool{
	ast.TypeHeading: true, ast.TypeParagraph: true, ast.TypeList: true,
	ast.TypeTable: true, ast.TypeCodeBlock: true, ast.TypeBlockquote: true,
	ast.TypeHR: true, ast.TypeImage: true,
}

var validSpanKinds = map[string]bool{
	ast.SpanText: true, ast.SpanBold: true, ast.SpanItalic: true,
	ast.SpanBoldItalic: true, ast.SpanCode: true, ast.SpanStrike: true, ast.SpanLink: true,
}

func ValidateDocument(doc *ast.DocumentNode) error {
	if doc == nil {
		return NewError("ENGINE_ERR_INVALID_DOCUMENT", StageValidation, "Document must not be nil.")
	}
	if doc.Type != ast.TypeDocument {
		return NewError("ENGINE_ERR_INVALID_DOCUMENT", StageValidation, `Document type must be "document".`)
	}
	maxNodes := appLimits.Document.MaxNodes
	if len(doc.Children) > maxNodes {
		return NewError("ENGINE_ERR_TOO_MANY_NODES", StageValidation,
			msg("ENGINE_ERR_TOO_MANY_NODES", "max_nodes", maxNodes))
	}
	for i, node := range doc.Children {
		if err := validateNode(&node, i); err != nil {
			return err
		}
	}
	return nil
}

func validateNode(n *ast.Node, index int) error {
	if !validBlockTypes[n.Type] {
		return NewError("ENGINE_ERR_INVALID_NODE", StageValidation,
			fmt.Sprintf(`Unknown node type "%s" at index %d.`, n.Type, index))
	}

	switch n.Type {
	case ast.TypeHeading:
		if n.Text == "" {
			return NewError("ENGINE_ERR_INVALID_NODE", StageValidation, fmt.Sprintf("Heading at %d missing text.", index))
		}
		if n.Level < 1 || n.Level > 3 {
			return NewError("ENGINE_ERR_INVALID_NODE", StageValidation, fmt.Sprintf("Heading at %d has invalid level.", index))
		}
		if err := validateSpans(n.Spans, index); err != nil {
			return err
		}
	case ast.TypeParagraph:
		if n.Text == "" {
			return NewError("ENGINE_ERR_INVALID_NODE", StageValidation, fmt.Sprintf("Paragraph at %d missing text.", index))
		}
		if err := validateSpans(n.Spans, index); err != nil {
			return err
		}
	case ast.TypeList:
		if n.Items == nil {
			return NewError("ENGINE_ERR_INVALID_NODE", StageValidation, fmt.Sprintf("List at %d missing items.", index))
		}
	case ast.TypeTable:
		if n.Headers == nil {
			return NewError("ENGINE_ERR_INVALID_NODE", StageValidation, fmt.Sprintf("Table at %d missing headers.", index))
		}
	case ast.TypeCodeBlock:
		if n.Text == "" {
			return NewError("ENGINE_ERR_INVALID_NODE", StageValidation, fmt.Sprintf("Code block at %d missing text.", index))
		}
	case ast.TypeBlockquote:
		if err := validateSpans(n.Spans, index); err != nil {
			return err
		}
	case ast.TypeImage:
		if n.Src == "" && len(n.Data) == 0 {
			return NewError("ENGINE_ERR_INVALID_NODE", StageValidation, fmt.Sprintf("Image at %d missing src or data.", index))
		}
	}
	return nil
}

func validateSpans(spans []ast.InlineSpan, nodeIndex int) error {
	for _, s := range spans {
		if !validSpanKinds[s.Kind] {
			return NewError("ENGINE_ERR_INVALID_NODE", StageValidation,
				fmt.Sprintf(`Unknown span kind "%s" at node %d.`, s.Kind, nodeIndex))
		}
	}
	return nil
}
