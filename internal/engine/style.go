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

type DocumentStyle struct {
	Page       PageConfig
	Heading1   TextStyle
	Heading2   TextStyle
	Heading3   TextStyle
	Paragraph  TextStyle
	Code       TextStyle
	Blockquote TextStyle
}

var DocStyle = DocumentStyle{
	Page: PageConfig{
		Width:        794,
		Height:       1123,
		MarginX:      56,
		MarginTop:    48,
		MarginBottom: 48,
	},
	Heading1: TextStyle{
		FontFamily:    FontSans,
		FontSize:      28,
		FontWeight:    "800",
		FontStyle:     "normal",
		LineHeight:    1.1,
		LetterSpacing: "-0.03em",
		MarginBottom:  20,
		Color:         "#1d1d1f",
	},
	Heading2: TextStyle{
		FontFamily:    FontSans,
		FontSize:      20,
		FontWeight:    "700",
		FontStyle:     "normal",
		LineHeight:    1.2,
		LetterSpacing: "-0.02em",
		MarginBottom:  16,
		Color:         "#1d1d1f",
	},
	Heading3: TextStyle{
		FontFamily:    FontSans,
		FontSize:      15,
		FontWeight:    "600",
		FontStyle:     "normal",
		LineHeight:    1.3,
		LetterSpacing: "-0.01em",
		MarginBottom:  12,
		Color:         "#1d1d1f",
	},
	Paragraph: TextStyle{
		FontFamily:    FontSans,
		FontSize:      13,
		FontWeight:    "400",
		FontStyle:     "normal",
		LineHeight:    1.65,
		LetterSpacing: "0",
		MarginBottom:  12,
		Color:         "#3a3a3c",
	},
	Code: TextStyle{
		FontFamily:    FontMono,
		FontSize:      11,
		FontWeight:    "400",
		FontStyle:     "normal",
		LineHeight:    1.6,
		LetterSpacing: "0",
		MarginBottom:  12,
		Color:         "#1d1d1f",
	},
	Blockquote: TextStyle{
		FontFamily:    FontSans,
		FontSize:      13,
		FontWeight:    "400",
		FontStyle:     "italic",
		LineHeight:    1.65,
		LetterSpacing: "0",
		MarginBottom:  12,
		Color:         "#3a3a3c",
	},
}
