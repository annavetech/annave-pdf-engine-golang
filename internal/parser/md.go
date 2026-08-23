// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"regexp"
	"strings"

	"annave.tech/pdf-engine/internal/ast"
)

var (
	mdUnorderedItem = regexp.MustCompile(`^[-*+]\s+(.+)$`)
	mdOrderedItem   = regexp.MustCompile(`^\d+[.)]\s+(.+)$`)
	mdTableRow      = regexp.MustCompile(`^\|.+\|$`)
	mdTableSep      = regexp.MustCompile(`^\|[-: |]+\|$`)
	mdATXHeading    = regexp.MustCompile(`^(#{1,6})\s+(.+?)(?:\s+#+\s*)?$`)
	// mdFence is an interpreted string, not a backtick raw literal, because
	// the pattern itself must match a backtick fence character.
	mdFence         = regexp.MustCompile("^(`{3,}|~{3,})\\s*(\\S*)\\s*$")
	mdThematicBreak = regexp.MustCompile(`^(\s*[-*_]){3,}\s*$`)
	mdImage         = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)]+)\)\s*$`)
	mdHasHeading    = regexp.MustCompile(`(?m)^#{1,}\s`)
	mdQuotePrefix   = regexp.MustCompile(`^>\s?`)

	// ParseInline delimiter patterns.
	boldItalicRe  = regexp.MustCompile(`^(\*{3})([\s\S]*?)\*{3}`)
	boldItalicRe2 = regexp.MustCompile(`^(_{3})([\s\S]*?)_{3}`)
	boldRe        = regexp.MustCompile(`^(\*{2})([\s\S]*?)\*{2}`)
	boldRe2       = regexp.MustCompile(`^(_{2})([\s\S]*?)_{2}`)
	italicRe      = regexp.MustCompile(`^\*([\s\S]*?)\*`)
	italicRe2     = regexp.MustCompile(`^_([\s\S]*?)_`)
	codeRe        = regexp.MustCompile("^`([^`]+)`")
	strikeRe      = regexp.MustCompile(`^~~([\s\S]*?)~~`)
	linkRe        = regexp.MustCompile(`^\[([^\]]+)\]\(([^)]+)\)`)

	// stripInline delimiter patterns.
	stripBoldItalic      = regexp.MustCompile(`\*{3}([^*]+)\*{3}`)
	stripBold            = regexp.MustCompile(`\*{2}([^*]+)\*{2}`)
	stripItalic          = regexp.MustCompile(`\*([^*]+)\*`)
	stripUnderBoldItalic = regexp.MustCompile(`_{3}([^_]+)_{3}`)
	stripUnderBold       = regexp.MustCompile(`_{2}([^_]+)_{2}`)
	stripUnder           = regexp.MustCompile(`_([^_]+)_`)
	stripCode            = regexp.MustCompile("`([^`]+)`")
	stripStrike          = regexp.MustCompile(`~~([^~]+)~~`)
	stripLink            = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
)

type MdParser struct{}

func (p *MdParser) CanParse(input string) bool {
	return mdHasHeading.MatchString(input)
}

func (p *MdParser) Parse(input string) (*ast.DocumentNode, error) {
	lines := strings.Split(input, "\n")
	var children []ast.Node
	i := 0

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			i++
			continue
		}

		// Fenced code block
		if m := mdFence.FindStringSubmatch(line); m != nil {
			fenceChar := string(m[1][0])
			fenceLen := len(m[1])
			lang := m[2]
			i++
			var codeLines []string
			closingPrefix := strings.Repeat(fenceChar, fenceLen)
			for i < len(lines) {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), closingPrefix) {
					// check it's a closing fence
					cl := strings.TrimSpace(lines[i])
					allFence := true
					for _, r := range cl {
						if string(r) != fenceChar {
							allFence = false
							break
						}
					}
					if allFence && len(cl) >= fenceLen {
						i++
						break
					}
				}
				codeLines = append(codeLines, lines[i])
				i++
			}
			codeText := strings.Join(codeLines, "\n")
			if strings.TrimSpace(codeText) != "" {
				children = append(children, ast.Node{Type: ast.TypeCodeBlock, Language: lang, Text: codeText})
			}
			continue
		}

		// Blockquote
		if strings.HasPrefix(trimmed, ">") {
			var quoteLines []string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">") {
				stripped := mdQuotePrefix.ReplaceAllString(strings.TrimSpace(lines[i]), "")
				quoteLines = append(quoteLines, stripped)
				i++
			}
			raw := strings.Join(quoteLines, " ")
			if raw != "" {
				children = append(children, ast.Node{
					Type:  ast.TypeBlockquote,
					Spans: ParseInline(raw),
				})
			}
			continue
		}

		// Thematic break
		if mdThematicBreak.MatchString(trimmed) && !mdUnorderedItem.MatchString(trimmed) {
			children = append(children, ast.Node{Type: ast.TypeHR})
			i++
			continue
		}

		// ATX heading
		if m := mdATXHeading.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			if level > 3 {
				level = 3
			}
			raw := strings.TrimSpace(m[2])
			children = append(children, ast.Node{
				Type:  ast.TypeHeading,
				Level: level,
				Text:  stripInline(raw),
				Spans: ParseInline(raw),
			})
			i++
			continue
		}

		// Standalone image
		if m := mdImage.FindStringSubmatch(trimmed); m != nil {
			children = append(children, ast.Node{Type: ast.TypeImage, Alt: m[1], Src: m[2]})
			i++
			continue
		}

		// GFM table
		if mdTableRow.MatchString(line) && i+1 < len(lines) && mdTableSep.MatchString(strings.TrimSpace(lines[i+1])) {
			tableLines := []string{line, lines[i+1]}
			i += 2
			for i < len(lines) && mdTableRow.MatchString(strings.TrimSpace(lines[i])) {
				tableLines = append(tableLines, lines[i])
				i++
			}
			if node := parseGFMTable(tableLines); node != nil {
				children = append(children, *node)
			}
			continue
		}

		// Unordered list
		if mdUnorderedItem.MatchString(trimmed) {
			var items []string
			var itemSpans [][]ast.InlineSpan
			for i < len(lines) && mdUnorderedItem.MatchString(strings.TrimSpace(lines[i])) {
				m := mdUnorderedItem.FindStringSubmatch(strings.TrimSpace(lines[i]))
				raw := m[1]
				items = append(items, stripInline(raw))
				itemSpans = append(itemSpans, ParseInline(raw))
				i++
			}
			children = append(children, ast.Node{Type: ast.TypeList, Ordered: false, Items: items, ItemSpans: itemSpans})
			continue
		}

		// Ordered list
		if mdOrderedItem.MatchString(trimmed) {
			var items []string
			var itemSpans [][]ast.InlineSpan
			for i < len(lines) && mdOrderedItem.MatchString(strings.TrimSpace(lines[i])) {
				m := mdOrderedItem.FindStringSubmatch(strings.TrimSpace(lines[i]))
				raw := m[1]
				items = append(items, stripInline(raw))
				itemSpans = append(itemSpans, ParseInline(raw))
				i++
			}
			children = append(children, ast.Node{Type: ast.TypeList, Ordered: true, Items: items, ItemSpans: itemSpans})
			continue
		}

		// Paragraph
		var paraLines []string
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" && !isMdSpecialLine(lines[i]) {
			paraLines = append(paraLines, lines[i])
			i++
		}
		if len(paraLines) > 0 {
			raw := strings.Join(paraLines, " ")
			text := stripInline(raw)
			if text != "" {
				children = append(children, ast.Node{
					Type:  ast.TypeParagraph,
					Text:  text,
					Spans: ParseInline(raw),
				})
			}
		}
	}

	return &ast.DocumentNode{Type: ast.TypeDocument, Children: children}, nil
}

func parseGFMTable(lines []string) *ast.Node {
	parseRow := func(line string) []string {
		s := strings.TrimPrefix(line, "|")
		s = strings.TrimSuffix(s, "|")
		parts := strings.Split(s, "|")
		for i, p := range parts {
			parts[i] = stripInline(strings.TrimSpace(p))
		}
		return parts
	}
	headers := parseRow(lines[0])
	if len(headers) == 0 {
		return nil
	}
	var rows [][]string
	for _, l := range lines[2:] {
		rows = append(rows, parseRow(l))
	}
	n := ast.Node{Type: ast.TypeTable, Headers: headers, Rows: rows}
	return &n
}

func isMdSpecialLine(line string) bool {
	t := strings.TrimSpace(line)
	return mdATXHeading.MatchString(line) ||
		mdFence.MatchString(line) ||
		strings.HasPrefix(t, ">") ||
		(mdThematicBreak.MatchString(t) && !mdUnorderedItem.MatchString(t)) ||
		mdTableRow.MatchString(t) ||
		mdUnorderedItem.MatchString(t) ||
		mdOrderedItem.MatchString(t)
}

// ParseInline converts raw markdown inline text to InlineSpan[].
func ParseInline(raw string) []ast.InlineSpan {
	var spans []ast.InlineSpan
	s := raw

	appendText := func(t string) {
		if len(spans) > 0 && spans[len(spans)-1].Kind == ast.SpanText {
			spans[len(spans)-1].Text += t
		} else {
			spans = append(spans, ast.InlineSpan{Kind: ast.SpanText, Text: t})
		}
	}

	for len(s) > 0 {
		if m := boldItalicRe.FindStringSubmatch(s); m != nil {
			spans = append(spans, ast.InlineSpan{Kind: ast.SpanBoldItalic, Text: m[2]})
			s = s[len(m[0]):]
			continue
		}
		if m := boldItalicRe2.FindStringSubmatch(s); m != nil {
			spans = append(spans, ast.InlineSpan{Kind: ast.SpanBoldItalic, Text: m[2]})
			s = s[len(m[0]):]
			continue
		}
		if m := boldRe.FindStringSubmatch(s); m != nil {
			spans = append(spans, ast.InlineSpan{Kind: ast.SpanBold, Text: m[2]})
			s = s[len(m[0]):]
			continue
		}
		if m := boldRe2.FindStringSubmatch(s); m != nil {
			spans = append(spans, ast.InlineSpan{Kind: ast.SpanBold, Text: m[2]})
			s = s[len(m[0]):]
			continue
		}
		if m := italicRe.FindStringSubmatch(s); m != nil {
			spans = append(spans, ast.InlineSpan{Kind: ast.SpanItalic, Text: m[1]})
			s = s[len(m[0]):]
			continue
		}
		if m := italicRe2.FindStringSubmatch(s); m != nil {
			spans = append(spans, ast.InlineSpan{Kind: ast.SpanItalic, Text: m[1]})
			s = s[len(m[0]):]
			continue
		}
		if m := codeRe.FindStringSubmatch(s); m != nil {
			spans = append(spans, ast.InlineSpan{Kind: ast.SpanCode, Text: m[1]})
			s = s[len(m[0]):]
			continue
		}
		if m := strikeRe.FindStringSubmatch(s); m != nil {
			spans = append(spans, ast.InlineSpan{Kind: ast.SpanStrike, Text: m[1]})
			s = s[len(m[0]):]
			continue
		}
		if m := linkRe.FindStringSubmatch(s); m != nil {
			spans = append(spans, ast.InlineSpan{Kind: ast.SpanLink, Text: m[1], Href: m[2]})
			s = s[len(m[0]):]
			continue
		}
		// Plain text — advance to next marker
		nextIdx := strings.IndexAny(s, "*_`~[")
		if nextIdx == 0 {
			appendText(string(s[0]))
			s = s[1:]
		} else if nextIdx < 0 {
			appendText(s)
			s = ""
		} else {
			appendText(s[:nextIdx])
			s = s[nextIdx:]
		}
	}

	if len(spans) == 0 {
		return []ast.InlineSpan{{Kind: ast.SpanText, Text: raw}}
	}
	return spans
}

func stripInline(s string) string {
	res := s
	res = stripBoldItalic.ReplaceAllString(res, "$1")
	res = stripBold.ReplaceAllString(res, "$1")
	res = stripItalic.ReplaceAllString(res, "$1")
	res = stripUnderBoldItalic.ReplaceAllString(res, "$1")
	res = stripUnderBold.ReplaceAllString(res, "$1")
	res = stripUnder.ReplaceAllString(res, "$1")
	res = stripCode.ReplaceAllString(res, "$1")
	res = stripStrike.ReplaceAllString(res, "$1")
	res = stripLink.ReplaceAllString(res, "$1")
	return res
}
