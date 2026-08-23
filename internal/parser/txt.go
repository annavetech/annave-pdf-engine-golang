// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"regexp"
	"strings"

	"github.com/annavetech/annave-pdf-engine-golang/internal/ast"
)

const headingMaxChars = 72

var (
	txtUnorderedItem = regexp.MustCompile(`^[-*•]\s+(.+)$`)
	txtOrderedItem   = regexp.MustCompile(`^\d+[.)]\s+(.+)$`)
	sentenceEnd      = regexp.MustCompile(`[.!?,;:]$`)
)

type TxtParser struct{}

func (p *TxtParser) CanParse(_ string) bool { return true }

func (p *TxtParser) Parse(input string) (*ast.DocumentNode, error) {
	var children []ast.Node
	blocks := splitBlocks(input)

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := nonEmptyLines(block)
		if len(lines) == 0 {
			continue
		}

		// Unordered list
		if len(lines) > 1 && allMatch(lines, txtUnorderedItem) {
			items := extractGroup(lines, txtUnorderedItem, 1)
			children = append(children, ast.Node{Type: ast.TypeList, Ordered: false, Items: items})
			continue
		}

		// Ordered list
		if len(lines) > 1 && allMatch(lines, txtOrderedItem) {
			items := extractGroup(lines, txtOrderedItem, 1)
			children = append(children, ast.Node{Type: ast.TypeList, Ordered: true, Items: items})
			continue
		}

		// Heading heuristic
		if len(lines) == 1 && len(block) <= headingMaxChars && !sentenceEnd.MatchString(block) {
			children = append(children, ast.Node{Type: ast.TypeHeading, Level: 1, Text: block})
			continue
		}

		// Paragraph
		text := strings.ReplaceAll(block, "\n", " ")
		children = append(children, ast.Node{Type: ast.TypeParagraph, Text: text})
	}

	return &ast.DocumentNode{Type: ast.TypeDocument, Children: children}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func splitBlocks(s string) []string {
	return strings.Split(s, "\n\n")
}

func nonEmptyLines(block string) []string {
	var out []string
	for _, l := range strings.Split(block, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func allMatch(lines []string, re *regexp.Regexp) bool {
	for _, l := range lines {
		if !re.MatchString(l) {
			return false
		}
	}
	return true
}

func extractGroup(lines []string, re *regexp.Regexp, group int) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		m := re.FindStringSubmatch(l)
		if m != nil && group < len(m) {
			out[i] = m[group]
		} else {
			out[i] = l
		}
	}
	return out
}
