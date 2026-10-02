// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/annavetech/pdfengine/internal/ast"
	"gopkg.in/yaml.v3"
)

var (
	yamlKeyValue = regexp.MustCompile(`^[a-zA-Z0-9_"'][^:]*\s*:`)
	yamlTopList  = regexp.MustCompile(`^- `)
)

// maxYAMLDepth caps mapping and sequence nesting; a document's top-level collection is level 1.
const maxYAMLDepth = 300

type YamlParser struct {
	maxAliasChars int // alias budget, where each value reached through an alias counts as its length in characters plus one; 0 allows none
}

// AliasLimitError reports that aliases expand past Limit, where each value reached through an alias counts as its length in characters plus one.
type AliasLimitError struct{ Limit int }

func (e *AliasLimitError) Error() string {
	return fmt.Sprintf("yaml: aliases expand past the limit of %d", e.Limit)
}

func (p *YamlParser) CanParse(input []byte) bool {
	t := bytes.TrimSpace(input)
	return bytes.HasPrefix(t, []byte("---")) ||
		yamlKeyValue.Match(t) ||
		yamlTopList.Match(t)
}

func (p *YamlParser) Parse(input []byte) (*ast.DocumentNode, error) {
	docs, err := decodeYAMLDocuments(input)
	if err != nil {
		return fallbackDoc(input), nil
	}
	if err := checkYAMLKeys(docs); err != nil {
		return fallbackDoc(input), nil
	}
	// One walker per file, so the alias budget is shared by every document.
	w := &yamlWalker{budget: p.maxAliasChars, limit: p.maxAliasChars}
	for _, doc := range docs {
		for _, content := range doc.Content {
			n, expanded, err := w.resolve(content, false)
			if err != nil {
				return nil, err
			}
			if err := w.walk(n, 1, expanded); err != nil {
				return nil, err
			}
		}
	}
	return &ast.DocumentNode{Type: ast.TypeDocument, Children: w.out}, nil
}

// decodeYAMLDocuments parses every document in input into yaml.v3 node trees.
func decodeYAMLDocuments(input []byte) ([]*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(input))
	var docs []*yaml.Node
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			return docs, nil
		}
		if err != nil {
			return nil, err
		}
		docs = append(docs, &doc)
	}
}

// yamlKey identifies a mapping key; YAML keys are equal when tag and value are equal.
type yamlKey struct{ tag, value string }

// checkYAMLKeys rejects a mapping with a duplicate key or with a list or map as a key.
// It visits each source node once and never follows aliases, so cycles cannot loop it.
func checkYAMLKeys(docs []*yaml.Node) error {
	stack := append([]*yaml.Node(nil), docs...)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n.Kind == yaml.MappingNode {
			// A fresh map per mapping; clearing one large map for many small mappings is quadratic.
			seen := make(map[yamlKey]struct{}, len(n.Content)/2)
			for i := 0; i+1 < len(n.Content); i += 2 {
				k := n.Content[i]
				if k.Kind == yaml.AliasNode && k.Alias != nil {
					k = k.Alias
				}
				if k.Kind != yaml.ScalarNode {
					return errors.New("yaml: mapping key is not a scalar")
				}
				key := yamlKey{tag: k.ShortTag(), value: k.Value}
				if _, dup := seen[key]; dup {
					return errors.New("yaml: duplicate mapping key")
				}
				seen[key] = struct{}{}
			}
		}
		stack = append(stack, n.Content...)
	}
	return nil
}

// yamlWalker builds the AST for a YAML file and charges content read through aliases against budget.
type yamlWalker struct {
	out    []ast.Node
	budget int // alias budget left, where each value reached through an alias counts as its length in characters plus one
	limit  int
}

// yamlRef is a resolved node and whether it was reached through an alias.
type yamlRef struct {
	n        *yaml.Node
	expanded bool
}

// resolve follows n if it is an alias and charges the result when it was reached through an alias.
func (w *yamlWalker) resolve(n *yaml.Node, expanded bool) (*yaml.Node, bool, error) {
	if n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
		expanded = true
	}
	if expanded {
		// The extra 1 charges empty scalars and collections, so empty-collection bombs are bounded too.
		w.budget -= utf8.RuneCountInString(n.Value) + 1
		if w.budget < 0 {
			return nil, false, &AliasLimitError{Limit: w.limit}
		}
	}
	return n, expanded, nil
}

// walk appends the AST for the resolved node n; level is n's nesting level if n is a collection.
func (w *yamlWalker) walk(n *yaml.Node, level int, expanded bool) error {
	switch n.Kind {
	case yaml.MappingNode:
		if level > maxYAMLDepth {
			return &DepthLimitError{Format: FormatYAML, Limit: maxYAMLDepth}
		}
		pairs := make([]yamlRef, len(n.Content))
		for i, c := range n.Content {
			r, e, err := w.resolve(c, expanded)
			if err != nil {
				return err
			}
			pairs[i] = yamlRef{n: r, expanded: e}
		}
		if level > 1 {
			if tbl := yamlTable(pairs); tbl != nil {
				w.out = append(w.out, *tbl)
				return nil
			}
		}
		// checkYAMLKeys has already ensured every key is a scalar.
		for i := 0; i+1 < len(pairs); i += 2 {
			key := pairs[i].n.Value
			w.out = append(w.out, ast.Node{Type: ast.TypeHeading, Level: 2, Text: key, Spans: []ast.InlineSpan{{Kind: ast.SpanText, Text: key}}})
			if err := w.walk(pairs[i+1].n, level+1, pairs[i+1].expanded); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		if level > maxYAMLDepth {
			return &DepthLimitError{Format: FormatYAML, Limit: maxYAMLDepth}
		}
		var items []string
		for _, c := range n.Content {
			item, e, err := w.resolve(c, expanded)
			if err != nil {
				return err
			}
			if item.Kind == yaml.ScalarNode {
				items = append(items, item.Value)
				continue
			}
			appendYAMLList(items, &w.out)
			items = nil
			if err := w.walk(item, level+1, e); err != nil {
				return err
			}
		}
		appendYAMLList(items, &w.out)
	case yaml.ScalarNode:
		if text := strings.TrimSpace(n.Value); text != "" {
			w.out = append(w.out, ast.Node{Type: ast.TypeParagraph, Text: text, Spans: ParseInline(text)})
		}
	}
	return nil
}

// yamlTable returns a Key/Value table when every key and value in pairs is a scalar, else nil.
func yamlTable(pairs []yamlRef) *ast.Node {
	if len(pairs) == 0 {
		return nil
	}
	rows := make([][]string, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		k, v := pairs[i].n, pairs[i+1].n
		if k.Kind != yaml.ScalarNode || v.Kind != yaml.ScalarNode {
			return nil
		}
		rows = append(rows, []string{k.Value, strings.TrimSpace(v.Value)})
	}
	return &ast.Node{Type: ast.TypeTable, Headers: []string{"Key", "Value"}, Rows: rows}
}

func appendYAMLList(items []string, out *[]ast.Node) {
	if len(items) == 0 {
		return
	}
	spans := make([][]ast.InlineSpan, len(items))
	for i, it := range items {
		spans[i] = ParseInline(it)
	}
	*out = append(*out, ast.Node{Type: ast.TypeList, Ordered: false, Items: items, ItemSpans: spans})
}
