// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package pdfengine

import "github.com/annavetech/annave-pdf-engine-golang/internal/engine"

// Option configures a single Convert call.
type Option func(*options)

type options struct {
	style *Style
}

// WithStyle overrides specific typography and page settings for one
// conversion, on top of the engine's built-in defaults. Fields left nil on
// s leave the corresponding default untouched.
func WithStyle(s Style) Option {
	return func(o *options) { o.style = &s }
}

// Style overrides specific typography and page settings on top of the
// engine's built-in defaults (config/style.yaml). All fields are optional;
// a nil field leaves the corresponding default untouched.
type Style struct {
	Heading1   *TextStyle
	Heading2   *TextStyle
	Heading3   *TextStyle
	Paragraph  *TextStyle
	Code       *TextStyle
	Blockquote *TextStyle
	Page       *PageStyle
}

// TextStyle overrides the font and spacing settings for one block kind
// (a heading level, a paragraph, a code block, or a blockquote). A nil
// field leaves the corresponding default untouched.
type TextStyle struct {
	FontSize     *float64
	FontWeight   *string
	FontStyle    *string
	LineHeight   *float64
	MarginBottom *float64
	Color        *string
}

// PageStyle overrides page margins, in points. A nil field leaves the
// corresponding default untouched.
type PageStyle struct {
	MarginX      *float64
	MarginTop    *float64
	MarginBottom *float64
}

// toStyleOverride translates the public Style into the internal override
// structure the pipeline accepts. Only the pointer fields are copied, never
// the internal type itself.
func toStyleOverride(s Style) *engine.StyleOverride {
	return &engine.StyleOverride{
		Heading1:   toPartialTextStyle(s.Heading1),
		Heading2:   toPartialTextStyle(s.Heading2),
		Heading3:   toPartialTextStyle(s.Heading3),
		Paragraph:  toPartialTextStyle(s.Paragraph),
		Code:       toPartialTextStyle(s.Code),
		Blockquote: toPartialTextStyle(s.Blockquote),
		Page:       toPartialPageConfig(s.Page),
	}
}

func toPartialTextStyle(t *TextStyle) *engine.PartialTextStyle {
	if t == nil {
		return nil
	}
	return &engine.PartialTextStyle{
		FontSize:     t.FontSize,
		FontWeight:   t.FontWeight,
		FontStyle:    t.FontStyle,
		LineHeight:   t.LineHeight,
		MarginBottom: t.MarginBottom,
		Color:        t.Color,
	}
}

func toPartialPageConfig(p *PageStyle) *engine.PartialPageConfig {
	if p == nil {
		return nil
	}
	return &engine.PartialPageConfig{
		MarginX:      p.MarginX,
		MarginTop:    p.MarginTop,
		MarginBottom: p.MarginBottom,
	}
}
