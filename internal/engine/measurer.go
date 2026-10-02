// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"embed"
	"fmt"
	"io/fs"
	"math"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/annavetech/pdfengine/internal/ast"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed fonts
var fontsFS embed.FS

const lineHeightMultiplier = 1.65
const cacheMax = 50_000

// TextSegment is a styled run of text ready for measurement.
type TextSegment struct {
	Text       string
	FontFamily string
	FontSize   float64
	FontWeight string
	FontStyle  string
}

// MeasuredToken is a word or space with its measured width.
type MeasuredToken struct {
	Text    string
	Width   float64
	Kind    string // "word" | "space"
	Segment TextSegment
}

// LayoutLine is a single line of measured tokens.
type LayoutLine struct {
	Tokens []MeasuredToken
	Width  float64
	Height float64
	Ascent float64
}

// TextMeasurer measures text using embedded Inter/JetBrains Mono fonts.
type TextMeasurer struct {
	mu          sync.Mutex
	parsedFonts map[string]*opentype.Font // "inter-400-normal" → parsed font
	faces       map[string]font.Face      // fontKey → face
	cache       map[string]float64        // "fontKey|text" → width in px
}

var (
	measurerOnce   sync.Once
	globalMeasurer *TextMeasurer
)

func GetMeasurer() *TextMeasurer {
	measurerOnce.Do(func() {
		m := &TextMeasurer{
			parsedFonts: make(map[string]*opentype.Font),
			faces:       make(map[string]font.Face),
			cache:       make(map[string]float64),
		}
		m.loadFonts()
		globalMeasurer = m
	})
	return globalMeasurer
}

func (m *TextMeasurer) loadFonts() {
	fontFiles := map[string]string{
		"inter-400-normal": "fonts/Inter-Regular.ttf",
		"inter-400-italic": "fonts/Inter-Italic.ttf",
		"inter-600-normal": "fonts/Inter-SemiBold.ttf",
		"inter-700-normal": "fonts/Inter-Bold.ttf",
		"inter-800-normal": "fonts/Inter-ExtraBold.ttf",
		"mono-400-normal":  "fonts/JetBrainsMono-Regular.ttf",
	}
	for key, path := range fontFiles {
		data, err := fs.ReadFile(fontsFS, path)
		if err != nil {
			continue
		}
		f, err := opentype.Parse(data)
		if err != nil {
			continue
		}
		m.parsedFonts[key] = f
	}
}

// fontKeyFor maps a TextSegment to one of the loaded font keys.
func (m *TextMeasurer) fontKeyFor(seg TextSegment) string {
	isMono := strings.Contains(seg.FontFamily, "JetBrains") ||
		strings.Contains(seg.FontFamily, "SF Mono") ||
		strings.Contains(seg.FontFamily, "Menlo") ||
		strings.Contains(seg.FontFamily, "Consolas") ||
		strings.Contains(seg.FontFamily, "monospace")

	family := "inter"
	if isMono {
		family = "mono"
	}

	weight := seg.FontWeight
	if weight == "" {
		weight = "400"
	}
	style := seg.FontStyle
	if style == "" || style == "normal" {
		style = "normal"
	} else {
		style = "italic"
	}
	// mono only has regular
	if family == "mono" {
		return "mono-400-normal"
	}
	key := family + "-" + weight + "-" + style
	if _, ok := m.parsedFonts[key]; !ok {
		// fallback
		return "inter-400-normal"
	}
	return key
}

func (m *TextMeasurer) faceFor(seg TextSegment) font.Face {
	fontKey := m.fontKeyFor(seg)
	cacheKey := fontKey + "|" + formatFloat(seg.FontSize)

	m.mu.Lock()
	defer m.mu.Unlock()

	if f, ok := m.faces[cacheKey]; ok {
		return f
	}

	parsed, ok := m.parsedFonts[fontKey]
	if !ok {
		parsed = m.parsedFonts["inter-400-normal"]
	}

	// Size in pt at 72 DPI = same as CSS px at 96 DPI for advance-width purposes.
	// We use DPI=72 so that 1pt = 1px, matching canvas.measureText() scale.
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size: seg.FontSize,
		DPI:  72,
	})
	if err != nil {
		return nil
	}
	m.faces[cacheKey] = face
	return face
}

func (m *TextMeasurer) measureText(text string, seg TextSegment) float64 {
	fontKey := m.fontKeyFor(seg)
	key := fontKey + "|" + formatFloat(seg.FontSize) + "|" + text

	m.mu.Lock()
	if w, ok := m.cache[key]; ok {
		m.mu.Unlock()
		return w
	}
	if len(m.cache) >= cacheMax {
		m.cache = make(map[string]float64)
	}
	m.mu.Unlock()

	face := m.faceFor(seg)
	if face == nil {
		// rough fallback: 0.6 * fontSize per character
		w := float64(len([]rune(text))) * seg.FontSize * 0.6
		m.mu.Lock()
		m.cache[key] = w
		m.mu.Unlock()
		return w
	}

	var advance fixed.Int26_6
	for _, r := range text {
		adv, ok := face.GlyphAdvance(r)
		if ok {
			advance += adv
		}
	}
	w := math.Round(float64(advance)/64.0*1000) / 1000

	m.mu.Lock()
	m.cache[key] = w
	m.mu.Unlock()
	return w
}

// MeasureWidth returns pixel width of text in the given style.
func (m *TextMeasurer) MeasureWidth(text string, style TextStyle) float64 {
	seg := TextSegment{
		Text:       text,
		FontFamily: style.FontFamily,
		FontSize:   style.FontSize,
		FontWeight: style.FontWeight,
		FontStyle:  style.FontStyle,
	}
	return m.measureText(text, seg)
}

// SpansToSegments converts InlineSpan[] + base style → TextSegment[].
func (m *TextMeasurer) SpansToSegments(spans []ast.InlineSpan, base TextStyle) []TextSegment {
	segs := make([]TextSegment, len(spans))
	for i, span := range spans {
		segs[i] = m.spanToSegment(span, base)
	}
	return segs
}

func (m *TextMeasurer) spanToSegment(span ast.InlineSpan, base TextStyle) TextSegment {
	switch span.Kind {
	case ast.SpanBold:
		return TextSegment{Text: span.Text, FontFamily: base.FontFamily, FontSize: base.FontSize, FontWeight: "700", FontStyle: "normal"}
	case ast.SpanItalic:
		return TextSegment{Text: span.Text, FontFamily: base.FontFamily, FontSize: base.FontSize, FontWeight: base.FontWeight, FontStyle: "italic"}
	case ast.SpanBoldItalic:
		return TextSegment{Text: span.Text, FontFamily: base.FontFamily, FontSize: base.FontSize, FontWeight: "700", FontStyle: "italic"}
	case ast.SpanCode:
		return TextSegment{Text: span.Text, FontFamily: FontMono, FontSize: base.FontSize - 2, FontWeight: "400", FontStyle: "normal"}
	default:
		return TextSegment{Text: span.Text, FontFamily: base.FontFamily, FontSize: base.FontSize, FontWeight: base.FontWeight, FontStyle: base.FontStyle}
	}
}

// TextToSegments creates a single-segment slice from plain text + style.
func (m *TextMeasurer) TextToSegments(text string, style TextStyle) []TextSegment {
	return []TextSegment{{
		Text:       text,
		FontFamily: style.FontFamily,
		FontSize:   style.FontSize,
		FontWeight: style.FontWeight,
		FontStyle:  style.FontStyle,
	}}
}

// BuildLines wraps TextSegment[] into LayoutLine[] respecting maxWidth.
func (m *TextMeasurer) BuildLines(segs []TextSegment, maxWidth float64) []LayoutLine {
	var lines []LayoutLine
	current := LayoutLine{}

	finalize := func() {
		if len(current.Tokens) > 0 {
			m.computeLineMetrics(&current)
			lines = append(lines, current)
			current = LayoutLine{}
		}
	}

	for _, seg := range segs {
		tokens := m.tokenize(seg)
		for _, tok := range tokens {
			if tok.Text == "\n" {
				finalize()
				continue
			}
			if tok.Kind == "space" {
				if len(current.Tokens) > 0 && current.Width+tok.Width <= maxWidth {
					current.Tokens = append(current.Tokens, tok)
					current.Width += tok.Width
				}
				continue
			}
			if current.Width+tok.Width <= maxWidth {
				current.Tokens = append(current.Tokens, tok)
				current.Width += tok.Width
				continue
			}
			if tok.Width > maxWidth {
				finalize()
				frags := m.splitWord(tok.Text, tok.Segment, maxWidth)
				for _, frag := range frags {
					if current.Width+frag.Width <= maxWidth {
						current.Tokens = append(current.Tokens, frag)
						current.Width += frag.Width
					} else {
						finalize()
						current.Tokens = append(current.Tokens, frag)
						current.Width = frag.Width
					}
				}
				continue
			}
			finalize()
			current.Tokens = append(current.Tokens, tok)
			current.Width = tok.Width
		}
	}
	finalize()

	if len(lines) == 0 {
		var emptySeg TextSegment
		if len(segs) > 0 {
			emptySeg = segs[0]
		}
		h := emptySeg.FontSize * lineHeightMultiplier
		return []LayoutLine{{Height: h, Ascent: h}}
	}
	return lines
}

func (m *TextMeasurer) tokenize(seg TextSegment) []MeasuredToken {
	var tokens []MeasuredToken
	parts := splitTokens(seg.Text)
	for _, part := range parts {
		if part == "" {
			continue
		}
		if part == "\n" {
			tokens = append(tokens, MeasuredToken{Text: "\n", Width: 0, Kind: "space", Segment: seg})
		} else if isSpaces(part) {
			w := m.measureText(part, seg)
			tokens = append(tokens, MeasuredToken{Text: part, Width: w, Kind: "space", Segment: seg})
		} else {
			w := m.measureText(part, seg)
			tokens = append(tokens, MeasuredToken{Text: part, Width: w, Kind: "word", Segment: seg})
		}
	}
	return tokens
}

// splitWord breaks word into the longest fragments that fit maxWidth.
// Each rune is measured once; widths use the same formula as measureText.
func (m *TextMeasurer) splitWord(word string, seg TextSegment, maxWidth float64) []MeasuredToken {
	face := m.faceFor(seg)
	width := func(adv fixed.Int26_6, runes int) float64 {
		if face == nil {
			return float64(runes) * seg.FontSize * 0.6
		}
		return math.Round(float64(adv)/64.0*1000) / 1000
	}

	var result []MeasuredToken
	start, runes := 0, 0
	var adv fixed.Int26_6
	for i, ch := range word {
		var chAdv fixed.Int26_6
		if face != nil {
			if a, ok := face.GlyphAdvance(ch); ok {
				chAdv = a
			}
		}
		if width(adv+chAdv, runes+1) <= maxWidth {
			adv += chAdv
			runes++
			continue
		}
		if runes > 0 {
			result = append(result, MeasuredToken{
				Text: word[start:i], Width: width(adv, runes), Kind: "word", Segment: seg,
			})
		}
		start, runes, adv = i, 1, chAdv
	}
	if runes > 0 {
		result = append(result, MeasuredToken{
			Text: word[start:], Width: width(adv, runes), Kind: "word", Segment: seg,
		})
	}
	return result
}

func (m *TextMeasurer) computeLineMetrics(line *LayoutLine) {
	var maxFontSize float64
	for _, t := range line.Tokens {
		if t.Segment.FontSize > maxFontSize {
			maxFontSize = t.Segment.FontSize
		}
	}
	line.Height = maxFontSize * lineHeightMultiplier
	line.Ascent = line.Height
}

// LinesHeight sums heights of all lines.
func (m *TextMeasurer) LinesHeight(lines []LayoutLine) float64 {
	var total float64
	for _, l := range lines {
		total += l.Height
	}
	return total
}

// WrapText wraps plain text into lines, returns line strings.
func (m *TextMeasurer) WrapText(text string, maxWidth float64, style TextStyle) []string {
	segs := m.TextToSegments(text, style)
	lines := m.BuildLines(segs, maxWidth)
	result := make([]string, len(lines))
	for i, l := range lines {
		var sb strings.Builder
		for _, t := range l.Tokens {
			sb.WriteString(t.Text)
		}
		result[i] = sb.String()
	}
	return result
}

// BlockHeight computes height of pre-wrapped lines.
func (m *TextMeasurer) BlockHeight(lines []string, style TextStyle) float64 {
	return float64(len(lines)) * style.FontSize * style.LineHeight
}

// SpansToText extracts plain text from spans.
func SpansToText(spans []ast.InlineSpan) string {
	var sb strings.Builder
	for _, s := range spans {
		sb.WriteString(s.Text)
	}
	return sb.String()
}

// ── helpers ──────────────────────────────────────────────────────────────────

// splitTokens splits s into "\n", runs of spaces and runs of other runes.
// Parts are substrings of s, so a long run costs O(len) rather than O(len^2).
func splitTokens(s string) []string {
	if !utf8.ValidString(s) {
		// Match range semantics: each invalid byte becomes U+FFFD.
		var b strings.Builder
		for _, r := range s {
			b.WriteRune(r)
		}
		s = b.String()
	}
	var parts []string
	start := 0
	inSpaces := false
	for i, r := range s {
		switch {
		case r == '\n':
			if i > start {
				parts = append(parts, s[start:i])
			}
			parts = append(parts, "\n")
			start = i + 1
		case (r == ' ') != inSpaces && i > start:
			parts = append(parts, s[start:i])
			start = i
		}
		if r != '\n' {
			inSpaces = r == ' '
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func isSpaces(s string) bool {
	for _, r := range s {
		if r != ' ' {
			return false
		}
	}
	return len(s) > 0
}

func formatFloat(f float64) string {
	s := fmt.Sprintf("%.3f", f)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
