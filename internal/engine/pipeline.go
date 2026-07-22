// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"annave.tech/pdf-engine/internal/ast"
	"annave.tech/pdf-engine/internal/parser"
)

// Pipeline orchestrates: normalize → sanitize → parse → validate → layout → paginate → render.
type Pipeline struct {
	registry  *parser.Registry
	layout    *LayoutEngine
	paginator *Paginator
}

func NewPipeline() *Pipeline {
	return &Pipeline{
		registry:  parser.NewRegistry(),
		layout:    NewLayoutEngine(),
		paginator: NewPaginator(),
	}
}

// RunOption is a functional option for Pipeline.Run.
type RunOption func(*runConfig)

type runConfig struct {
	styleOverride *StyleOverride
}

// WithStyleOverride applies a per-request style override on top of the defaults
// from config/style.yaml.
func WithStyleOverride(o *StyleOverride) RunOption {
	return func(cfg *runConfig) { cfg.styleOverride = o }
}

// Run converts raw text input to PDF bytes.
// Optional RunOption values are applied after loading the base style.
func (p *Pipeline) Run(input string, format parser.InputFormat, opts ...RunOption) ([]byte, error) {
	cfg := &runConfig{}
	for _, o := range opts {
		o(cfg)
	}

	maxBytes := appLimits.Input.MaxFileSizeBytes
	if len(input) > maxBytes {
		return nil, NewError("ENGINE_ERR_FILE_TOO_LARGE", StageInput,
			msg("ENGINE_ERR_FILE_TOO_LARGE", "max_mb", maxBytes/1024/1024))
	}

	// Binary formats carry raw bytes — normalization strips sub-0x20 bytes and
	// corrupts the data. Pass binary input directly to the parser.
	isBinary := format == parser.FormatImage || format == parser.FormatDocx ||
		(format == parser.FormatAuto && p.registry.IsBinaryInput(input))

	var (
		normalized string
		err        error
	)
	if isBinary {
		normalized = input
	} else {
		normalized, err = NormalizeInput(input)
		if err != nil {
			return nil, err
		}
	}

	if !isBinary && (format == parser.FormatHTML ||
		(format == parser.FormatAuto && p.registry.LooksLikeHTML(normalized))) {
		normalized = SanitizeHTML(normalized)
	}

	doc, err := p.registry.Parse(normalized, format)
	if err != nil {
		return nil, NewError("ENGINE_ERR_PARSE_FAILED", StageParser, err.Error())
	}

	return p.runFromDoc(doc, cfg)
}

// RunFromDoc allows re-use when a DocumentNode is already available (e.g., DOCX).
func (p *Pipeline) RunFromDoc(doc *ast.DocumentNode, opts ...RunOption) ([]byte, error) {
	cfg := &runConfig{}
	for _, o := range opts {
		o(cfg)
	}
	return p.runFromDoc(doc, cfg)
}

func (p *Pipeline) runFromDoc(doc *ast.DocumentNode, cfg *runConfig) ([]byte, error) {
	if err := ValidateDocument(doc); err != nil {
		return nil, err
	}

	docStyle := MergeDocStyle(DocStyle, cfg.styleOverride)

	boxes := p.layout.Compute(doc, docStyle)
	pages := p.paginator.Paginate(boxes, docStyle.Page)

	maxPages := appLimits.Document.MaxPages
	if len(pages) > maxPages {
		return nil, NewError("ENGINE_ERR_TOO_MANY_PAGES", StagePagination,
			msg("ENGINE_ERR_TOO_MANY_PAGES", "page_count", len(pages), "max_pages", maxPages))
	}

	renderer, err := NewRenderer()
	if err != nil {
		return nil, NewError("ENGINE_ERR_RENDERER_INIT", StageRender,
			msg("ENGINE_ERR_RENDERER_INIT", "detail", err.Error()))
	}

	pdf, err := renderer.Render(pages)
	if err != nil {
		return nil, NewError("ENGINE_ERR_RENDER_FAILED", StageRender,
			msg("ENGINE_ERR_RENDER_FAILED", "detail", err.Error()))
	}

	return pdf, nil
}
