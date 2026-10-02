// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"strings"
	"testing"
)

// benchInlineText is a 50-character string mixing bold, italic, and inline
// code: the constructs ParseInline and stripInline branch on.
const benchInlineText = `This is **bold** and _italic_ and ` + "`code`" + `.`

func BenchmarkParseInline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseInline(benchInlineText)
	}
}

func BenchmarkStripInline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		stripInline(benchInlineText)
	}
}

// benchMdDocument4000Items builds an unordered list of 4,000 items, each
// with inline formatting.
func benchMdDocument4000Items() string {
	var sb strings.Builder
	for i := 0; i < 4000; i++ {
		fmt.Fprintf(&sb, "- item %d with **bold** and _italic_ text\n", i)
	}
	return sb.String()
}

func BenchmarkMdParser_Parse(b *testing.B) {
	input := []byte(benchMdDocument4000Items())
	p := &MdParser{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Parse(input); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMdParser_CanParse measures detection on about 500 KB of plain words, Markdown and bracket-heavy text.
func BenchmarkMdParser_CanParse(b *testing.B) {
	plain := "lorem ipsum dolor sit amet consectetur\n"
	inputs := []struct{ name, unit, tail string }{
		{"plain", plain, ""},
		{"markdown link first", "Some text with a [link](https://example.com) here.\n", ""},
		{"markdown link last", plain, "See [the docs](docs/a.md).\n"},
		{"markdown heading last", plain, "## Notes\n"},
		{"brackets", "[a](", ""},
	}
	p := &MdParser{}
	for _, in := range inputs {
		input := []byte(strings.Repeat(in.unit, 500_000/len(in.unit)) + in.tail)
		b.Run(in.name, func(b *testing.B) {
			b.SetBytes(int64(len(input)))
			for i := 0; i < b.N; i++ {
				p.CanParse(input)
			}
		})
	}
}
