// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import "github.com/annavetech/annave-pdf-engine-golang/internal/ast"

type Parser interface {
	CanParse(input string) bool
	Parse(input string) (*ast.DocumentNode, error)
}
