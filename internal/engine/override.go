// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

// StyleOverride is the JSON-decodable structure accepted in the ?style= query
// parameter or the `style` multipart/form field. All fields are optional;
// only the fields present in the JSON are applied on top of the defaults from
// config/style.yaml.
//
// Example JSON:
//
//	{
//	  "paragraph": { "fontSize": 14, "lineHeight": 1.8 },
//	  "heading1":  { "color": "#cc0000" },
//	  "page":      { "marginX": 72 }
//	}
type StyleOverride struct {
	Heading1   *PartialTextStyle  `json:"heading1,omitempty"`
	Heading2   *PartialTextStyle  `json:"heading2,omitempty"`
	Heading3   *PartialTextStyle  `json:"heading3,omitempty"`
	Paragraph  *PartialTextStyle  `json:"paragraph,omitempty"`
	Code       *PartialTextStyle  `json:"code,omitempty"`
	Blockquote *PartialTextStyle  `json:"blockquote,omitempty"`
	Page       *PartialPageConfig `json:"page,omitempty"`
}

// PartialTextStyle mirrors TextStyle with all fields optional (pointer).
type PartialTextStyle struct {
	FontSize     *float64 `json:"fontSize,omitempty"`
	FontWeight   *string  `json:"fontWeight,omitempty"`
	FontStyle    *string  `json:"fontStyle,omitempty"`
	LineHeight   *float64 `json:"lineHeight,omitempty"`
	MarginBottom *float64 `json:"marginBottom,omitempty"`
	Color        *string  `json:"color,omitempty"`
}

// PartialPageConfig mirrors PageConfig with all fields optional (pointer).
type PartialPageConfig struct {
	MarginX      *float64 `json:"marginX,omitempty"`
	MarginTop    *float64 `json:"marginTop,omitempty"`
	MarginBottom *float64 `json:"marginBottom,omitempty"`
}

// MergeDocStyle returns a copy of base with any non-nil fields from override applied.
func MergeDocStyle(base DocumentStyle, o *StyleOverride) DocumentStyle {
	if o == nil {
		return base
	}
	if o.Heading1 != nil {
		base.Heading1 = applyTextStyle(base.Heading1, o.Heading1)
	}
	if o.Heading2 != nil {
		base.Heading2 = applyTextStyle(base.Heading2, o.Heading2)
	}
	if o.Heading3 != nil {
		base.Heading3 = applyTextStyle(base.Heading3, o.Heading3)
	}
	if o.Paragraph != nil {
		base.Paragraph = applyTextStyle(base.Paragraph, o.Paragraph)
	}
	if o.Code != nil {
		base.Code = applyTextStyle(base.Code, o.Code)
	}
	if o.Blockquote != nil {
		base.Blockquote = applyTextStyle(base.Blockquote, o.Blockquote)
	}
	if o.Page != nil {
		base.Page = applyPageConfig(base.Page, o.Page)
	}
	return base
}

func applyTextStyle(base TextStyle, p *PartialTextStyle) TextStyle {
	if p.FontSize != nil {
		base.FontSize = *p.FontSize
	}
	if p.FontWeight != nil {
		base.FontWeight = *p.FontWeight
	}
	if p.FontStyle != nil {
		base.FontStyle = *p.FontStyle
	}
	if p.LineHeight != nil {
		base.LineHeight = *p.LineHeight
	}
	if p.MarginBottom != nil {
		base.MarginBottom = *p.MarginBottom
	}
	if p.Color != nil {
		base.Color = *p.Color
	}
	return base
}

func applyPageConfig(base PageConfig, p *PartialPageConfig) PageConfig {
	if p.MarginX != nil {
		base.MarginX = *p.MarginX
	}
	if p.MarginTop != nil {
		base.MarginTop = *p.MarginTop
	}
	if p.MarginBottom != nil {
		base.MarginBottom = *p.MarginBottom
	}
	return base
}
