// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/annavetech/pdfengine/internal/engine"
	"github.com/annavetech/pdfengine/internal/parser"
)

func main() {
	path := os.Args[1]
	data, _ := os.ReadFile(path) //nolint:gosec // path is the local dev debug tool's CLI argument, not external input
	input := string(data)

	cfg, err := engine.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load default config: %v\n", err)
		os.Exit(1)
	}

	normalized, _ := engine.NormalizeInput(input, cfg)
	fmt.Printf("Input chars: %d\n", len([]rune(normalized)))
	fmt.Printf("Input lines: %d\n", len(strings.Split(normalized, "\n")))

	reg := parser.NewRegistry(cfg.Limits.Input.MaxInputChars)
	doc, _ := reg.Parse([]byte(normalized), parser.FormatMd)
	fmt.Printf("AST nodes:   %d\n", len(doc.Children))
	for i, n := range doc.Children {
		text := n.Text
		if len(text) > 60 {
			text = text[:60]
		}
		fmt.Printf("  [%02d] %-12s level=%d items=%d text=%q\n", i, n.Type, n.Level, len(n.Items), text)
	}

	layout := engine.NewLayoutEngine()
	boxes := layout.Compute(doc, cfg.Style)
	fmt.Printf("Layout boxes: %d\n", len(boxes))

	pag := engine.NewPaginator()
	pages := pag.Paginate(boxes, cfg.Style.Page)
	fmt.Printf("Pages:        %d\n", len(pages))
}
