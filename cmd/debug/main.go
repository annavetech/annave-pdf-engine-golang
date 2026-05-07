// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"strings"

	"annave.tech/pdf-engine/internal/engine"
	"annave.tech/pdf-engine/internal/parser"
)

func main() {
	path := os.Args[1]
	data, _ := os.ReadFile(path)
	input := string(data)

	normalized, _ := engine.NormalizeInput(input)
	fmt.Printf("Input chars: %d\n", len([]rune(normalized)))
	fmt.Printf("Input lines: %d\n", len(strings.Split(normalized, "\n")))

	reg := parser.NewRegistry()
	doc, _ := reg.Parse(normalized, parser.FormatMd)
	fmt.Printf("AST nodes:   %d\n", len(doc.Children))
	for i, n := range doc.Children {
		text := n.Text
		if len(text) > 60 {
			text = text[:60]
		}
		fmt.Printf("  [%02d] %-12s level=%d items=%d text=%q\n", i, n.Type, n.Level, len(n.Items), text)
	}

	layout := engine.NewLayoutEngine()
	boxes := layout.Compute(doc, engine.DocStyle)
	fmt.Printf("Layout boxes: %d\n", len(boxes))

	pag := engine.NewPaginator()
	pages := pag.Paginate(boxes, engine.DocStyle.Page)
	fmt.Printf("Pages:        %d\n", len(pages))
}
