// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/annavetech/pdfengine/internal/ast"
)

// yamlShape summarises nodes as "type:text" for compact comparison.
func yamlShape(nodes []ast.Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		switch n.Type {
		case ast.TypeList:
			out[i] = "list:" + strings.Join(n.Items, "|")
		case ast.TypeTable:
			rows := make([]string, len(n.Rows))
			for j, r := range n.Rows {
				rows[j] = strings.Join(r, "=")
			}
			out[i] = "table:" + strings.Join(rows, "|")
		default:
			out[i] = n.Type + ":" + n.Text
		}
	}
	return out
}

func parseYAMLShape(t *testing.T, input string) []string {
	t.Helper()
	doc, err := (&YamlParser{maxAliasChars: testMaxInputChars}).Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	return yamlShape(doc.Children)
}

func TestYamlParser_Parse_OutputShape(t *testing.T) {
	input := "title: Report\n" +
		"tags:\n  - one\n  - two\n" +
		"owner:\n  name: Ann\n  role: Editor\n" +
		"notes: |\n  line one\n  line two\n" +
		"---\n" +
		"second: doc\n"
	want := []string{
		"heading:title", "paragraph:Report",
		"heading:tags", "list:one|two",
		"heading:owner", "table:name=Ann|role=Editor",
		"heading:notes", "paragraph:line one\nline two",
		"heading:second", "paragraph:doc",
	}
	if got := parseYAMLShape(t, input); !reflect.DeepEqual(got, want) {
		t.Errorf("shape\ngot:  %q\nwant: %q", got, want)
	}
}

func TestYamlParser_Parse_AnchorAndAlias(t *testing.T) {
	input := "base: &b shared value\ncopy: *b\n"
	want := []string{"heading:base", "paragraph:shared value", "heading:copy", "paragraph:shared value"}
	if got := parseYAMLShape(t, input); !reflect.DeepEqual(got, want) {
		t.Errorf("shape\ngot:  %q\nwant: %q", got, want)
	}
}

func TestYamlParser_Parse_FlowMapping(t *testing.T) {
	input := "person: {name: Ann, city: Tallinn}\n"
	want := []string{"heading:person", "table:name=Ann|city=Tallinn"}
	if got := parseYAMLShape(t, input); !reflect.DeepEqual(got, want) {
		t.Errorf("shape\ngot:  %q\nwant: %q", got, want)
	}
}

func TestYamlParser_Parse_InvalidFallsBackToText(t *testing.T) {
	input := "key: value: other"
	want := []string{"paragraph:" + input}
	if got := parseYAMLShape(t, input); !reflect.DeepEqual(got, want) {
		t.Errorf("shape\ngot:  %q\nwant: %q", got, want)
	}
}

// nestedYAML returns levels nested block mappings with a scalar at the bottom.
func nestedYAML(levels int) []byte {
	var b strings.Builder
	for i := 0; i < levels; i++ {
		b.WriteString(strings.Repeat("  ", i))
		b.WriteString("k:\n")
	}
	b.WriteString(strings.Repeat("  ", levels))
	b.WriteString("v\n")
	return []byte(b.String())
}

func TestYamlParser_Parse_DepthLimit(t *testing.T) {
	if _, err := (&YamlParser{maxAliasChars: testMaxInputChars}).Parse(nestedYAML(maxYAMLDepth)); err != nil {
		t.Fatalf("Parse() error at %d levels: %v", maxYAMLDepth, err)
	}
	_, err := (&YamlParser{maxAliasChars: testMaxInputChars}).Parse(nestedYAML(maxYAMLDepth + 1))
	var depthErr *DepthLimitError
	if !errors.As(err, &depthErr) {
		t.Fatalf("Parse() error = %v, want *DepthLimitError", err)
	}
	if depthErr.Format != FormatYAML || depthErr.Limit != maxYAMLDepth {
		t.Errorf("DepthLimitError = %+v, want Format %q Limit %d", depthErr, FormatYAML, maxYAMLDepth)
	}
}

// yamlAliasBomb has six lists: the first holds nine scalars and each later one holds nine aliases to the list before it,
// so the last list expands to 9^6 = 531,441 scalars.
const yamlAliasBomb = `a: &a ["lol","lol","lol","lol","lol","lol","lol","lol","lol"]
b: &b [*a,*a,*a,*a,*a,*a,*a,*a,*a]
c: &c [*b,*b,*b,*b,*b,*b,*b,*b,*b]
d: &d [*c,*c,*c,*c,*c,*c,*c,*c,*c]
e: &e [*d,*d,*d,*d,*d,*d,*d,*d,*d]
f: &f [*e,*e,*e,*e,*e,*e,*e,*e,*e]
`

const mb = 1 << 20

// allocatedBytes returns the bytes allocated while f runs. Callers must not run in parallel.
func allocatedBytes(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// parseYAMLAllocs parses input with budget and returns the bytes allocated and the error.
func parseYAMLAllocs(budget int, input []byte) (uint64, error) {
	var err error
	n := allocatedBytes(func() {
		_, err = (&YamlParser{maxAliasChars: budget}).Parse(input)
	})
	return n, err
}

func wantAliasLimitError(t *testing.T, err error, limit int) {
	t.Helper()
	var aliasErr *AliasLimitError
	if !errors.As(err, &aliasErr) {
		t.Fatalf("Parse() error = %v, want *AliasLimitError", err)
	}
	if aliasErr.Limit != limit {
		t.Errorf("AliasLimitError.Limit = %d, want %d", aliasErr.Limit, limit)
	}
}

func wantDepthLimitError(t *testing.T, err error) {
	t.Helper()
	var depthErr *DepthLimitError
	if !errors.As(err, &depthErr) {
		t.Fatalf("Parse() error = %v, want *DepthLimitError", err)
	}
	if depthErr.Format != FormatYAML || depthErr.Limit != maxYAMLDepth {
		t.Errorf("DepthLimitError = %+v, want Format %q Limit %d", depthErr, FormatYAML, maxYAMLDepth)
	}
}

// joinRepeat returns n copies of item separated by commas.
func joinRepeat(item string, n int) string {
	return strings.TrimSuffix(strings.Repeat(item+",", n), ",")
}

// distinctKeysYAML returns a flow mapping of n distinct keys, optionally preceded by one alias.
func distinctKeysYAML(n int, alias bool) []byte {
	var b strings.Builder
	if alias {
		b.WriteString("x: &x 1\ny: *x\n")
	}
	b.WriteString("z: {")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatInt(int64(i), 36))
	}
	b.WriteString("}\n")
	return []byte(b.String())
}

// duplicateKeysYAML returns one alias followed by a flow mapping that repeats the key a n times.
func duplicateKeysYAML(n int) []byte {
	return []byte("x: &x 1\ny: *x\nz: {" + joinRepeat("a", n) + "}\n")
}

// plainYAMLList returns one flow sequence of item, padded with spaces to exactly size bytes, with no aliases.
func plainYAMLList(item string, size int) []byte {
	const prefix, suffix = "a: [", "]\n"
	n := (size - len(prefix) - len(suffix) + 1) / (len(item) + 1)
	body := joinRepeat(item, n)
	pad := size - len(prefix) - len(body) - len(suffix)
	return []byte(prefix + body + strings.Repeat(" ", pad) + suffix)
}

func TestYamlParser_Parse_RejectsAliasBomb(t *testing.T) {
	baseline := plainYAMLList(`"lol"`, testMaxInputChars)
	baseAllocs, err := parseYAMLAllocs(testMaxInputChars, baseline)
	if err != nil {
		t.Fatalf("Parse() error for the %d-byte plain file: %v", len(baseline), err)
	}
	bombAllocs, err := parseYAMLAllocs(testMaxInputChars, []byte(yamlAliasBomb))
	wantAliasLimitError(t, err, testMaxInputChars)
	t.Logf("plain file: %d bytes allocated; alias bomb: %d bytes allocated; ratio %.2f",
		baseAllocs, bombAllocs, float64(bombAllocs)/float64(baseAllocs))
	// Aliases add at most testMaxInputChars, where each value reached through an alias counts as its length in characters plus one,
	// which is about one plain file's worth; the test allows 3x the plain file to leave room for allocator noise.
	if bombAllocs > 3*baseAllocs {
		t.Errorf("alias bomb allocated %d bytes, want at most 3x the plain file's %d", bombAllocs, baseAllocs)
	}
}

func TestYamlParser_Parse_AliasBudgetBoundary(t *testing.T) {
	const limit = 10
	p := &YamlParser{maxAliasChars: limit}
	if _, err := p.Parse([]byte("a: &a abcd\nb: *a\nc: *a")); err != nil {
		t.Fatalf("Parse() error at an alias cost of exactly %d: %v", limit, err)
	}
	_, err := p.Parse([]byte("a: &a abcd\nb: *a\nc: *a\nd: *a"))
	wantAliasLimitError(t, err, limit)
}

func TestYamlParser_Parse_AliasBudgetSharedAcrossDocuments(t *testing.T) {
	const limit = 1000
	p := &YamlParser{maxAliasChars: limit}
	doc1 := "a: &a [" + joinRepeat("x", 300) + "]\nb: *a\n"
	if _, err := p.Parse([]byte(doc1)); err != nil {
		t.Fatalf("Parse() error for one document: %v", err)
	}
	_, err := p.Parse([]byte(doc1 + "---\nc: *a\n"))
	wantAliasLimitError(t, err, limit)
}

// s1ReviewerInput builds a file whose first document defines nested anchors and whose later documents each alias the largest one.
func s1ReviewerInput(docs int) []byte {
	var b strings.Builder
	b.WriteString("- [" + joinRepeat("x", 9000) + "]\n")
	b.WriteString("- &a [" + joinRepeat("x", 100) + "]\n")
	b.WriteString("- &b [" + joinRepeat("*a", 100) + "]\n")
	b.WriteString("- &c [" + joinRepeat("*b", 20) + "]\n")
	for i := 1; i < docs; i++ {
		b.WriteString("---\n- [" + joinRepeat("x", 4500) + "]\n- *c\n")
	}
	return []byte(b.String())
}

func TestYamlParser_Parse_S1ReviewerInput(t *testing.T) {
	for _, docs := range []int{1, 20, 55} {
		t.Run(fmt.Sprintf("docs=%d", docs), func(t *testing.T) {
			input := s1ReviewerInput(docs)
			allocs, err := parseYAMLAllocs(testMaxInputChars, input)
			t.Logf("%d documents, %d input bytes: %d bytes allocated", docs, len(input), allocs)
			if docs == 1 {
				if err != nil {
					t.Fatalf("Parse() error for one document: %v", err)
				}
			} else {
				wantAliasLimitError(t, err, testMaxInputChars)
			}
			if allocs > 256*mb {
				t.Errorf("allocated %d bytes, want at most %d", allocs, 256*mb)
			}
		})
	}
}

func TestYamlParser_Parse_EmptyCollectionBomb(t *testing.T) {
	var b strings.Builder
	b.WriteString("a: &a [" + joinRepeat("[]", 9) + "]\n")
	for c := 'b'; c <= 'i'; c++ {
		fmt.Fprintf(&b, "%c: &%c [%s]\n", c, c, joinRepeat("*"+string(c-1), 9))
	}
	_, err := (&YamlParser{maxAliasChars: testMaxInputChars}).Parse([]byte(b.String()))
	wantAliasLimitError(t, err, testMaxInputChars)
}

func TestYamlParser_Parse_LargeScalarAliasBomb(t *testing.T) {
	anchor := "a: &a " + strings.Repeat("y", 100_000) + "\n"
	var table strings.Builder
	table.WriteString(anchor + "m:\n  n:\n")
	for i := 1; i <= 6; i++ {
		fmt.Fprintf(&table, "    k%d: *a\n", i)
	}
	cases := map[string]string{
		"sequence":    anchor + "b: [" + joinRepeat("*a", 10) + "]\n",
		"table cells": table.String(),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := (&YamlParser{maxAliasChars: testMaxInputChars}).Parse([]byte(input))
			wantAliasLimitError(t, err, testMaxInputChars)
		})
	}
}

func TestYamlParser_Parse_AliasCycle(t *testing.T) {
	for _, input := range []string{"a: &a [*a]", "a: &a {k: *a}"} {
		t.Run(input, func(t *testing.T) {
			_, err := (&YamlParser{maxAliasChars: testMaxInputChars}).Parse([]byte(input))
			wantDepthLimitError(t, err)
		})
	}
}

// minParseTime returns the fastest of three parses of input.
func minParseTime(t *testing.T, input []byte) time.Duration {
	t.Helper()
	best := time.Duration(-1)
	for i := 0; i < 3; i++ {
		start := time.Now()
		if _, err := (&YamlParser{maxAliasChars: testMaxInputChars}).Parse(input); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		if d := time.Since(start); best < 0 || d < best {
			best = d
		}
	}
	return best
}

func TestYamlParser_Parse_B1DistinctKeysCPU(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	input := distinctKeysYAML(110_000, true)
	withAlias := minParseTime(t, input)
	without := minParseTime(t, distinctKeysYAML(110_000, false))
	ratio := float64(withAlias) / float64(without)
	t.Logf("110000 keys, %d input bytes: %v with alias, %v without, ratio %.2f", len(input), withAlias, without, ratio)
	if ratio > 3 {
		t.Errorf("time with alias / time without = %.2f, want at most 3", ratio)
	}
}

func TestYamlParser_Parse_DuplicateKeysFallBack(t *testing.T) {
	cases := map[string]string{
		"no alias":         "a: 1\na: 2",
		"with alias":       "a: &x 1\na: *x",
		"quoted and plain": "\"a\": 1\na: 2",
		"nested":           "top:\n  b: 1\n  b: 2",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			want := []string{"paragraph:" + input}
			if got := parseYAMLShape(t, input); !reflect.DeepEqual(got, want) {
				t.Errorf("shape\ngot:  %q\nwant: %q", got, want)
			}
		})
	}
}

func TestYamlParser_Parse_DistinctTaggedKeysRender(t *testing.T) {
	want := []string{"heading:1", "paragraph:x", "heading:1", "paragraph:y"}
	if got := parseYAMLShape(t, "1: x\n\"1\": y"); !reflect.DeepEqual(got, want) {
		t.Errorf("shape\ngot:  %q\nwant: %q", got, want)
	}
}

func TestYamlParser_Parse_AliasKeyRenders(t *testing.T) {
	want := []string{"heading:k", "paragraph:name", "heading:name", "paragraph:v"}
	if got := parseYAMLShape(t, "k: &k name\n*k : v"); !reflect.DeepEqual(got, want) {
		t.Errorf("shape\ngot:  %q\nwant: %q", got, want)
	}
}

func TestYamlParser_Parse_B1ReviewerInput(t *testing.T) {
	input := duplicateKeysYAML(4000)
	var doc *ast.DocumentNode
	var err error
	allocs := allocatedBytes(func() {
		doc, err = (&YamlParser{maxAliasChars: testMaxInputChars}).Parse(input)
	})
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	t.Logf("4000 duplicate keys, %d input bytes: %d bytes allocated", len(input), allocs)
	want := []string{"paragraph:" + strings.TrimSpace(string(input))}
	if got := yamlShape(doc.Children); !reflect.DeepEqual(got, want) {
		t.Errorf("want the plain-text fallback, got %d nodes", len(got))
	}
	if allocs > 64*mb {
		t.Errorf("allocated %d bytes, want at most %d", allocs, 64*mb)
	}
}

func TestYamlParser_Parse_DuplicateKeyCheckIsLinear(t *testing.T) {
	smallAllocs, small := parseYAMLAllocs(testMaxInputChars, duplicateKeysYAML(4000))
	largeAllocs, large := parseYAMLAllocs(testMaxInputChars, duplicateKeysYAML(64000))
	if small != nil || large != nil {
		t.Fatalf("Parse() errors: %v, %v", small, large)
	}
	ratio := float64(largeAllocs) / float64(smallAllocs)
	t.Logf("allocated %d bytes for 4000 keys, %d for 64000, ratio %.2f", smallAllocs, largeAllocs, ratio)
	if ratio > 32 {
		t.Errorf("allocation ratio = %.2f, want at most 32", ratio)
	}
}

func TestYamlParser_Parse_AliasBombInLaterDocument(t *testing.T) {
	_, err := (&YamlParser{maxAliasChars: testMaxInputChars}).Parse([]byte("x: 1\n---\n" + yamlAliasBomb))
	wantAliasLimitError(t, err, testMaxInputChars)
}

func TestYamlParser_Parse_AliasAcrossDocuments(t *testing.T) {
	want := []string{"heading:a", "paragraph:1", "heading:b", "paragraph:1"}
	if got := parseYAMLShape(t, "a: &x 1\n---\nb: *x"); !reflect.DeepEqual(got, want) {
		t.Errorf("shape\ngot:  %q\nwant: %q", got, want)
	}
}

func TestYamlParser_Parse_DepthLimitAllCollections(t *testing.T) {
	shapes := map[string]func(levels int) string{
		"flow sequences": func(levels int) string {
			return strings.Repeat("[", levels) + "x" + strings.Repeat("]", levels)
		},
		"block sequences": func(levels int) string {
			return strings.Repeat("- ", levels) + "x"
		},
		"mapping and sequence": func(levels int) string {
			var b strings.Builder
			for i := 0; i < levels; i++ {
				if i%2 == 0 {
					b.WriteString("{k: ")
				} else {
					b.WriteString("[")
				}
			}
			b.WriteString("x")
			for i := levels - 1; i >= 0; i-- {
				if i%2 == 0 {
					b.WriteString("}")
				} else {
					b.WriteString("]")
				}
			}
			return b.String()
		},
	}
	for name, build := range shapes {
		t.Run(name, func(t *testing.T) {
			p := &YamlParser{maxAliasChars: testMaxInputChars}
			if _, err := p.Parse([]byte(build(maxYAMLDepth))); err != nil {
				t.Fatalf("Parse() error at %d levels: %v", maxYAMLDepth, err)
			}
			_, err := p.Parse([]byte(build(maxYAMLDepth + 1)))
			wantDepthLimitError(t, err)
		})
	}
}

func TestYamlParser_Parse_NonScalarKeyFallsBack(t *testing.T) {
	for _, input := range []string{
		"? [a, b]\n: v",
		"? {x: 1}\n: v",
		"k: &k [a]\n? *k\n: v",
		"a: &a {*a: 1}",
	} {
		t.Run(input, func(t *testing.T) {
			want := []string{"paragraph:" + input}
			if got := parseYAMLShape(t, input); !reflect.DeepEqual(got, want) {
				t.Errorf("shape\ngot:  %q\nwant: %q", got, want)
			}
		})
	}
}

// yamlNsPerByte benchmarks parsing input and returns nanoseconds per input byte.
func yamlNsPerByte(t *testing.T, input []byte) float64 {
	t.Helper()
	p := &YamlParser{maxAliasChars: testMaxInputChars}
	parse := func() error {
		_, err := p.Parse(input)
		var depthErr *DepthLimitError
		if err != nil && !errors.As(err, &depthErr) {
			return err
		}
		return nil
	}
	if err := parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	res := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := parse(); err != nil {
				b.Fatalf("Parse() error: %v", err)
			}
		}
	})
	return float64(res.T.Nanoseconds()) / float64(res.N) / float64(len(input))
}

func TestYamlParser_Parse_ScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	cases := []struct {
		name         string
		small, large []byte
	}{
		{"nesting", nestedYAML(250), nestedYAML(2000)},
		{"width with alias", distinctKeysYAML(2500, true), distinctKeysYAML(160_000, true)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			small := yamlNsPerByte(t, tc.small)
			large := yamlNsPerByte(t, tc.large)
			ratio := large / small
			t.Logf("%d bytes: %.2f ns/byte, %d bytes: %.2f ns/byte, ratio %.2f", len(tc.small), small, len(tc.large), large, ratio)
			if ratio > 8 {
				t.Errorf("per-byte cost ratio = %.2f, want at most 8", ratio)
			}
		})
	}
}

func BenchmarkYamlParser_Nesting(b *testing.B) {
	for _, levels := range []int{250, 500, 1000, 2000, 4000} {
		input := nestedYAML(levels)
		b.Run(fmt.Sprintf("levels=%d", levels), func(b *testing.B) {
			b.SetBytes(int64(len(input)))
			for i := 0; i < b.N; i++ {
				_, err := (&YamlParser{maxAliasChars: testMaxInputChars}).Parse(input)
				var depthErr *DepthLimitError
				if err != nil && !errors.As(err, &depthErr) {
					b.Fatalf("Parse() error: %v", err)
				}
			}
		})
	}
}
