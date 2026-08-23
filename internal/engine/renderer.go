// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"strings"

	"github.com/signintech/gopdf"
)

const (
	pxToMm       = 25.4 / 96.0 // 1 CSS pixel → mm at 96 DPI
	pageWidthMm  = 794 * pxToMm
	pageHeightMm = 1123 * pxToMm
)

// fontName maps the logical font key to the name registered in gopdf.
const (
	fnInterRegular   = "Inter-Regular"
	fnInterItalic    = "Inter-Italic"
	fnInterSemiBold  = "Inter-SemiBold"
	fnInterBold      = "Inter-Bold"
	fnInterExtraBold = "Inter-ExtraBold"
	fnMono           = "JetBrainsMono"
)

// Renderer writes pages to PDF bytes using gopdf.
type Renderer struct {
	pdf *gopdf.GoPdf
}

func NewRenderer() (*Renderer, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{
		PageSize: gopdf.Rect{W: pageWidthMm, H: pageHeightMm},
		Unit:     gopdf.Unit_MM,
	})

	// gopdf assigns PDF object numbers in font registration order, so this
	// order must be fixed rather than ranged over a map — otherwise the
	// same document renders to different bytes on every run.
	fonts := []struct{ name, path string }{
		{fnInterRegular, "fonts/Inter-Regular.ttf"},
		{fnInterItalic, "fonts/Inter-Italic.ttf"},
		{fnInterSemiBold, "fonts/Inter-SemiBold.ttf"},
		{fnInterBold, "fonts/Inter-Bold.ttf"},
		{fnInterExtraBold, "fonts/Inter-ExtraBold.ttf"},
		{fnMono, "fonts/JetBrainsMono-Regular.ttf"},
	}

	for _, f := range fonts {
		name, path := f.name, f.path
		data, err := fs.ReadFile(fontsFS, path)
		if err != nil {
			return nil, fmt.Errorf("renderer: load font %s: %w", name, err)
		}
		if err := pdf.AddTTFFontByReader(name, bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("renderer: register font %s: %w", name, err)
		}
	}

	return &Renderer{pdf: pdf}, nil
}

// Render converts pages → PDF bytes.
func (r *Renderer) Render(pages []Page) ([]byte, error) {
	for _, page := range pages {
		r.pdf.AddPage()
		for _, pb := range page.Boxes {
			if err := r.renderBox(pb); err != nil {
				return nil, err
			}
		}
	}
	b, err := r.pdf.GetBytesPdfReturnErr()
	if err != nil {
		return nil, fmt.Errorf("renderer: write PDF: %w", err)
	}
	return b, nil
}

func (r *Renderer) renderBox(pb PageBox) error {
	node := pb.Node
	style := pb.Style
	x := mm(pb.X)
	y := mm(pb.PageY)
	w := mm(pb.Width)

	switch node.Type {
	case "heading", "paragraph":
		r.pdf.SetTextColor(hexToRGB(style.Color))
		// Use rich per-token rendering when RichLines are available.
		richLines := pb.RichLines
		if pb.LineSlice != nil {
			richLines = richLines[pb.LineSlice[0]:pb.LineSlice[1]]
		}
		if len(richLines) > 0 {
			for _, rline := range richLines {
				cx := x
				lh := mm(rline.Height)
				for _, tok := range rline.Tokens {
					if tok.Text == "" || tok.Kind == "space" && strings.TrimSpace(tok.Text) == "" {
						cx += mm(tok.Width)
						continue
					}
					fn := r.fontForSegment(tok.Segment)
					if err := r.pdf.SetFont(fn, "", tok.Segment.FontSize*pxToMm*2.835); err != nil {
						return err
					}
					r.pdf.SetX(cx)
					r.pdf.SetY(y)
					_ = r.pdf.Cell(&gopdf.Rect{W: mm(tok.Width) + 0.2, H: lh}, tok.Text)
					cx += mm(tok.Width)
				}
				y += lh
			}
		} else {
			// Fallback: plain line rendering.
			fn := r.fontForStyle(style)
			if err := r.pdf.SetFont(fn, "", style.FontSize*pxToMm*2.835); err != nil {
				return err
			}
			lines := pb.Lines
			if pb.LineSlice != nil {
				lines = lines[pb.LineSlice[0]:pb.LineSlice[1]]
			}
			lh := mm(style.FontSize * style.LineHeight)
			for _, line := range lines {
				r.pdf.SetX(x)
				r.pdf.SetY(y)
				_ = r.pdf.Cell(&gopdf.Rect{W: w, H: lh}, line)
				y += lh
			}
		}

	case "list":
		fn := r.fontForStyle(style)
		if err := r.pdf.SetFont(fn, "", style.FontSize*pxToMm*2.835); err != nil {
			return err
		}
		r.pdf.SetTextColor(hexToRGB(style.Color))
		lh := mm(style.FontSize * style.LineHeight)

		itemLines := pb.ItemLines
		start, end := 0, len(itemLines)
		if pb.ItemSlice != nil {
			start, end = pb.ItemSlice[0], pb.ItemSlice[1]
		}

		for i := start; i < end; i++ {
			indent := 0
			if i < len(node.ItemIndents) {
				indent = node.ItemIndents[i]
			}
			indentPx := mm(float64(indent) * 18)
			markerX := x + indentPx
			itemX := markerX + mm(18)
			itemW := w - mm(18) - indentPx

			marker := "•"
			if node.Ordered {
				marker = fmt.Sprintf("%d.", i+1)
			}
			r.pdf.SetX(markerX)
			r.pdf.SetY(y)
			_ = r.pdf.Cell(&gopdf.Rect{W: mm(16), H: lh}, marker)
			for _, line := range itemLines[i] {
				r.pdf.SetX(itemX)
				r.pdf.SetY(y)
				_ = r.pdf.Cell(&gopdf.Rect{W: itemW, H: lh}, line)
				y += lh
			}
			y += mm(listItemGap)
		}

	case "table":
		style2 := style
		headers := node.Headers
		rows := node.Rows
		if pb.RowSlice != nil {
			rows = rows[pb.RowSlice[0]:pb.RowSlice[1]]
		}
		colWidths := pb.ColWidths
		if len(colWidths) == 0 {
			colWidths = evenColWidths(pb.Width, len(headers))
		}

		// Header row
		if len(headers) > 0 {
			style2.FontWeight = "600"
			fn := r.fontForStyle(style2)
			_ = r.pdf.SetFont(fn, "", style.FontSize*pxToMm*2.835)
			r.pdf.SetTextColor(hexToRGB(style.Color))
			hh := mm(tableHeaderH)
			cx := x
			for ci, h := range headers {
				cw := mm(colWidths[ci])
				r.pdf.SetFillColor(235, 235, 235)
				_ = r.pdf.Rectangle(cx, y, cx+cw, y+hh, "FD", 0, 0)
				r.pdf.SetTextColor(hexToRGB(style.Color))
				r.pdf.SetX(cx + mm(4))
				r.pdf.SetY(y + (hh-mm(style.FontSize))/2)
				_ = r.pdf.Cell(&gopdf.Rect{W: cw - mm(8), H: mm(style.FontSize)}, h)
				cx += cw
			}
			y += hh
		}

		// Data rows
		fn := r.fontForStyle(style)
		_ = r.pdf.SetFont(fn, "", style.FontSize*pxToMm*2.835)
		measurer := GetMeasurer()
		lineH := mm(style.FontSize * style.LineHeight)
		for ri, row := range rows {
			rowIdx := ri
			if pb.RowSlice != nil {
				rowIdx = ri + pb.RowSlice[0]
			}
			rh := mm(tableRowH)
			if pb.RowHeights != nil && rowIdx < len(pb.RowHeights) {
				rh = mm(pb.RowHeights[rowIdx])
			}

			var rowR, rowG, rowB uint8
			if rowIdx%2 == 0 {
				rowR, rowG, rowB = 250, 250, 250
			} else {
				rowR, rowG, rowB = 255, 255, 255
			}

			cx := x
			for ci := range colWidths {
				cw := mm(colWidths[ci])
				r.pdf.SetFillColor(rowR, rowG, rowB)
				_ = r.pdf.Rectangle(cx, y, cx+cw, y+rh, "FD", 0, 0)
				r.pdf.SetTextColor(hexToRGB(style.Color))

				cell := ""
				if ci < len(row) {
					cell = row[ci]
				}
				cellW := math.Max(colWidths[ci]-8, 1)
				wrapped := measurer.WrapText(cell, cellW, style)
				if len(wrapped) == 0 {
					wrapped = []string{""}
				}

				textBlockH := float64(len(wrapped)) * lineH
				startY := y + (rh-textBlockH)/2
				for _, line := range wrapped {
					r.pdf.SetX(cx + mm(4))
					r.pdf.SetY(startY)
					_ = r.pdf.Cell(&gopdf.Rect{W: cw - mm(8), H: lineH}, line)
					startY += lineH
				}
				cx += cw
			}
			y += rh
		}

	case "code-block":
		// Background rect
		r.pdf.SetFillColor(246, 246, 246)
		_ = r.pdf.Rectangle(x, y, x+w, y+mm(pb.Height), "F", 0, 0)

		fn := fnMono
		_ = r.pdf.SetFont(fn, "", style.FontSize*pxToMm*2.835)
		r.pdf.SetTextColor(hexToRGB(style.Color))
		lh := mm(style.FontSize * style.LineHeight)
		topPad := mm(12)

		if node.Language != "" {
			r.pdf.SetX(x + mm(16))
			r.pdf.SetY(y + topPad/2)
			_ = r.pdf.Cell(&gopdf.Rect{W: w - mm(32), H: mm(langLabelH)}, node.Language)
			topPad += mm(langLabelH + langLabelGap)
		}

		lines := pb.Lines
		if pb.LineSlice != nil {
			lines = lines[pb.LineSlice[0]:pb.LineSlice[1]]
		}
		cy := y + topPad
		for _, line := range lines {
			r.pdf.SetX(x + mm(16))
			r.pdf.SetY(cy)
			_ = r.pdf.Cell(&gopdf.Rect{W: w - mm(32), H: lh}, line)
			cy += lh
		}

	case "blockquote":
		// Left border line
		r.pdf.SetLineWidth(mm(3))
		r.pdf.SetStrokeColor(200, 200, 200)
		r.pdf.Line(x, y, x, y+mm(pb.Height))
		r.pdf.SetLineWidth(0.2)

		fn := fnInterItalic
		_ = r.pdf.SetFont(fn, "", style.FontSize*pxToMm*2.835)
		r.pdf.SetTextColor(hexToRGB(style.Color))
		lh := mm(style.FontSize * style.LineHeight)
		textX := x + mm(quoteIndent)
		textW := w - mm(quoteIndent)
		cy := y + mm(8)
		for _, line := range pb.Lines {
			r.pdf.SetX(textX)
			r.pdf.SetY(cy)
			_ = r.pdf.Cell(&gopdf.Rect{W: textW, H: lh}, line)
			cy += lh
		}

	case "hr":
		r.pdf.SetLineWidth(0.3)
		r.pdf.SetStrokeColor(200, 200, 200)
		r.pdf.Line(x, y, x+w, y)
		r.pdf.SetLineWidth(0.2)

	case "image":
		rendered := false
		if len(node.Data) > 0 {
			holder, holderErr := gopdf.ImageHolderByBytes(node.Data)
			if holderErr != nil {
				slog.Warn("renderer: ImageHolderByBytes failed", "err", holderErr)
			} else {
				imgErr := r.pdf.ImageByHolder(holder, x, y, &gopdf.Rect{W: w, H: mm(pb.Height)})
				if imgErr != nil {
					slog.Warn("renderer: ImageByHolder failed", "err", imgErr)
				}
				rendered = imgErr == nil
			}
		}
		if !rendered {
			r.pdf.SetFillColor(240, 240, 240)
			_ = r.pdf.Rectangle(x, y, x+w, y+mm(pb.Height), "F", 0, 0)
			if node.Alt != "" {
				_ = r.pdf.SetFont(fnInterRegular, "", style.FontSize*pxToMm*2.835)
				r.pdf.SetTextColor(100, 100, 100)
				r.pdf.SetX(x + mm(8))
				r.pdf.SetY(y + mm(pb.Height)/2)
				_ = r.pdf.Cell(&gopdf.Rect{W: w - mm(16), H: mm(style.FontSize * style.LineHeight)}, "[Image: "+node.Alt+"]")
			}
		}
	}
	return nil
}

func (r *Renderer) fontForSegment(seg TextSegment) string {
	isMono := strings.Contains(seg.FontFamily, "JetBrains") ||
		strings.Contains(seg.FontFamily, "Mono") ||
		strings.Contains(seg.FontFamily, "monospace")
	if isMono {
		return fnMono
	}
	if seg.FontStyle == "italic" {
		return fnInterItalic
	}
	switch seg.FontWeight {
	case "800":
		return fnInterExtraBold
	case "700":
		return fnInterBold
	case "600":
		return fnInterSemiBold
	default:
		return fnInterRegular
	}
}

func (r *Renderer) fontForStyle(style TextStyle) string {
	isMono := strings.Contains(style.FontFamily, "JetBrains") ||
		strings.Contains(style.FontFamily, "Mono") ||
		strings.Contains(style.FontFamily, "monospace")
	if isMono {
		return fnMono
	}
	if style.FontStyle == "italic" {
		return fnInterItalic
	}
	switch style.FontWeight {
	case "800":
		return fnInterExtraBold
	case "700":
		return fnInterBold
	case "600":
		return fnInterSemiBold
	default:
		return fnInterRegular
	}
}

func mm(px float64) float64 {
	return math.Round(px*pxToMm*1000) / 1000
}

func hexToRGB(hex string) (uint8, uint8, uint8) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return 0, 0, 0
	}
	var r, g, b uint8
	_, _ = fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

func evenColWidths(totalW float64, n int) []float64 {
	if n == 0 {
		return nil
	}
	w := totalW / float64(n)
	result := make([]float64, n)
	for i := range result {
		result[i] = w
	}
	return result
}
