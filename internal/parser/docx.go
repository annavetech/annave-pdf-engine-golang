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

	"github.com/annavetech/annave-pdf-engine-golang/internal/ast"
)

// DocxParser parses Microsoft Word (.docx) files.
type DocxParser struct{}

func (p *DocxParser) CanParse(input []byte) bool {
	if len(input) < 4 || input[0] != 0x50 || input[1] != 0x4b || input[2] != 0x03 || input[3] != 0x04 {
		return false
	}
	da, err := newDocxArchive(input)
	if err != nil {
		return false
	}
	return da.entry("word/document.xml") != nil
}

func (p *DocxParser) Parse(input []byte) (*ast.DocumentNode, error) {
	da, err := newDocxArchive(input)
	if err != nil {
		return nil, err
	}

	orderedNums := map[string]bool{}
	if numData, err := da.readNamedEntry("word/numbering.xml"); err != nil {
		return nil, err
	} else if numData != nil {
		orderedNums = docxParseNumbering(numData)
	}

	// Build relationship map: rId → image path inside the ZIP.
	relsData, err := da.readNamedEntry("word/_rels/document.xml.rels")
	if err != nil {
		return nil, err
	}
	rels := docxParseRelsXML(relsData)

	xmlData, err := da.readNamedEntry("word/document.xml")
	if err != nil {
		return nil, err
	}
	if xmlData == nil {
		return nil, fmt.Errorf("docx: word/document.xml not found")
	}

	blocks, err := docxParseDocument(xmlData)
	if err != nil {
		return nil, err
	}

	children, err := docxBlocksToAST(blocks, orderedNums, rels, da)
	if err != nil {
		return nil, err
	}

	return &ast.DocumentNode{
		Type:     ast.TypeDocument,
		Children: children,
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

// Safety caps on decompressed docx content, checked before and after
// decompression so a small, pathological archive (a "zip bomb") cannot exhaust memory.
const (
	// maxDocxEntryBytes caps the uncompressed size of any single zip entry
	// read from a .docx archive.
	maxDocxEntryBytes uint64 = 50 * 1024 * 1024

	// maxDocxTotalBytes caps the cumulative uncompressed bytes read across
	// every entry decoded from one .docx archive.
	maxDocxTotalBytes uint64 = 150 * 1024 * 1024
)

// docxArchive indexes a .docx zip archive's entries for O(1) lookup and
// enforces the decompressed-size caps across every entry read through it.
type docxArchive struct {
	files map[string]*zip.File
	read  uint64 // cumulative uncompressed bytes read so far via readFile
}

// newDocxArchive opens data as a zip archive and indexes its entries.
func newDocxArchive(data []byte) (*docxArchive, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("docx: invalid archive: %w", err)
	}
	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		files[f.Name] = f
	}
	return &docxArchive{files: files}, nil
}

// entry returns the zip.File for name, or nil if the archive has no such
// entry. This is an O(1) map lookup, not a linear scan.
func (da *docxArchive) entry(name string) *zip.File {
	return da.files[name]
}

// readNamedEntry reads and returns the full uncompressed content of the
// entry named name, or (nil, nil) if the archive has no such entry.
func (da *docxArchive) readNamedEntry(name string) ([]byte, error) {
	f := da.entry(name)
	if f == nil {
		return nil, nil
	}
	return da.readFile(f)
}

// readFile reads the full uncompressed content of f, enforcing both the
// per-entry cap and the cumulative cap shared across this archive.
func (da *docxArchive) readFile(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > maxDocxEntryBytes {
		return nil, fmt.Errorf("docx: entry %q declares %d uncompressed bytes, exceeding the %d byte per-entry cap",
			f.Name, f.UncompressedSize64, maxDocxEntryBytes)
	}
	// da.read never exceeds maxDocxTotalBytes, so this subtraction cannot underflow.
	if f.UncompressedSize64 > maxDocxTotalBytes-da.read {
		return nil, fmt.Errorf("docx: reading entry %q would bring the archive's cumulative uncompressed bytes to %d, exceeding the %d byte archive-wide cap",
			f.Name, da.read+f.UncompressedSize64, maxDocxTotalBytes)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("docx: open %q: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	// Read one byte past the cap so an entry whose actual decompressed size
	// exceeds its declared size is caught here, not by exhausting memory.
	data, err := io.ReadAll(io.LimitReader(rc, int64(maxDocxEntryBytes)+1)) //nolint:gosec // maxDocxEntryBytes is a fixed 50 MiB constant
	if err != nil {
		return nil, fmt.Errorf("docx: read %q: %w", f.Name, err)
	}
	if uint64(len(data)) > maxDocxEntryBytes {
		return nil, fmt.Errorf("docx: entry %q exceeds the %d byte per-entry cap after decompression",
			f.Name, maxDocxEntryBytes)
	}
	da.read += uint64(len(data))
	return data, nil
}

func docxParseRelsXML(data []byte) map[string]string {
	if data == nil {
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

// ── numbering.xml → ordered flag map ──────────────────────────────────────────

func docxParseNumbering(data []byte) map[string]bool {
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

func docxBlocksToAST(blocks []any, orderedNums map[string]bool, rels map[string]string, da *docxArchive) ([]ast.Node, error) {
	var nodes []ast.Node
	i := 0
	for i < len(blocks) {
		switch v := blocks[i].(type) {
		case docxParaBlock:
			// Embedded image takes priority over text content.
			if v.imageRID != "" {
				imgNode, ok, err := docxImageNode(v.imageRID, rels, da)
				if err != nil {
					return nil, err
				}
				if ok {
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
	return nodes, nil
}

// docxImageNode reads the embedded image referenced by rID and returns it
// as an ast.Node. ok is false when the image is not resolvable.
func docxImageNode(rID string, rels map[string]string, da *docxArchive) (ast.Node, bool, error) {
	if rels == nil {
		return ast.Node{}, false, nil
	}
	path, ok := rels[rID]
	if !ok {
		return ast.Node{}, false, nil
	}
	f := da.entry(path)
	if f == nil {
		return ast.Node{}, false, nil
	}
	data, err := da.readFile(f)
	if err != nil {
		return ast.Node{}, false, fmt.Errorf("docx: embedded image %q: %w", path, err)
	}
	return ast.Node{
		Type: ast.TypeImage,
		Alt:  "image",
		Src:  path,
		Data: data,
	}, true, nil
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
