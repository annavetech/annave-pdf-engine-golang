// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"annave.tech/pdf-engine/internal/ast"
)

// DocxParser parses Microsoft Word (.docx) files.
type DocxParser struct{}

func (p *DocxParser) CanParse(input string) bool {
	b := []byte(input)
	if len(b) < 4 || b[0] != 0x50 || b[1] != 0x4b || b[2] != 0x03 || b[3] != 0x04 {
		return false
	}
	return docxHasWordDoc(b)
}

func (p *DocxParser) Parse(input string) (*ast.DocumentNode, error) {
	data := []byte(input)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("docx: invalid archive: %w", err)
	}

	orderedNums := map[string]bool{}
	if f := docxZipEntry(zr, "word/numbering.xml"); f != nil {
		orderedNums = docxParseNumbering(f)
	}

	// Build relationship map: rId → image path inside the ZIP.
	rels := docxParseRels(zr)

	docFile := docxZipEntry(zr, "word/document.xml")
	if docFile == nil {
		return nil, fmt.Errorf("docx: word/document.xml not found")
	}
	rc, err := docFile.Open()
	if err != nil {
		return nil, fmt.Errorf("docx: open document.xml: %w", err)
	}
	defer func() { _ = rc.Close() }()

	xmlData, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("docx: read document.xml: %w", err)
	}

	blocks, err := docxParseDocument(xmlData)
	if err != nil {
		return nil, err
	}

	return &ast.DocumentNode{
		Type:     ast.TypeDocument,
		Children: docxBlocksToAST(blocks, orderedNums, rels, zr),
	}, nil
}

// ── intermediate types ────────────────────────────────────────────────────────

type docxParaBlock struct {
	style    string // normalised pStyle value
	numID    string // numbering ID; non-empty = list item
	ilvl     int    // list indent level (0 = top)
	imageRID string // relationship ID of embedded drawing/image
	runs     []docxRun
}

type docxTableBlock struct {
	rows [][]string // [row][col] text
}

type docxRun struct {
	text   string
	bold   bool
	italic bool
	strike bool
	code   bool
}

// ── zip helpers ───────────────────────────────────────────────────────────────

func docxParseRels(zr *zip.Reader) map[string]string {
	f := docxZipEntry(zr, "word/_rels/document.xml.rels")
	if f == nil {
		return nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil
	}
	rels := map[string]string{}
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "Relationship" {
			id := docxAttr(se, "Id")
			target := docxAttr(se, "Target")
			if id != "" && target != "" {
				// Target is relative to word/; prepend it.
				rels[id] = "word/" + target
			}
		}
	}
	return rels
}

func docxZipEntry(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func docxHasWordDoc(data []byte) bool {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	return docxZipEntry(zr, "word/document.xml") != nil
}

// ── numbering.xml → ordered flag map ──────────────────────────────────────────

func docxParseNumbering(f *zip.File) map[string]bool {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil
	}

	// abstractNumId → ordered (decimal = true)
	abstractOrdered := map[string]bool{}
	// numId → abstractNumId
	numToAbstract := map[string]string{}

	dec := xml.NewDecoder(bytes.NewReader(data))
	var stack []string
	var curAbstract, curNum string

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch v := tok.(type) {
		case xml.StartElement:
			stack = append(stack, v.Name.Local)
			switch v.Name.Local {
			case "abstractNum":
				curAbstract = docxAttr(v, "abstractNumId")
			case "num":
				curNum = docxAttr(v, "numId")
			case "numFmt":
				parent := docxStackParent(stack)
				if parent == "lvl" && curAbstract != "" {
					if _, seen := abstractOrdered[curAbstract]; !seen {
						abstractOrdered[curAbstract] = docxAttr(v, "val") == "decimal"
					}
				}
			case "abstractNumId":
				if docxStackParent(stack) == "num" && curNum != "" {
					numToAbstract[curNum] = docxAttr(v, "val")
				}
			}
		case xml.EndElement:
			switch v.Name.Local {
			case "abstractNum":
				curAbstract = ""
			case "num":
				curNum = ""
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}

	result := make(map[string]bool, len(numToAbstract))
	for numID, absID := range numToAbstract {
		result[numID] = abstractOrdered[absID]
	}
	return result
}

func docxStackParent(stack []string) string {
	if len(stack) < 2 {
		return ""
	}
	return stack[len(stack)-2]
}

// ── document.xml streaming parser ────────────────────────────────────────────

func docxParseDocument(data []byte) ([]any, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("docx: <body> not found in document.xml")
			}
			return nil, err
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "body" {
			return docxParseBody(dec)
		}
	}
}

func docxParseBody(dec *xml.Decoder) ([]any, error) {
	var blocks []any
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			switch v.Name.Local {
			case "p":
				para, err := docxParsePara(dec)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, para)
			case "tbl":
				tbl, err := docxParseTable(dec)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, tbl)
			default:
				if err := dec.Skip(); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if v.Name.Local == "body" {
				return blocks, nil
			}
		}
	}
}

func docxParsePara(dec *xml.Decoder) (docxParaBlock, error) {
	var para docxParaBlock
	for {
		tok, err := dec.Token()
		if err != nil {
			return para, err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			switch v.Name.Local {
			case "pPr":
				if err := docxParseParaProps(dec, &para); err != nil {
					return para, err
				}
			case "r":
				run, err := docxParseRun(dec)
				if err != nil {
					return para, err
				}
				if run.text != "" {
					para.runs = append(para.runs, run)
				}
			case "hyperlink":
				if err := docxParseHyperlink(dec, &para); err != nil {
					return para, err
				}
			case "drawing":
				rID, err := docxParseDrawing(dec)
				if err != nil {
					return para, err
				}
				if rID != "" && para.imageRID == "" {
					para.imageRID = rID
				}
			default:
				if err := dec.Skip(); err != nil {
					return para, err
				}
			}
		case xml.EndElement:
			if v.Name.Local == "p" {
				return para, nil
			}
		}
	}
}

func docxParseParaProps(dec *xml.Decoder, para *docxParaBlock) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			switch v.Name.Local {
			case "pStyle":
				raw := docxAttr(v, "val")
				para.style = strings.ToLower(strings.ReplaceAll(raw, " ", ""))
				if err := dec.Skip(); err != nil {
					return err
				}
			case "numPr":
				if err := docxParseNumPr(dec, para); err != nil {
					return err
				}
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if v.Name.Local == "pPr" {
				return nil
			}
		}
	}
}

func docxParseNumPr(dec *xml.Decoder, para *docxParaBlock) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			switch v.Name.Local {
			case "numId":
				para.numID = docxAttr(v, "val")
			case "ilvl":
				if s := docxAttr(v, "val"); s != "" {
					for _, c := range s {
						if c >= '0' && c <= '9' {
							para.ilvl = para.ilvl*10 + int(c-'0')
						}
					}
				}
			}
			if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if v.Name.Local == "numPr" {
				return nil
			}
		}
	}
}

// docxParseDrawing scans inside a <w:drawing> element for the blip embed rId.
func docxParseDrawing(dec *xml.Decoder) (string, error) {
	var rID string
	for {
		tok, err := dec.Token()
		if err != nil {
			return rID, err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			if v.Name.Local == "blip" {
				for _, a := range v.Attr {
					if a.Name.Local == "embed" {
						rID = a.Value
					}
				}
			}
		case xml.EndElement:
			if v.Name.Local == "drawing" {
				return rID, nil
			}
		}
	}
}

func docxParseRun(dec *xml.Decoder) (docxRun, error) {
	var run docxRun
	for {
		tok, err := dec.Token()
		if err != nil {
			return run, err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			switch v.Name.Local {
			case "rPr":
				if err := docxParseRunProps(dec, &run); err != nil {
					return run, err
				}
			case "t":
				text, err := docxReadText(dec)
				if err != nil {
					return run, err
				}
				run.text += text
			case "br", "tab":
				run.text += " "
				if err := dec.Skip(); err != nil {
					return run, err
				}
			default:
				if err := dec.Skip(); err != nil {
					return run, err
				}
			}
		case xml.EndElement:
			if v.Name.Local == "r" {
				return run, nil
			}
		}
	}
}

func docxParseRunProps(dec *xml.Decoder, run *docxRun) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			switch v.Name.Local {
			case "b":
				val := docxAttr(v, "val")
				run.bold = val != "0" && val != "false"
			case "i":
				val := docxAttr(v, "val")
				run.italic = val != "0" && val != "false"
			case "strike":
				val := docxAttr(v, "val")
				run.strike = val != "0" && val != "false"
			case "rStyle":
				s := strings.ToLower(docxAttr(v, "val"))
				run.code = strings.Contains(s, "code") || strings.Contains(s, "verbatim")
			}
			if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if v.Name.Local == "rPr" {
				return nil
			}
		}
	}
}

func docxParseHyperlink(dec *xml.Decoder, para *docxParaBlock) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			if v.Name.Local == "r" {
				run, err := docxParseRun(dec)
				if err != nil {
					return err
				}
				if run.text != "" {
					para.runs = append(para.runs, run)
				}
			} else {
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if v.Name.Local == "hyperlink" {
				return nil
			}
		}
	}
}

func docxParseTable(dec *xml.Decoder) (docxTableBlock, error) {
	var tbl docxTableBlock
	for {
		tok, err := dec.Token()
		if err != nil {
			return tbl, err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			if v.Name.Local == "tr" {
				row, err := docxParseRow(dec)
				if err != nil {
					return tbl, err
				}
				tbl.rows = append(tbl.rows, row)
			} else {
				if err := dec.Skip(); err != nil {
					return tbl, err
				}
			}
		case xml.EndElement:
			if v.Name.Local == "tbl" {
				return tbl, nil
			}
		}
	}
}

func docxParseRow(dec *xml.Decoder) ([]string, error) {
	var cells []string
	for {
		tok, err := dec.Token()
		if err != nil {
			return cells, err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			if v.Name.Local == "tc" {
				text, span, err := docxParseCell(dec)
				if err != nil {
					return cells, err
				}
				cells = append(cells, text)
				// Fill empty strings for merged columns.
				for i := 1; i < span; i++ {
					cells = append(cells, "")
				}
			} else {
				if err := dec.Skip(); err != nil {
					return cells, err
				}
			}
		case xml.EndElement:
			if v.Name.Local == "tr" {
				return cells, nil
			}
		}
	}
}

func docxParseCell(dec *xml.Decoder) (text string, gridSpan int, err error) {
	gridSpan = 1
	var parts []string
	for {
		tok, tokErr := dec.Token()
		if tokErr != nil {
			return "", gridSpan, tokErr
		}
		switch v := tok.(type) {
		case xml.StartElement:
			switch v.Name.Local {
			case "tcPr":
				// Read tcPr looking for gridSpan.
				if gs, gsErr := docxParseCellProps(dec); gsErr == nil && gs > 1 {
					gridSpan = gs
				}
			case "p":
				para, pErr := docxParsePara(dec)
				if pErr != nil {
					return "", gridSpan, pErr
				}
				if t := docxRunsText(para.runs); t != "" {
					parts = append(parts, t)
				}
			default:
				if err := dec.Skip(); err != nil {
					return "", gridSpan, err
				}
			}
		case xml.EndElement:
			if v.Name.Local == "tc" {
				return strings.Join(parts, " "), gridSpan, nil
			}
		}
	}
}

func docxParseCellProps(dec *xml.Decoder) (gridSpan int, err error) {
	gridSpan = 1
	for {
		tok, err := dec.Token()
		if err != nil {
			return gridSpan, err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			if v.Name.Local == "gridSpan" {
				s := docxAttr(v, "val")
				n := 0
				for _, c := range s {
					if c >= '0' && c <= '9' {
						n = n*10 + int(c-'0')
					}
				}
				if n > 1 {
					gridSpan = n
				}
			}
			if err := dec.Skip(); err != nil {
				return gridSpan, err
			}
		case xml.EndElement:
			if v.Name.Local == "tcPr" {
				return gridSpan, nil
			}
		}
	}
}

func docxReadText(dec *xml.Decoder) (string, error) {
	var sb strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch v := tok.(type) {
		case xml.CharData:
			sb.Write(v)
		case xml.EndElement:
			return sb.String(), nil
		case xml.StartElement:
			if err := dec.Skip(); err != nil {
				return "", err
			}
		}
	}
}

// ── AST conversion ────────────────────────────────────────────────────────────

func docxBlocksToAST(blocks []any, orderedNums map[string]bool, rels map[string]string, zr *zip.Reader) []ast.Node {
	var nodes []ast.Node
	i := 0
	for i < len(blocks) {
		switch v := blocks[i].(type) {
		case docxParaBlock:
			// Embedded image takes priority over text content.
			if v.imageRID != "" {
				if imgNode, ok := docxImageNode(v.imageRID, rels, zr); ok {
					nodes = append(nodes, imgNode)
				}
				i++
				continue
			}

			if v.numID != "" && v.numID != "0" {
				// Collect consecutive list items sharing the same numID.
				numID := v.numID
				ordered := orderedNums[numID]
				var items []string
				var itemSpans [][]ast.InlineSpan
				var itemIndents []int
				for i < len(blocks) {
					p, ok := blocks[i].(docxParaBlock)
					if !ok || p.numID != numID {
						break
					}
					text := docxRunsText(p.runs)
					items = append(items, text)
					itemSpans = append(itemSpans, docxRunsToSpans(p.runs))
					itemIndents = append(itemIndents, p.ilvl)
					i++
				}
				nodes = append(nodes, ast.Node{
					Type:        ast.TypeList,
					Ordered:     ordered,
					Items:       items,
					ItemSpans:   itemSpans,
					ItemIndents: itemIndents,
				})
			} else {
				if n, ok := docxParaToNode(v); ok {
					nodes = append(nodes, n)
				}
				i++
			}
		case docxTableBlock:
			nodes = append(nodes, docxTableToNode(v))
			i++
		default:
			i++
		}
	}
	return nodes
}

func docxImageNode(rID string, rels map[string]string, zr *zip.Reader) (ast.Node, bool) {
	if rels == nil {
		return ast.Node{}, false
	}
	path, ok := rels[rID]
	if !ok {
		return ast.Node{}, false
	}
	f := docxZipEntry(zr, path)
	if f == nil {
		return ast.Node{}, false
	}
	rc, err := f.Open()
	if err != nil {
		return ast.Node{}, false
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		return ast.Node{}, false
	}
	return ast.Node{
		Type: ast.TypeImage,
		Alt:  "image",
		Src:  path,
		Data: data,
	}, true
}

func docxParaToNode(p docxParaBlock) (ast.Node, bool) {
	level := docxHeadingLevel(p.style)
	text := docxRunsText(p.runs)
	if text == "" {
		return ast.Node{}, false
	}
	if level > 0 {
		return ast.Node{
			Type:  ast.TypeHeading,
			Level: level,
			Text:  text,
			Spans: docxRunsToSpans(p.runs),
		}, true
	}
	return ast.Node{
		Type:  ast.TypeParagraph,
		Text:  text,
		Spans: docxRunsToSpans(p.runs),
	}, true
}

func docxTableToNode(t docxTableBlock) ast.Node {
	if len(t.rows) == 0 {
		return ast.Node{Type: ast.TypeTable}
	}
	var dataRows [][]string
	if len(t.rows) > 1 {
		dataRows = t.rows[1:]
	}
	return ast.Node{
		Type:    ast.TypeTable,
		Headers: t.rows[0],
		Rows:    dataRows,
	}
}

func docxHeadingLevel(norm string) int {
	switch norm {
	case "title", "heading1":
		return 1
	case "subtitle", "heading2":
		return 2
	case "heading3", "heading4", "heading5", "heading6":
		return 3
	}
	if strings.HasPrefix(norm, "heading") {
		return 3
	}
	return 0
}

func docxRunsText(runs []docxRun) string {
	var sb strings.Builder
	for _, r := range runs {
		sb.WriteString(r.text)
	}
	return strings.TrimSpace(sb.String())
}

func docxRunsToSpans(runs []docxRun) []ast.InlineSpan {
	allPlain := true
	for _, r := range runs {
		if r.bold || r.italic || r.strike || r.code {
			allPlain = false
			break
		}
	}
	if allPlain {
		return nil
	}
	spans := make([]ast.InlineSpan, 0, len(runs))
	for _, r := range runs {
		if r.text == "" {
			continue
		}
		kind := ast.SpanText
		switch {
		case r.bold && r.italic:
			kind = ast.SpanBoldItalic
		case r.bold:
			kind = ast.SpanBold
		case r.italic:
			kind = ast.SpanItalic
		case r.strike:
			kind = ast.SpanStrike
		case r.code:
			kind = ast.SpanCode
		}
		spans = append(spans, ast.InlineSpan{Kind: kind, Text: r.text})
	}
	return spans
}

func docxAttr(se xml.StartElement, localName string) string {
	for _, a := range se.Attr {
		if a.Name.Local == localName {
			return a.Value
		}
	}
	return ""
}
