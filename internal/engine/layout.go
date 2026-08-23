// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"math"
	"strings"

	"github.com/annavetech/annave-pdf-engine-golang/internal/ast"
)

// Layout constants — must match paginator.go
const (
	listItemGap       = 4
	tableRowH         = 26
	tableHeaderH      = 30
	codePadding       = 24
	langLabelH        = 9
	langLabelGap      = 4
	quotePadding      = 16
	quoteIndent       = 35
	codeIndent        = 32
	hrHeight          = 1
	hrSpacing         = 16
	imageHeightInline = 80
	imageMaxHeight    = 700
)

type LayoutBox struct {
	Node       ast.Node
	X          float64
	Y          float64
	Width      float64
	Height     float64
	Lines      []string
	RichLines  []LayoutLine // parallel to Lines; non-nil when spans are present
	ItemLines  [][]string
	ColWidths  []float64
	RowHeights []float64 // per-row heights for tables, parallel to node.Rows
	Style      TextStyle
}

type LayoutEngine struct {
	measurer *TextMeasurer
}

func NewLayoutEngine() *LayoutEngine {
	return &LayoutEngine{measurer: GetMeasurer()}
}

func (le *LayoutEngine) Compute(doc *ast.DocumentNode, ds DocumentStyle) []LayoutBox {
	page := ds.Page
	contentWidth := page.Width - page.MarginX*2
	var boxes []LayoutBox
	cursor := page.MarginTop

	for _, node := range doc.Children {
		switch node.Type {
		case ast.TypeHeading, ast.TypeParagraph:
			style := le.styleForNode(node, ds)
			// Always build rich lines so the renderer can switch fonts per token.
			var segs []TextSegment
			if len(node.Spans) > 0 {
				segs = le.measurer.SpansToSegments(node.Spans, style)
			} else {
				segs = le.measurer.TextToSegments(node.Text, style)
			}
			ll := le.measurer.BuildLines(segs, contentWidth)
			height := round(le.measurer.LinesHeight(ll))
			lines := extractLineStrings(ll)
			boxes = append(boxes, LayoutBox{
				Node: node, X: page.MarginX, Y: round(cursor),
				Width: contentWidth, Height: height, Lines: lines, RichLines: ll, Style: style,
			})
			cursor += height + style.MarginBottom

		case ast.TypeList:
			style := ds.Paragraph
			markerW := 18.0
			itemWidth := contentWidth - markerW
			var itemLines [][]string
			totalHeight := 0.0
			for idx, item := range node.Items {
				var wrapped []string
				var itemH float64
				if idx < len(node.ItemSpans) && len(node.ItemSpans[idx]) > 0 {
					segs := le.measurer.SpansToSegments(node.ItemSpans[idx], style)
					ll := le.measurer.BuildLines(segs, itemWidth)
					wrapped = extractLineStrings(ll)
					itemH = le.measurer.LinesHeight(ll)
				} else {
					wrapped = le.measurer.WrapText(item, itemWidth, style)
					itemH = le.measurer.BlockHeight(wrapped, style)
				}
				itemLines = append(itemLines, wrapped)
				totalHeight += itemH + listItemGap
			}
			if len(node.Items) > 0 {
				totalHeight -= listItemGap
			}
			listHeight := round(math.Max(totalHeight, 0))
			boxes = append(boxes, LayoutBox{
				Node: node, X: page.MarginX, Y: round(cursor),
				Width: contentWidth, Height: listHeight, Lines: nil, ItemLines: itemLines, Style: style,
			})
			cursor += listHeight + style.MarginBottom

		case ast.TypeTable:
			style := ds.Paragraph
			headerStyle := style
			headerStyle.FontWeight = "600"
			numCols := len(node.Headers)
			for _, row := range node.Rows {
				if len(row) > numCols {
					numCols = len(row)
				}
			}
			if numCols == 0 {
				numCols = 1
			}
			naturalWidths := make([]float64, numCols)
			for ci := range naturalWidths {
				var headerW float64
				if ci < len(node.Headers) {
					headerW = le.measurer.MeasureWidth(node.Headers[ci], headerStyle)
				}
				cellW := 0.0
				for _, row := range node.Rows {
					text := ""
					if ci < len(row) {
						text = row[ci]
					}
					w := le.measurer.MeasureWidth(text, style)
					if w > cellW {
						cellW = w
					}
				}
				if headerW > cellW {
					naturalWidths[ci] = headerW
				} else {
					naturalWidths[ci] = cellW
				}
			}
			totalNatural := 0.0
			for _, w := range naturalWidths {
				totalNatural += w
			}
			// Per-column minimum: widest single word + cell padding (8px).
			// This prevents mid-word line breaks after proportional scaling.
			wordMinWidths := make([]float64, numCols)
			for ci := range wordMinWidths {
				wm := 40.0
				if ci < len(node.Headers) {
					for _, word := range strings.Fields(node.Headers[ci]) {
						if w := le.measurer.MeasureWidth(word, headerStyle) + 8; w > wm {
							wm = w
						}
					}
				}
				for _, row := range node.Rows {
					text := ""
					if ci < len(row) {
						text = row[ci]
					}
					for _, word := range strings.Fields(text) {
						if w := le.measurer.MeasureWidth(word, style) + 8; w > wm {
							wm = w
						}
					}
				}
				wordMinWidths[ci] = wm
			}

			// Allocate column widths:
			// 1. Lock each column to its word minimum (prevents mid-word breaks).
			// 2. Distribute remaining slack proportionally based on natural content width.
			// 3. If even the minimums exceed contentWidth, scale minimums uniformly.
			totalMin := 0.0
			for _, w := range wordMinWidths {
				totalMin += w
			}

			colWidths := make([]float64, numCols)
			if totalMin >= contentWidth {
				// No slack: scale minimums uniformly so the table fits.
				scale := contentWidth / totalMin
				for i, w := range wordMinWidths {
					colWidths[i] = round(w * scale)
				}
			} else {
				slack := contentWidth - totalMin
				totalExtra := 0.0
				for i, nat := range naturalWidths {
					if nat > wordMinWidths[i] {
						totalExtra += nat - wordMinWidths[i]
					}
				}
				for i, nat := range naturalWidths {
					extra := 0.0
					if totalExtra > 0 && nat > wordMinWidths[i] {
						extra = ((nat - wordMinWidths[i]) / totalExtra) * slack
					} else if totalExtra == 0 {
						extra = slack / float64(numCols)
					}
					colWidths[i] = round(wordMinWidths[i] + extra)
				}
			}
			headerH := 0.0
			if len(node.Headers) > 0 {
				headerH = tableHeaderH
			}
			rowHeights := make([]float64, len(node.Rows))
			for ri, row := range node.Rows {
				maxLines := 1
				for ci, cw := range colWidths {
					cellW := math.Max(cw-8, 1)
					text := ""
					if ci < len(row) {
						text = row[ci]
					}
					wrapped := le.measurer.WrapText(text, cellW, style)
					if len(wrapped) > maxLines {
						maxLines = len(wrapped)
					}
				}
				rowHeights[ri] = tableRowH + float64(maxLines-1)*style.FontSize*style.LineHeight
			}
			height := headerH
			for _, rh := range rowHeights {
				height += rh
			}
			boxes = append(boxes, LayoutBox{
				Node: node, X: page.MarginX, Y: round(cursor),
				Width: contentWidth, Height: height, Lines: nil, ColWidths: colWidths, RowHeights: rowHeights, Style: style,
			})
			cursor += height + style.MarginBottom

		case ast.TypeCodeBlock:
			style := ds.Code
			var allWrapped []string
			for _, line := range splitLines(node.Text) {
				text := line
				if text == "" {
					text = " "
				}
				wrapped := le.measurer.WrapText(text, contentWidth-codeIndent, style)
				allWrapped = append(allWrapped, wrapped...)
			}
			langExtra := 0.0
			if node.Language != "" {
				langExtra = langLabelH + langLabelGap
			}
			height := round(le.measurer.BlockHeight(allWrapped, style) + codePadding + langExtra)
			boxes = append(boxes, LayoutBox{
				Node: node, X: page.MarginX, Y: round(cursor),
				Width: contentWidth, Height: height, Lines: allWrapped, Style: style,
			})
			cursor += height + style.MarginBottom

		case ast.TypeBlockquote:
			style := ds.Blockquote
			segs := le.measurer.SpansToSegments(node.Spans, style)
			ll := le.measurer.BuildLines(segs, contentWidth-quoteIndent)
			lines := extractLineStrings(ll)
			height := round(le.measurer.LinesHeight(ll) + quotePadding)
			boxes = append(boxes, LayoutBox{
				Node: node, X: page.MarginX, Y: round(cursor),
				Width: contentWidth, Height: height, Lines: lines, Style: style,
			})
			cursor += height + style.MarginBottom

		case ast.TypeHR:
			style := ds.Paragraph
			boxes = append(boxes, LayoutBox{
				Node: node, X: page.MarginX, Y: round(cursor),
				Width: contentWidth, Height: hrHeight, Lines: nil, Style: style,
			})
			cursor += hrHeight + hrSpacing

		case ast.TypeImage:
			style := ds.Paragraph
			imgH := float64(imageHeightInline)
			if node.NaturalWidth > 0 {
				ratio := node.NaturalHeight / node.NaturalWidth
				imgH = math.Min(math.Round(contentWidth*ratio), imageMaxHeight)
			}
			boxes = append(boxes, LayoutBox{
				Node: node, X: page.MarginX, Y: round(cursor),
				Width: contentWidth, Height: imgH, Lines: nil, Style: style,
			})
			cursor += imgH + style.MarginBottom
		}
	}

	return boxes
}

func (le *LayoutEngine) styleForNode(node ast.Node, ds DocumentStyle) TextStyle {
	if node.Type == ast.TypeHeading {
		switch node.Level {
		case 1:
			return ds.Heading1
		case 2:
			return ds.Heading2
		default:
			return ds.Heading3
		}
	}
	return ds.Paragraph
}

func round(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func extractLineStrings(lines []LayoutLine) []string {
	result := make([]string, len(lines))
	for i, l := range lines {
		var sb strings.Builder
		for _, t := range l.Tokens {
			sb.WriteString(t.Text)
		}
		result[i] = sb.String()
	}
	return result
}

func splitLines(s string) []string {
	return strings.Split(s, "\n")
}
