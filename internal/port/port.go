// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

// Package port defines the primary and secondary port interfaces for the
// ANNAVE PDF Engine's hexagonal architecture.
//
// Hexagonal architecture separates the domain core from the outside world
// through explicit ports. The engine is the domain core; everything else
// is an adapter that implements one of these port interfaces.
//
// Primary ports (driven by the caller):
//   - Converter  — the application's single entry point for document conversion
//
// Secondary ports (driven by the engine):
//   - DocumentParser — transforms raw input into the canonical AST representation
//
// Adding a new delivery mechanism (gRPC, CLI, AWS Lambda) requires only
// wiring that mechanism to Converter. Adding a new input format requires
// only implementing DocumentParser.
package port

import "annave.tech/pdf-engine/internal/ast"

// Converter is the primary application port. Any delivery adapter — HTTP
// handler, gRPC server, CLI command — drives the engine through this interface.
//
// format is the IANA-style hint for the input ("md", "html", "json", …).
// Pass an empty string to trigger automatic format detection.
type Converter interface {
	Convert(input []byte, format string) (output []byte, err error)
}

// DocumentParser is the secondary input port. Each supported file format
// is an independent adapter that implements this interface.
//
// CanParse is called by the registry to probe a parser before committing.
// Implementations must be stateless and safe for concurrent use.
type DocumentParser interface {
	CanParse(input string) bool
	Parse(input string) (*ast.DocumentNode, error)
}
