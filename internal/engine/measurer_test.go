// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

var longWordStyle = TextStyle{FontFamily: FontSans, FontSize: 14, FontWeight: "400", FontStyle: "normal"}

// buildLongWord lays out one word of n runes with no spaces at a typical content width.
func buildLongWord(n int) []LayoutLine {
	m := GetMeasurer()
	return m.BuildLines(m.TextToSegments(strings.Repeat("a", n), longWordStyle), 700)
}

// bytesAllocated reports heap bytes allocated by one call of f.
func bytesAllocated(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// TestBuildLines_LongWordScalesLinearly guards against quadratic work on a word with no break points.
func TestBuildLines_LongWordScalesLinearly(t *testing.T) {
	small := bytesAllocated(func() { buildLongWord(50_000) })
	large := bytesAllocated(func() { buildLongWord(200_000) })
	// 4x the input should cost about 4x the memory; quadratic growth would be about 16x.
	if ratio := float64(large) / float64(small); ratio > 6 {
		t.Fatalf("allocation grew %.1fx for 4x input (50k: %d B, 200k: %d B), want about 4x", ratio, small, large)
	}
}

// TestBuildLines_LongWordFragmentsMatchMeasuredWidth checks fragment text, width and fit for a split word.
func TestBuildLines_LongWordFragmentsMatchMeasuredWidth(t *testing.T) {
	m := GetMeasurer()
	word := strings.Repeat("Wiqé", 500)
	const maxW = 300.0
	lines := m.BuildLines(m.TextToSegments(word, longWordStyle), maxW)

	var sb strings.Builder
	for i, l := range lines {
		if len(l.Tokens) != 1 {
			t.Fatalf("line %d has %d tokens, want 1", i, len(l.Tokens))
		}
		tok := l.Tokens[0]
		if want := m.MeasureWidth(tok.Text, longWordStyle); tok.Width != want {
			t.Fatalf("line %d width %v, want measured %v", i, tok.Width, want)
		}
		if tok.Width > maxW {
			t.Fatalf("line %d width %v exceeds %v", i, tok.Width, maxW)
		}
		if i < len(lines)-1 {
			next := []rune(lines[i+1].Tokens[0].Text)[0]
			if m.MeasureWidth(tok.Text+string(next), longWordStyle) <= maxW {
				t.Fatalf("line %d breaks early: next rune %q still fits", i, next)
			}
		}
		sb.WriteString(tok.Text)
	}
	if sb.String() != word {
		t.Fatal("fragments do not join back into the original word")
	}
}

// refSplitTokens is the previous concatenating splitter, kept as the reference output.
func refSplitTokens(s string) []string {
	var parts []string
	cur := ""
	for _, r := range s {
		switch r {
		case '\n':
			if cur != "" {
				parts = append(parts, cur)
				cur = ""
			}
			parts = append(parts, "\n")
		case ' ':
			if cur != "" && !isSpaces(cur) {
				parts = append(parts, cur)
				cur = ""
			}
			cur += string(r)
		default:
			if isSpaces(cur) {
				parts = append(parts, cur)
				cur = ""
			}
			cur += string(r)
		}
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}

// refSplitWord is the previous prefix-measuring splitter, kept as the reference output.
func refSplitWord(m *TextMeasurer, word string, seg TextSegment, maxWidth float64) []MeasuredToken {
	var result []MeasuredToken
	fragment := ""
	for _, ch := range word {
		candidate := fragment + string(ch)
		if m.measureText(candidate, seg) <= maxWidth {
			fragment = candidate
			continue
		}
		if fragment != "" {
			result = append(result, MeasuredToken{Text: fragment, Width: m.measureText(fragment, seg), Kind: "word", Segment: seg})
		}
		fragment = string(ch)
	}
	if fragment != "" {
		result = append(result, MeasuredToken{Text: fragment, Width: m.measureText(fragment, seg), Kind: "word", Segment: seg})
	}
	return result
}

// TestSplitTokensAndSplitWord_MatchReference compares both splitters with the previous output
// on every string of up to four symbols, each also repeated to form longer runs.
func TestSplitTokensAndSplitWord_MatchReference(t *testing.T) {
	m := GetMeasurer()
	alphabet := []string{"a", "W", "i", " ", "  ", "\n", "\u00e9", "\u4e2d", "\U0001F600", "\xff", "\xe2\x82", "-", "."}
	segs := []TextSegment{
		{FontFamily: FontSans, FontSize: 14, FontWeight: "400", FontStyle: "normal"},
		{FontFamily: FontSans, FontSize: 14, FontWeight: "700", FontStyle: "italic"},
		{FontFamily: FontMono, FontSize: 12, FontWeight: "400", FontStyle: "normal"},
	}
	widths := []float64{1, 9.5, 30, 70}

	inputs := []string{""}
	for frontier := []string{""}; len(frontier[0]) < 4; {
		var next []string
		for _, prefix := range frontier {
			for _, sym := range alphabet {
				next = append(next, prefix+sym)
			}
		}
		inputs = append(inputs, next...)
		frontier = next
	}

	for i, base := range inputs {
		for _, s := range []string{base, strings.Repeat(base, 8)} {
			got, want := splitTokens(s), refSplitTokens(s)
			if !slices.Equal(got, want) {
				t.Fatalf("splitTokens(%q) = %q, want %q", s, got, want)
			}
			seg, maxW := segs[i%len(segs)], widths[i%len(widths)]
			for _, part := range got {
				gotW, wantW := m.splitWord(part, seg, maxW), refSplitWord(m, part, seg, maxW)
				if !slices.Equal(gotW, wantW) {
					t.Fatalf("splitWord(%q, %v) = %+v, want %+v", part, maxW, gotW, wantW)
				}
			}
		}
	}
}

func BenchmarkBuildLines_LongWord(b *testing.B) {
	for _, n := range []int{50_000, 100_000, 200_000, 500_000} {
		b.Run(fmt.Sprintf("%dk", n/1000), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				buildLongWord(n)
			}
		})
	}
}

// TestBuildLines_NoFontFallbackSplitsLongWord forces the no-face path and checks fragments use 0.6 x font size per rune.
func TestBuildLines_NoFontFallbackSplitsLongWord(t *testing.T) {
	m := &TextMeasurer{
		parsedFonts: map[string]*opentype.Font{},
		faces:       map[string]font.Face{},
		cache:       map[string]float64{},
	}
	style := TextStyle{FontFamily: FontSans, FontSize: 10, FontWeight: "400", FontStyle: "normal"}
	segs := m.TextToSegments("éaéaéaé", style)
	// A cached nil face makes faceFor return nil, as when no font can be loaded.
	m.faces[m.fontKeyFor(segs[0])+"|"+formatFloat(style.FontSize)] = nil

	lines := m.BuildLines(segs, 20)
	want := []string{"éaé", "aéa", "é"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, l := range lines {
		if len(l.Tokens) != 1 {
			t.Fatalf("line %d has %d tokens, want 1", i, len(l.Tokens))
		}
		tok := l.Tokens[0]
		wantW := float64(len([]rune(want[i]))) * style.FontSize * 0.6
		if tok.Text != want[i] || tok.Width != wantW {
			t.Errorf("line %d = %q width %v, want %q width %v", i, tok.Text, tok.Width, want[i], wantW)
		}
	}
}
