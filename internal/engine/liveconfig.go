// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	engineconfig "github.com/annavetech/annave-pdf-engine-golang/config"
	"gopkg.in/yaml.v3"
)

// Config holds everything one Engine needs to convert documents: input/output
// limits, message templates, and document style, built once by LoadConfig.
type Config struct {
	Limits   LimitsConfig
	Messages MessagesConfig
	Style    DocumentStyle
}

// LimitsConfig mirrors config/limits.yaml: the input and output bounds the
// pipeline enforces.
type LimitsConfig struct {
	Input struct {
		MaxFileSizeBytes int `yaml:"max_file_size_bytes"`
		MaxInputChars    int `yaml:"max_input_chars"`
	} `yaml:"input"`
	Document struct {
		MaxNodes int `yaml:"max_nodes"`
		MaxPages int `yaml:"max_pages"`
	} `yaml:"document"`
}

// MessagesConfig mirrors config/messages.yaml: the user-facing message
// template for each ENGINE_ERR_* code.
type MessagesConfig struct {
	Errors map[string]string `yaml:"errors"`
}

// msg returns the configured message for code, interpolating {key}
// placeholders with the given (key, value, ...) pairs. Unknown codes return a placeholder string.
func (c *Config) msg(code string, args ...any) string {
	template, ok := c.Messages.Errors[code]
	if !ok {
		return fmt.Sprintf("unknown error code: %s", code)
	}
	for i := 0; i+1 < len(args); i += 2 {
		k := fmt.Sprintf("{%v}", args[i])
		v := fmt.Sprintf("%v", args[i+1])
		template = strings.ReplaceAll(template, k, v)
	}
	return template
}

// configState accumulates the choices made by ConfigOption values passed to
// LoadConfig before any file is read or any YAML is parsed.
type configState struct {
	limitsPath    string
	stylePath     string
	styleOverride *StyleOverride
}

// ConfigOption configures a single call to LoadConfig.
type ConfigOption func(*configState)

// WithLimitsFile overrides the embedded default config/limits.yaml with the
// YAML file at path.
func WithLimitsFile(path string) ConfigOption {
	return func(s *configState) { s.limitsPath = path }
}

// WithStyleFile overrides the embedded default config/style.yaml with the
// YAML file at path.
func WithStyleFile(path string) ConfigOption {
	return func(s *configState) { s.stylePath = path }
}

// WithStyleOverride applies field-level style overrides on top of whichever
// style base LoadConfig otherwise resolves to. Only the fields set on o are applied.
func WithStyleOverride(o *StyleOverride) ConfigOption {
	return func(s *configState) { s.styleOverride = o }
}

// LoadConfig builds a Config from the embedded default limits, messages, and
// style, applying any ConfigOption overrides, and returns an error on failure.
func LoadConfig(opts ...ConfigOption) (*Config, error) {
	st := &configState{}
	for _, o := range opts {
		o(st)
	}

	limits, err := loadLimits(st.limitsPath)
	if err != nil {
		return nil, err
	}

	var messages MessagesConfig
	if err := yaml.Unmarshal(engineconfig.Messages, &messages); err != nil {
		// A failure here is a bug in this repo, not a caller mistake, but it
		// must still be reported rather than exiting the host process.
		return nil, fmt.Errorf("engine: parse embedded messages config: %w", err)
	}

	style, err := loadStyle(st.stylePath)
	if err != nil {
		return nil, err
	}
	style = MergeDocStyle(style, st.styleOverride)

	return &Config{Limits: limits, Messages: messages, Style: style}, nil
}

func loadLimits(path string) (LimitsConfig, error) {
	raw := engineconfig.Limits
	if path != "" {
		b, err := os.ReadFile(path) //nolint:gosec // path is caller-supplied by design; see WithLimitsFile.
		if err != nil {
			return LimitsConfig{}, fmt.Errorf("engine: read limits file %q: %w", path, err)
		}
		raw = b
	}

	var limits LimitsConfig
	if err := yaml.Unmarshal(raw, &limits); err != nil {
		return LimitsConfig{}, fmt.Errorf("engine: parse limits config: %w", err)
	}
	if err := validateLimits(limits); err != nil {
		return LimitsConfig{}, fmt.Errorf("engine: invalid limits config: %w", err)
	}
	return limits, nil
}

// validateLimits enforces that every bound is a usable positive number; a
// zero or negative limit is a misconfiguration, not a stricter setting.
func validateLimits(l LimitsConfig) error {
	var problems []string
	if l.Input.MaxFileSizeBytes <= 0 {
		problems = append(problems, "input.max_file_size_bytes must be greater than 0")
	}
	if l.Input.MaxInputChars <= 0 {
		problems = append(problems, "input.max_input_chars must be greater than 0")
	}
	if l.Document.MaxNodes <= 0 {
		problems = append(problems, "document.max_nodes must be greater than 0")
	}
	if l.Document.MaxPages <= 0 {
		problems = append(problems, "document.max_pages must be greater than 0")
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

// styleYAML mirrors config/style.yaml's on-disk shape and is resolved into
// DocumentStyle by loadStyle; it exists only as a decode target.
type styleYAML struct {
	Page struct {
		WidthPx        float64 `yaml:"width_px"`
		HeightPx       float64 `yaml:"height_px"`
		MarginXPx      float64 `yaml:"margin_x_px"`
		MarginTopPx    float64 `yaml:"margin_top_px"`
		MarginBottomPx float64 `yaml:"margin_bottom_px"`
	} `yaml:"page"`
	Fonts struct {
		Sans string `yaml:"sans"`
		Mono string `yaml:"mono"`
	} `yaml:"fonts"`
	TextStyles struct {
		Heading1   styleYAMLTextStyle `yaml:"heading1"`
		Heading2   styleYAMLTextStyle `yaml:"heading2"`
		Heading3   styleYAMLTextStyle `yaml:"heading3"`
		Paragraph  styleYAMLTextStyle `yaml:"paragraph"`
		Code       styleYAMLTextStyle `yaml:"code"`
		Blockquote styleYAMLTextStyle `yaml:"blockquote"`
	} `yaml:"text_styles"`
}

type styleYAMLTextStyle struct {
	FontFamily     string  `yaml:"font_family"`
	FontSizePx     float64 `yaml:"font_size_px"`
	FontWeight     string  `yaml:"font_weight"`
	FontStyle      string  `yaml:"font_style"`
	LineHeight     float64 `yaml:"line_height"`
	MarginBottomPx float64 `yaml:"margin_bottom_px"`
	Color          string  `yaml:"color"`
}

func loadStyle(path string) (DocumentStyle, error) {
	raw := engineconfig.Style
	if path != "" {
		b, err := os.ReadFile(path) //nolint:gosec // path is caller-supplied by design; see WithStyleFile.
		if err != nil {
			return DocumentStyle{}, fmt.Errorf("engine: read style file %q: %w", path, err)
		}
		raw = b
	}

	var y styleYAML
	if err := yaml.Unmarshal(raw, &y); err != nil {
		return DocumentStyle{}, fmt.Errorf("engine: parse style config: %w", err)
	}

	fonts := map[string]string{"sans": y.Fonts.Sans, "mono": y.Fonts.Mono}
	if fonts["sans"] == "" {
		fonts["sans"] = FontSans
	}
	if fonts["mono"] == "" {
		fonts["mono"] = FontMono
	}

	resolve := func(field string, s styleYAMLTextStyle) (TextStyle, error) {
		family, ok := fonts[s.FontFamily]
		if !ok {
			return TextStyle{}, fmt.Errorf("%s.font_family: %q is not %q or %q", field, s.FontFamily, "sans", "mono")
		}
		if err := validateTextStyle(field, s); err != nil {
			return TextStyle{}, err
		}
		return TextStyle{
			FontFamily:   family,
			FontSize:     s.FontSizePx,
			FontWeight:   s.FontWeight,
			FontStyle:    s.FontStyle,
			LineHeight:   s.LineHeight,
			MarginBottom: s.MarginBottomPx,
			Color:        s.Color,
		}, nil
	}

	ds := DocumentStyle{
		Page: PageConfig{
			Width:        y.Page.WidthPx,
			Height:       y.Page.HeightPx,
			MarginX:      y.Page.MarginXPx,
			MarginTop:    y.Page.MarginTopPx,
			MarginBottom: y.Page.MarginBottomPx,
		},
	}
	if err := validatePage(ds.Page); err != nil {
		return DocumentStyle{}, fmt.Errorf("engine: invalid style config: page: %w", err)
	}

	var err error
	if ds.Heading1, err = resolve("text_styles.heading1", y.TextStyles.Heading1); err != nil {
		return DocumentStyle{}, fmt.Errorf("engine: invalid style config: %w", err)
	}
	if ds.Heading2, err = resolve("text_styles.heading2", y.TextStyles.Heading2); err != nil {
		return DocumentStyle{}, fmt.Errorf("engine: invalid style config: %w", err)
	}
	if ds.Heading3, err = resolve("text_styles.heading3", y.TextStyles.Heading3); err != nil {
		return DocumentStyle{}, fmt.Errorf("engine: invalid style config: %w", err)
	}
	if ds.Paragraph, err = resolve("text_styles.paragraph", y.TextStyles.Paragraph); err != nil {
		return DocumentStyle{}, fmt.Errorf("engine: invalid style config: %w", err)
	}
	if ds.Code, err = resolve("text_styles.code", y.TextStyles.Code); err != nil {
		return DocumentStyle{}, fmt.Errorf("engine: invalid style config: %w", err)
	}
	if ds.Blockquote, err = resolve("text_styles.blockquote", y.TextStyles.Blockquote); err != nil {
		return DocumentStyle{}, fmt.Errorf("engine: invalid style config: %w", err)
	}

	return ds, nil
}

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func validateTextStyle(field string, s styleYAMLTextStyle) error {
	if s.FontSizePx <= 0 {
		return fmt.Errorf("%s.font_size_px must be greater than 0", field)
	}
	switch s.FontWeight {
	case "400", "600", "700", "800":
	default:
		return fmt.Errorf("%s.font_weight: %q is not one of 400, 600, 700, 800", field, s.FontWeight)
	}
	switch s.FontStyle {
	case "normal", "italic":
	default:
		return fmt.Errorf(`%s.font_style: %q is not "normal" or "italic"`, field, s.FontStyle)
	}
	if s.LineHeight <= 0 {
		return fmt.Errorf("%s.line_height must be greater than 0", field)
	}
	if s.MarginBottomPx < 0 {
		return fmt.Errorf("%s.margin_bottom_px must not be negative", field)
	}
	if !hexColorRe.MatchString(s.Color) {
		return fmt.Errorf("%s.color: %q is not a 7-character #rrggbb hex colour", field, s.Color)
	}
	return nil
}

func validatePage(p PageConfig) error {
	if p.Width <= 0 || p.Height <= 0 {
		return fmt.Errorf("width_px and height_px must be greater than 0")
	}
	if p.MarginX < 0 || p.MarginTop < 0 || p.MarginBottom < 0 {
		return fmt.Errorf("margins must not be negative")
	}
	if p.MarginX*2 >= p.Width {
		return fmt.Errorf("margin_x_px is too large: two margins must leave a positive text column width")
	}
	return nil
}
