// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"regexp"
	"strings"

	"annave.tech/pdf-engine/internal/ast"
	"golang.org/x/net/html"
)

var htmlTagRe = regexp.MustCompile(`(?i)<[a-z][\s\S]*>`)

type HtmlParser struct{}

func (p *HtmlParser) CanParse(input string) bool {
	return htmlTagRe.MatchString(input)
}

func (p *HtmlParser) Parse(input string) (*ast.DocumentNode, error) {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return &ast.DocumentNode{Type: ast.TypeDocument}, nil
	}

	var children []ast.Node
	var body *html.Node
	var findBody func(*html.Node)
	findBody = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findBody(c)
		}
	}
	findBody(doc)
	if body == nil {
		body = doc
	}

	walkHTMLBlock(body, &children, false, false)
	return &ast.DocumentNode{Type: ast.TypeDocument, Children: children}, nil
}

var blockTags = map[string]bool{
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"p": true, "ul": true, "ol": true, "table": true, "pre": true,
	"blockquote": true, "hr": true, "img": true,
	"div": true, "section": true, "article": true, "main": true,
}

func walkHTMLBlock(n *html.Node, out *[]ast.Node, inBlockquote, inPre bool) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		tag := strings.ToLower(c.Data)

		if inBlockquote && tag != "blockquote" {
			continue
		}
		if inPre && tag != "pre" {
			continue
		}

		switch {
		case isHeadingTag(tag):
			text := nodeText(c)
			if text == "" {
				continue
			}
			level := int(tag[1] - '0')
			if level > 3 {
				level = 3
			}
			*out = append(*out, ast.Node{
				Type:  ast.TypeHeading,
				Level: level,
				Text:  text,
				Spans: extractHTMLSpans(c),
			})

		case tag == "p":
			text := nodeText(c)
			if text == "" {
				continue
			}
			*out = append(*out, ast.Node{
				Type:  ast.TypeParagraph,
				Text:  text,
				Spans: extractHTMLSpans(c),
			})

		case tag == "ul" || tag == "ol":
			var items []string
			var itemSpans [][]ast.InlineSpan
			for li := c.FirstChild; li != nil; li = li.NextSibling {
				if li.Type == html.ElementNode && li.Data == "li" {
					t := nodeText(li)
					if t != "" {
						items = append(items, t)
						itemSpans = append(itemSpans, extractHTMLSpans(li))
					}
				}
			}
			if len(items) > 0 {
				*out = append(*out, ast.Node{
					Type:      ast.TypeList,
					Ordered:   tag == "ol",
					Items:     items,
					ItemSpans: itemSpans,
				})
			}

		case tag == "table":
			var headers []string
			var rows [][]string
			for _, th := range findAll(c, "th") {
				headers = append(headers, nodeText(th))
			}
			for _, tr := range findAll(c, "tr") {
				if findFirst(tr, "td") == nil {
					continue
				}
				var cells []string
				for _, td := range findAll(tr, "td") {
					cells = append(cells, nodeText(td))
				}
				rows = append(rows, cells)
			}
			if len(headers) > 0 || len(rows) > 0 {
				*out = append(*out, ast.Node{Type: ast.TypeTable, Headers: headers, Rows: rows})
			}

		case tag == "pre":
			codeEl := findFirst(c, "code")
			var codeText string
			if codeEl != nil {
				codeText = strings.TrimRight(nodeText(codeEl), " \n")
			} else {
				codeText = strings.TrimRight(nodeText(c), " \n")
			}
			if strings.TrimSpace(codeText) == "" {
				continue
			}
			lang := ""
			if codeEl != nil {
				for _, a := range codeEl.Attr {
					if a.Key == "class" {
						if m := regexp.MustCompile(`language-(\S+)`).FindStringSubmatch(a.Val); m != nil {
							lang = m[1]
						}
					}
				}
			}
			*out = append(*out, ast.Node{Type: ast.TypeCodeBlock, Language: lang, Text: codeText})

		case tag == "blockquote":
			raw := nodeText(c)
			if raw == "" {
				continue
			}
			*out = append(*out, ast.Node{Type: ast.TypeBlockquote, Spans: extractHTMLSpans(c)})

		case tag == "hr":
			*out = append(*out, ast.Node{Type: ast.TypeHR})

		case tag == "img":
			src := attr(c, "src")
			alt := attr(c, "alt")
			if src != "" {
				*out = append(*out, ast.Node{Type: ast.TypeImage, Alt: alt, Src: src})
			}

		default:
			// div, section, etc — recurse
			if blockTags[tag] || tag == "div" || tag == "section" || tag == "article" {
				walkHTMLBlock(c, out, false, false)
			}
		}
	}
}

func extractHTMLSpans(n *html.Node) []ast.InlineSpan {
	var spans []ast.InlineSpan
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.TextNode:
			t := c.Data
			if t == "" {
				continue
			}
			if len(spans) > 0 && spans[len(spans)-1].Kind == ast.SpanText {
				spans[len(spans)-1].Text += t
			} else {
				spans = append(spans, ast.InlineSpan{Kind: ast.SpanText, Text: t})
			}
		case html.ElementNode:
			tag := strings.ToLower(c.Data)
			text := nodeText(c)
			if text == "" {
				continue
			}
			switch tag {
			case "strong", "b":
				spans = append(spans, ast.InlineSpan{Kind: ast.SpanBold, Text: text})
			case "em", "i":
				spans = append(spans, ast.InlineSpan{Kind: ast.SpanItalic, Text: text})
			case "code":
				spans = append(spans, ast.InlineSpan{Kind: ast.SpanCode, Text: text})
			case "del", "s", "strike":
				spans = append(spans, ast.InlineSpan{Kind: ast.SpanStrike, Text: text})
			case "a":
				spans = append(spans, ast.InlineSpan{Kind: ast.SpanLink, Text: text, Href: attr(c, "href")})
			default:
				spans = append(spans, extractHTMLSpans(c)...)
			}
		}
	}
	if len(spans) == 0 {
		t := strings.TrimSpace(nodeText(n))
		if t != "" {
			spans = append(spans, ast.InlineSpan{Kind: ast.SpanText, Text: t})
		}
	}
	return spans
}

func isHeadingTag(tag string) bool {
	return len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6'
}

func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(sb.String())
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func findFirst(n *html.Node, tag string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			return c
		}
		if found := findFirst(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func findAll(n *html.Node, tag string) []*html.Node {
	var result []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			result = append(result, c)
		} else {
			result = append(result, findAll(c, tag)...)
		}
	}
	return result
}
