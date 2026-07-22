// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import "math"

// Paginator constants — must match layout.go
const (
	minBottomSpace = 20
	minOrphanLines = 2
)

type PageBox struct {
	LayoutBox // embedded
	PageY     float64
	LineSlice *[2]int // [start, end) of Lines to render
	ItemSlice *[2]int // [start, end) of ItemLines to render
	RowSlice  *[2]int // [start, end) of node.Rows to render
	// HeightOverride replaces LayoutBox.Height when non-zero.
	HeightOverride float64
}

type Page struct {
	Index int
	Boxes []PageBox
}

type Paginator struct{}

func NewPaginator() *Paginator { return &Paginator{} }

func (pag *Paginator) Paginate(boxes []LayoutBox, page PageConfig) []Page {
	if len(boxes) == 0 {
		return nil
	}

	contentBottom := page.Height - page.MarginBottom

	var pages []Page
	current := pag.newPage(&pages)
	cursor := page.MarginTop

	place := func(box LayoutBox, pageY float64, overrides *PageBox) {
		pb := PageBox{LayoutBox: box, PageY: pageY}
		if overrides != nil {
			if overrides.LineSlice != nil {
				pb.LineSlice = overrides.LineSlice
			}
			if overrides.ItemSlice != nil {
				pb.ItemSlice = overrides.ItemSlice
			}
			if overrides.RowSlice != nil {
				pb.RowSlice = overrides.RowSlice
			}
			if overrides.HeightOverride != 0 {
				pb.LayoutBox.Height = overrides.HeightOverride
			}
		}
		current.Boxes = append(current.Boxes, pb)
	}

	startNewPage := func() {
		current = pag.newPage(&pages)
		cursor = page.MarginTop
	}

	for _, box := range boxes {
		avail := contentBottom - cursor - minBottomSpace
		fits := box.Height <= avail

		switch box.Node.Type {
		case "heading":
			afterHeading := avail - box.Height - box.Style.MarginBottom
			if (!fits || afterHeading < minBottomSpace) && len(current.Boxes) > 0 {
				startNewPage()
			}
			place(box, cursor, nil)
			cursor += box.Height + box.Style.MarginBottom

		case "table":
			if fits {
				place(box, cursor, nil)
				cursor += box.Height + box.Style.MarginBottom
				continue
			}
			headerH := 0.0
			if len(box.Node.Headers) > 0 {
				headerH = tableHeaderH
			}
			avail := contentBottom - cursor - minBottomSpace - headerH
			availRows := 0
			usedH := 0.0
			for ri := range box.Node.Rows {
				rh := float64(tableRowH)
				if ri < len(box.RowHeights) {
					rh = box.RowHeights[ri]
				}
				if usedH+rh > avail {
					break
				}
				usedH += rh
				availRows++
			}
			totalRows := len(box.Node.Rows)
			restRows := totalRows - availRows
			if availRows >= 1 && restRows >= 1 && len(current.Boxes) > 0 {
				h1 := headerH + usedH
				slice1 := [2]int{0, availRows}
				place(box, cursor, &PageBox{HeightOverride: h1, RowSlice: &slice1})
				startNewPage()
				h2 := headerH
				for ri := availRows; ri < totalRows; ri++ {
					rh := float64(tableRowH)
					if ri < len(box.RowHeights) {
						rh = box.RowHeights[ri]
					}
					h2 += rh
				}
				slice2 := [2]int{availRows, totalRows}
				place(box, cursor, &PageBox{HeightOverride: h2, RowSlice: &slice2})
				cursor += h2 + box.Style.MarginBottom
			} else {
				if !fits && len(current.Boxes) > 0 {
					startNewPage()
				}
				place(box, cursor, nil)
				cursor += box.Height + box.Style.MarginBottom
			}

		case "code-block":
			if fits || len(box.Lines) <= 1 {
				place(box, cursor, nil)
				cursor += box.Height + box.Style.MarginBottom
				continue
			}
			langExtra := 0.0
			if box.Node.Language != "" {
				langExtra = 13
			}
			lh := (box.Height - codePadding - langExtra) / float64(len(box.Lines))
			nFit := int(math.Floor((contentBottom - cursor - minBottomSpace - codePadding - langExtra) / lh))
			nRest := len(box.Lines) - nFit
			if nFit >= minOrphanLines && nRest >= 1 && len(current.Boxes) > 0 {
				slice1 := [2]int{0, nFit}
				place(box, cursor, &PageBox{HeightOverride: roundF(float64(nFit)*lh + codePadding), LineSlice: &slice1})
				startNewPage()
				restH := roundF(float64(nRest)*lh + codePadding)
				slice2 := [2]int{nFit, len(box.Lines)}
				place(box, cursor, &PageBox{HeightOverride: restH, LineSlice: &slice2})
				cursor += restH + box.Style.MarginBottom
			} else {
				if !fits && len(current.Boxes) > 0 {
					startNewPage()
				}
				place(box, cursor, nil)
				cursor += box.Height + box.Style.MarginBottom
			}

		case "image", "hr", "blockquote":
			if !fits && len(current.Boxes) > 0 {
				startNewPage()
			}
			place(box, cursor, nil)
			cursor += box.Height + box.Style.MarginBottom

		case "paragraph":
			if fits || len(box.Lines) <= 1 {
				place(box, cursor, nil)
				cursor += box.Height + box.Style.MarginBottom
				continue
			}
			lh := box.Height / float64(len(box.Lines))
			nFit := int(math.Floor((contentBottom - cursor - minBottomSpace) / lh))
			nRest := len(box.Lines) - nFit
			if nFit >= minOrphanLines && nRest >= minOrphanLines {
				slice1 := [2]int{0, nFit}
				place(box, cursor, &PageBox{HeightOverride: roundF(float64(nFit) * lh), LineSlice: &slice1})
				startNewPage()
				restH := roundF(float64(nRest) * lh)
				slice2 := [2]int{nFit, len(box.Lines)}
				place(box, cursor, &PageBox{HeightOverride: restH, LineSlice: &slice2})
				cursor += restH + box.Style.MarginBottom
			} else if len(current.Boxes) > 0 {
				startNewPage()
				place(box, cursor, nil)
				cursor += box.Height + box.Style.MarginBottom
			} else {
				place(box, cursor, nil)
				cursor += box.Height + box.Style.MarginBottom
			}

		case "list":
			if fits || len(box.ItemLines) <= 1 {
				place(box, cursor, nil)
				cursor += box.Height + box.Style.MarginBottom
				continue
			}
			lh := lineHStyle(box.Style)
			accH := 0.0
			splitAt := 0
			for j, item := range box.ItemLines {
				gap := 0.0
				if j < len(box.ItemLines)-1 {
					gap = listItemGap
				}
				itemH := float64(len(item))*lh + gap
				if accH+itemH > contentBottom-cursor-minBottomSpace {
					break
				}
				accH += itemH
				splitAt = j + 1
			}
			remaining := len(box.ItemLines) - splitAt
			if splitAt >= minOrphanLines && remaining >= 1 {
				slice1 := [2]int{0, splitAt}
				place(box, cursor, &PageBox{HeightOverride: roundF(accH), ItemSlice: &slice1})
				startNewPage()
				restH := 0.0
				for j := splitAt; j < len(box.ItemLines); j++ {
					gap := 0.0
					if j < len(box.ItemLines)-1 {
						gap = listItemGap
					}
					restH += float64(len(box.ItemLines[j]))*lh + gap
				}
				slice2 := [2]int{splitAt, len(box.ItemLines)}
				place(box, cursor, &PageBox{HeightOverride: roundF(restH), ItemSlice: &slice2})
				cursor += roundF(restH) + box.Style.MarginBottom
			} else if len(current.Boxes) > 0 {
				startNewPage()
				place(box, cursor, nil)
				cursor += box.Height + box.Style.MarginBottom
			} else {
				place(box, cursor, nil)
				cursor += box.Height + box.Style.MarginBottom
			}

		default:
			if !fits && len(current.Boxes) > 0 {
				startNewPage()
			}
			place(box, cursor, nil)
			cursor += box.Height + box.Style.MarginBottom
		}
	}

	return pages
}

func (pag *Paginator) newPage(pages *[]Page) *Page {
	p := Page{Index: len(*pages)}
	*pages = append(*pages, p)
	return &(*pages)[len(*pages)-1]
}

func lineHStyle(style TextStyle) float64 {
	return style.FontSize * style.LineHeight
}

func roundF(v float64) float64 {
	return math.Round(v*1000) / 1000
}
