// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package pdfengine

import "github.com/annavetech/pdfengine/internal/engine"

// Option configures an Engine at construction, in New. Options apply once,
// when the Engine is built; there is no per-Convert-call configuration.
type Option func(*options)

type options struct {
	style      *Style
	stylePath  string
	limitsPath string
}

// WithStyle overrides specific typography and page settings for this
// Engine. Fields left nil on s leave the corresponding default untouched.
func WithStyle(s Style) Option {
	return func(o *options) { o.style = &s }
}

// WithStyleFile replaces the embedded default config/style.yaml with the
// YAML file at path for this Engine.
func WithStyleFile(path string) Option {
	return func(o *options) { o.stylePath = path }
}

// WithLimitsFile replaces the embedded default config/limits.yaml with the
// YAML file at path for this Engine.
func WithLimitsFile(path string) Option {
	return func(o *options) { o.limitsPath = path }
}

// Style overrides specific typography and page settings on top of the
// engine's built-in defaults. A nil field leaves the default untouched.
type Style struct {
	Heading1   *TextStyle
	Heading2   *TextStyle
	Heading3   *TextStyle
	Paragraph  *TextStyle
	Code       *TextStyle
	Blockquote *TextStyle
	Page       *PageStyle
}

// TextStyle overrides the font and spacing settings for one block kind:
// a heading level, a paragraph, a code block, or a blockquote.
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
// structure the pipeline accepts.
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
