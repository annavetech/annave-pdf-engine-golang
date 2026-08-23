// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"github.com/annavetech/annave-pdf-engine-golang/internal/ast"
)

func TestMdParser_CanParse(t *testing.T) {
	p := &MdParser{}

	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"ATX heading", "# Hello", true},
		{"level-two heading", "## Hello", true},
		{"plain text without heading", "just some words", false},
		{"JSON object", `{"type":"document"}`, false},
		{"empty string", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := p.CanParse(tc.input); got != tc.want {
				t.Errorf("CanParse(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestMdParser_Parse_BlockTypes(t *testing.T) {
	input := `# Heading One

A paragraph with **bold** and _italic_ text.

## Heading Two

- item one
- item two

1. ordered one
2. ordered two

| Col A | Col B |
|-------|-------|
| r1a   | r1b   |

` + "```go\nfmt.Println()\n```" + `

> A blockquote.

---

![alt text](https://example.com/img.png)
`
	p := &MdParser{}
	doc, err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if doc.Type != ast.TypeDocument {
		t.Fatalf("doc.Type = %q, want %q", doc.Type, ast.TypeDocument)
	}

	wantTypes := []string{
		ast.TypeHeading,
		ast.TypeParagraph,
		ast.TypeHeading,
		ast.TypeList,
		ast.TypeList,
		ast.TypeTable,
		ast.TypeCodeBlock,
		ast.TypeBlockquote,
		ast.TypeHR,
		ast.TypeImage,
	}

	if len(doc.Children) != len(wantTypes) {
		t.Fatalf("got %d nodes, want %d\nnodes: %v", len(doc.Children), len(wantTypes), nodeTypes(doc.Children))
	}
	for i, want := range wantTypes {
		if doc.Children[i].Type != want {
			t.Errorf("node[%d]: got type %q, want %q", i, doc.Children[i].Type, want)
		}
	}
}

func TestMdParser_Parse_HeadingLevels(t *testing.T) {
	p := &MdParser{}
	doc, _ := p.Parse("# H1\n\n## H2\n\n### H3\n\n#### H4 (capped at 3)")

	if len(doc.Children) != 4 {
		t.Fatalf("expected 4 heading nodes, got %d", len(doc.Children))
	}
	levels := []int{1, 2, 3, 3}
	for i, want := range levels {
		if doc.Children[i].Level != want {
			t.Errorf("node[%d] level = %d, want %d", i, doc.Children[i].Level, want)
		}
	}
}

func TestMdParser_Parse_FencedCodeBlockLanguage(t *testing.T) {
	p := &MdParser{}
	doc, _ := p.Parse("# Title\n\n```python\nprint('hello')\n```")

	var codeNode *ast.Node
	for i := range doc.Children {
		if doc.Children[i].Type == ast.TypeCodeBlock {
			codeNode = &doc.Children[i]
			break
		}
	}
	if codeNode == nil {
		t.Fatal("no code block found")
	}
	if codeNode.Language != "python" {
		t.Errorf("Language = %q, want %q", codeNode.Language, "python")
	}
	if codeNode.Text != "print('hello')" {
		t.Errorf("Text = %q, want %q", codeNode.Text, "print('hello')")
	}
}

func TestMdParser_Parse_HTMLFragmentsInParagraph(t *testing.T) {
	// Markdown with inline HTML references must produce paragraph nodes,
	// not be misidentified as an HTML document.
	p := &MdParser{}
	doc, _ := p.Parse("# Title\n\nThis embeds content via <iframe> or <embed> tags.")

	if len(doc.Children) != 2 {
		t.Fatalf("expected 2 nodes (heading + paragraph), got %d", len(doc.Children))
	}
	if doc.Children[1].Type != ast.TypeParagraph {
		t.Errorf("node[1].Type = %q, want paragraph", doc.Children[1].Type)
	}
}

func TestParseInline_SpanKinds(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantKind string
		wantText string
	}{
		{"bold asterisks", "**bold**", ast.SpanBold, "bold"},
		{"italic underscore", "_italic_", ast.SpanItalic, "italic"},
		{"inline code", "`code`", ast.SpanCode, "code"},
		{"strikethrough", "~~strike~~", ast.SpanStrike, "strike"},
		{"link", "[label](https://example.com)", ast.SpanLink, "label"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spans := ParseInline(tc.input)
			if len(spans) == 0 {
				t.Fatalf("ParseInline(%q) returned no spans", tc.input)
			}
			found := false
			for _, s := range spans {
				if s.Kind == tc.wantKind && s.Text == tc.wantText {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ParseInline(%q) missing span {kind:%q text:%q}\ngot: %v",
					tc.input, tc.wantKind, tc.wantText, spans)
			}
		})
	}
}

// FuzzMdParser verifies the parser does not panic on arbitrary input.
func FuzzMdParser(f *testing.F) {
	seeds := []string{
		"# Hello\n\nWorld",
		"## Section\n\n- item one\n- item two",
		"```go\nfmt.Println()\n```",
		"| A | B |\n|---|---|\n| 1 | 2 |",
		"> blockquote text",
		"---",
		"![alt](src)",
		"",
		"plain text only",
		"**bold** and _italic_ and `code`",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	p := &MdParser{}
	f.Fuzz(func(t *testing.T, s string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("MdParser.Parse panicked on input %q: %v", s, r)
			}
		}()
		_, _ = p.Parse(s)
	})
}

func nodeTypes(nodes []ast.Node) []string {
	types := make([]string, len(nodes))
	for i, n := range nodes {
		types[i] = n.Type
	}
	return types
}
