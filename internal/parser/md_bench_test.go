// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"strings"
	"testing"
)

// benchInlineText is a 50-character string mixing bold, italic and inline
// code — the constructs ParseInline and stripInline actually branch on.
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
// with inline formatting, matching the scale the remediation spec measured
// MdParser.Parse against.
func benchMdDocument4000Items() string {
	var sb strings.Builder
	for i := 0; i < 4000; i++ {
		fmt.Fprintf(&sb, "- item %d with **bold** and _italic_ text\n", i)
	}
	return sb.String()
}

func BenchmarkMdParser_Parse(b *testing.B) {
	input := benchMdDocument4000Items()
	p := &MdParser{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Parse(input); err != nil {
			b.Fatal(err)
		}
	}
}
