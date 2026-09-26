// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

// A4 at 96 DPI: 794 × 1123 px
// All units in CSS pixels, matching the TypeScript engine exactly.

const (
	FontSans = `"Inter", -apple-system, "SF Pro Display", BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif`
	FontMono = `"JetBrains Mono", "SF Mono", "Menlo", "Monaco", "Consolas", "Liberation Mono", monospace`
)

type TextStyle struct {
	FontFamily    string
	FontSize      float64 // px
	FontWeight    string  // "400" | "600" | "700" | "800"
	FontStyle     string  // "normal" | "italic"
	LineHeight    float64 // multiplier
	LetterSpacing string
	MarginBottom  float64 // px
	Color         string
}

type PageConfig struct {
	Width        float64
	Height       float64
	MarginX      float64
	MarginTop    float64
	MarginBottom float64
}

// DocumentStyle is the fully resolved set of page and text styles one
// Engine renders with.
type DocumentStyle struct {
	Page       PageConfig
	Heading1   TextStyle
	Heading2   TextStyle
	Heading3   TextStyle
	Paragraph  TextStyle
	Code       TextStyle
	Blockquote TextStyle
}
