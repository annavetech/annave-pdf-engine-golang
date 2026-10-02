// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"

	"github.com/annavetech/pdfengine/internal/ast"
)

// Parser recognizes and parses raw document bytes into an AST.
type Parser interface {
	CanParse(input []byte) bool
	Parse(input []byte) (*ast.DocumentNode, error)
}

// DepthLimitError reports that a nested document exceeds Limit levels.
type DepthLimitError struct {
	Format InputFormat
	Limit  int
}

func (e *DepthLimitError) Error() string {
	return fmt.Sprintf("%s: nesting exceeds the limit of %d levels", e.Format, e.Limit)
}
