// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import "github.com/annavetech/annave-pdf-engine-golang/internal/ast"

// Parser recognizes and parses raw document bytes into an AST.
type Parser interface {
	CanParse(input []byte) bool
	Parse(input []byte) (*ast.DocumentNode, error)
}
