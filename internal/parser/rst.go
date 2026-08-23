// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"regexp"
	"strings"

	"annave.tech/pdf-engine/internal/ast"
)

var adornmentChars = map[byte]bool{'=': true, '-': true, '~': true, '^': true, '"': true, '\'': true, '#': true, '*': true, '+': true, '_': true}

var (
	rstHR        = regexp.MustCompile(`^-{4,}$`)
	rstDirective = regexp.MustCompile(`^\.\.\s+\S`)
	rstFieldList = regexp.MustCompile(`^:[^:]+:`)
	rstBullet    = regexp.MustCompile(`^[-*+]\s`)
	rstOrdered   = regexp.MustCompile(`^\d+[.)]\s`)

	// rstBulletPrefix and rstOrderedPrefix require one or more spaces after
	// the marker (unlike rstBullet/rstOrdered above, which only detect the
	// marker) because they also strip the prefix via ReplaceAllString.
	rstBulletPrefix  = regexp.MustCompile(`^[-*+]\s+`)
	rstOrderedPrefix = regexp.MustCompile(`^\d+[.)]\s+`)

	rstFieldPattern        = regexp.MustCompile(`^:([^:]+):\s*(.*)`)
	rstCodeDirective       = regexp.MustCompile(`^\.\.\s+code(?:-block)?::\s*(\w*)`)
	rstAdmonitionDirective = regexp.MustCompile(`^\.\.\s+(\w+)::\s*(.*)`)
	rstDoubleBacktick      = regexp.MustCompile("``([^`]+)``")
)

type RstParser struct{}

func (p *RstParser) CanParse(input string) bool {
	for _, l := range strings.Split(input, "\n") {
		t := strings.TrimSpace(l)
		if len(t) >= 4 && adornmentChars[t[0]] && isAllSame(t) {
			return true
		}
	}
	return false
}

func (p *RstParser) Parse(input string) (*ast.DocumentNode, error) {
	lines := strings.Split(input, "\n")
	var children []ast.Node
	levelMap := map[byte]int{}
	nextLevel := func(ch byte) int {
		if _, ok := levelMap[ch]; !ok {
			n := len(levelMap) + 1
			if n > 3 {
				n = 3
			}
			levelMap[ch] = n
		}
		return levelMap[ch]
	}

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			i++
			continue
		}

		// HR: 4+ dashes alone
		if rstHR.MatchString(trimmed) && !isAdornmentForTitle(lines, i) {
			children = append(children, ast.Node{Type: ast.TypeHR})
			i++
			continue
		}

		// Heading detection
		if node, next := detectRSTHeading(lines, i, nextLevel); node != nil {
			children = append(children, *node)
			i = next
			continue
		}

		// Directive .. something::
		if rstDirective.MatchString(trimmed) {
			node, next := parseRSTDirective(lines, i)
			if node != nil {
				children = append(children, *node)
			}
			i = next
			continue
		}

		// Literal block ::
		if trimmed == "::" {
			node, next := parseRSTLiteralBlock(lines, i+1, "")
			if node != nil {
				children = append(children, *node)
			}
			i = next
			continue
		}

		// Field list :key: value
		if rstFieldList.MatchString(trimmed) {
			node, next := parseRSTFieldList(lines, i)
			if node != nil {
				children = append(children, *node)
			}
			i = next
			continue
		}

		// Bullet list
		if rstBullet.MatchString(trimmed) {
			node, next := parseRSTList(lines, i, false)
			children = append(children, node)
			i = next
			continue
		}

		// Ordered list
		if rstOrdered.MatchString(trimmed) {
			node, next := parseRSTList(lines, i, true)
			children = append(children, node)
			i = next
			continue
		}

		// Indented blockquote
		if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "    ..") {
			node, next := parseRSTBlockquote(lines, i)
			if node != nil {
				children = append(children, *node)
			}
			i = next
			continue
		}

		// Paragraph
		node, next := parseRSTPararaph(lines, i)
		if node != nil {
			children = append(children, *node)
		}
		i = next
	}

	return &ast.DocumentNode{Type: ast.TypeDocument, Children: children}, nil
}

func detectRSTHeading(lines []string, i int, nextLevel func(byte) int) (*ast.Node, int) {
	line := strings.TrimSpace(lines[i])
	// Pattern A: overline + title + underline
	if isAdornLine(line) && i+2 < len(lines) {
		title := strings.TrimSpace(lines[i+1])
		under := strings.TrimSpace(lines[i+2])
		if title != "" && isAdornLine(under) && under[0] == line[0] {
			level := nextLevel(line[0])
			n := rstHeading(level, title)
			return &n, i + 3
		}
	}
	// Pattern B: title + underline
	if i+1 < len(lines) {
		under := strings.TrimSpace(lines[i+1])
		if line != "" && !isAdornLine(line) && isAdornLine(under) && len(under) >= len(line)-2 {
			level := nextLevel(under[0])
			n := rstHeading(level, line)
			return &n, i + 2
		}
	}
	return nil, i
}

func parseRSTList(lines []string, start int, ordered bool) (ast.Node, int) {
	prefixRe := rstBulletPrefix
	if ordered {
		prefixRe = rstOrderedPrefix
	}
	var items []string
	i := start
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			i++
			continue
		}
		if !prefixRe.MatchString(t) && !strings.HasPrefix(lines[i], "  ") {
			break
		}
		if prefixRe.MatchString(t) {
			items = append(items, prefixRe.ReplaceAllString(t, ""))
		} else if len(items) > 0 {
			items[len(items)-1] += " " + strings.TrimSpace(t)
		}
		i++
	}
	var itemSpans [][]ast.InlineSpan
	for _, it := range items {
		itemSpans = append(itemSpans, rstInline(it))
	}
	return ast.Node{Type: ast.TypeList, Ordered: ordered, Items: items, ItemSpans: itemSpans}, i
}

func parseRSTFieldList(lines []string, start int) (*ast.Node, int) {
	fieldRe := rstFieldPattern
	var rows [][]string
	i := start
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			i++
			continue
		}
		m := fieldRe.FindStringSubmatch(t)
		if m == nil {
			break
		}
		rows = append(rows, []string{m[1], m[2]})
		i++
	}
	if len(rows) == 0 {
		return nil, i
	}
	n := ast.Node{Type: ast.TypeTable, Headers: []string{"Field", "Value"}, Rows: rows}
	return &n, i
}

func parseRSTLiteralBlock(lines []string, start int, lang string) (*ast.Node, int) {
	i := start
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return nil, i
	}
	firstLine := lines[i]
	indent := len(firstLine) - len(strings.TrimLeft(firstLine, " \t"))
	if indent == 0 {
		return nil, i
	}
	var codeLines []string
	for i < len(lines) {
		l := lines[i]
		if strings.TrimSpace(l) == "" {
			codeLines = append(codeLines, "")
			i++
			continue
		}
		lineIndent := len(l) - len(strings.TrimLeft(l, " \t"))
		if lineIndent < indent {
			break
		}
		codeLines = append(codeLines, l[indent:])
		i++
	}
	for len(codeLines) > 0 && strings.TrimSpace(codeLines[len(codeLines)-1]) == "" {
		codeLines = codeLines[:len(codeLines)-1]
	}
	text := strings.Join(codeLines, "\n")
	if text == "" {
		return nil, i
	}
	n := ast.Node{Type: ast.TypeCodeBlock, Language: lang, Text: text}
	return &n, i
}

func parseRSTDirective(lines []string, start int) (*ast.Node, int) {
	line := strings.TrimSpace(lines[start])
	codeMatch := rstCodeDirective.FindStringSubmatch(line)
	if codeMatch != nil {
		return parseRSTLiteralBlock(lines, start+1, codeMatch[1])
	}
	admonition := rstAdmonitionDirective.FindStringSubmatch(line)
	if admonition != nil {
		label := admonition[1]
		body := strings.TrimSpace(admonition[2])
		i := start + 1
		for i < len(lines) {
			l := lines[i]
			if strings.TrimSpace(l) == "" {
				i++
				break
			}
			if strings.HasPrefix(l, "   ") || strings.HasPrefix(l, "\t") {
				body += " " + strings.TrimSpace(l)
				i++
			} else {
				break
			}
		}
		text := strings.ToUpper(label) + ": " + strings.TrimSpace(body)
		n := ast.Node{Type: ast.TypeBlockquote, Spans: rstInline(text)}
		return &n, i
	}
	i := start + 1
	for i < len(lines) && (strings.HasPrefix(lines[i], "   ") || strings.TrimSpace(lines[i]) == "") {
		i++
	}
	return nil, i
}

func parseRSTBlockquote(lines []string, start int) (*ast.Node, int) {
	var parts []string
	i := start
	for i < len(lines) {
		l := lines[i]
		if strings.TrimSpace(l) == "" {
			i++
			break
		}
		if strings.HasPrefix(l, "    ") {
			parts = append(parts, strings.TrimSpace(l))
			i++
		} else {
			break
		}
	}
	text := strings.Join(parts, " ")
	if text == "" {
		return nil, i
	}
	n := ast.Node{Type: ast.TypeBlockquote, Spans: rstInline(text)}
	return &n, i
}

func parseRSTPararaph(lines []string, start int) (*ast.Node, int) {
	var parts []string
	i := start
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			i++
			break
		}
		if isAdornLine(t) || rstBullet.MatchString(t) ||
			rstOrdered.MatchString(t) ||
			rstFieldList.MatchString(t) ||
			rstDirective.MatchString(t) {
			break
		}
		if i+1 < len(lines) && isAdornLine(strings.TrimSpace(lines[i+1])) {
			break
		}
		if strings.HasSuffix(t, "::") {
			parts = append(parts, strings.TrimSuffix(t, "::"))
			i++
			break
		}
		parts = append(parts, t)
		i++
	}
	text := strings.Join(parts, " ")
	if text == "" {
		return nil, i
	}
	n := ast.Node{Type: ast.TypeParagraph, Text: text, Spans: rstInline(text)}
	return &n, i
}

func rstInline(text string) []ast.InlineSpan {
	converted := rstDoubleBacktick.ReplaceAllString(text, "`$1`")
	return ParseInline(converted)
}

func rstHeading(level int, text string) ast.Node {
	return ast.Node{Type: ast.TypeHeading, Level: level, Text: text, Spans: rstInline(text)}
}

func isAdornLine(s string) bool {
	if len(s) < 4 || !adornmentChars[s[0]] {
		return false
	}
	return isAllSame(s)
}

func isAllSame(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}

func isAdornmentForTitle(lines []string, i int) bool {
	if i > 0 && strings.TrimSpace(lines[i-1]) != "" {
		return true
	}
	if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" {
		return true
	}
	return false
}
