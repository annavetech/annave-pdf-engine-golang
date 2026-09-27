// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"github.com/annavetech/pdfengine/internal/ast"
	"github.com/annavetech/pdfengine/internal/parser"
)

// Pipeline orchestrates normalize, sanitize, parse, validate, layout, paginate, and render.
// It is safe for concurrent use; a Pipeline's Config is fixed at construction.
type Pipeline struct {
	cfg       *Config
	registry  *parser.Registry
	layout    *LayoutEngine
	paginator *Paginator
}

// NewPipeline returns a Pipeline that runs with cfg. Build cfg with
// LoadConfig.
func NewPipeline(cfg *Config) *Pipeline {
	return &Pipeline{
		cfg:       cfg,
		registry:  parser.NewRegistry(),
		layout:    NewLayoutEngine(),
		paginator: NewPaginator(),
	}
}

// Run converts raw text input to PDF bytes, using the style, limits, and
// messages fixed on p at construction.
func (p *Pipeline) Run(input string, format parser.InputFormat) ([]byte, error) {
	// An unrecognised format is rejected before any payload-dependent work begins.
	if !p.registry.SupportsFormat(format) {
		return nil, NewError("ENGINE_ERR_UNSUPPORTED_FORMAT", StageInput,
			p.cfg.msg("ENGINE_ERR_UNSUPPORTED_FORMAT", "format", string(format)))
	}

	maxBytes := p.cfg.Limits.Input.MaxFileSizeBytes
	if len(input) > maxBytes {
		return nil, NewError("ENGINE_ERR_FILE_TOO_LARGE", StageInput,
			p.cfg.msg("ENGINE_ERR_FILE_TOO_LARGE", "max_mb", maxBytes/1024/1024))
	}

	// Binary formats carry raw bytes; normalization strips sub-0x20 bytes and
	// corrupts the data. Pass binary input directly to the parser.
	isBinary := p.registry.IsBinaryFormat(format) ||
		(format == parser.FormatAuto && p.registry.IsBinaryInput([]byte(input)))

	var (
		normalized string
		err        error
	)
	if isBinary {
		normalized = input
	} else {
		normalized, err = NormalizeInput(input, p.cfg)
		if err != nil {
			return nil, err
		}
	}

	if !isBinary && (format == parser.FormatHTML ||
		(format == parser.FormatAuto && p.registry.LooksLikeHTML([]byte(normalized)))) {
		normalized = SanitizeHTML(normalized)
	}

	doc, err := p.registry.Parse([]byte(normalized), format)
	if err != nil {
		return nil, NewError("ENGINE_ERR_PARSE_FAILED", StageParser, err.Error())
	}

	return p.runFromDoc(doc)
}

// RunFromDoc allows re-use when a DocumentNode is already available (e.g., DOCX).
func (p *Pipeline) RunFromDoc(doc *ast.DocumentNode) ([]byte, error) {
	return p.runFromDoc(doc)
}

func (p *Pipeline) runFromDoc(doc *ast.DocumentNode) ([]byte, error) {
	if err := ValidateDocument(doc, p.cfg); err != nil {
		return nil, err
	}

	docStyle := p.cfg.Style

	boxes := p.layout.Compute(doc, docStyle)
	pages := p.paginator.Paginate(boxes, docStyle.Page)

	maxPages := p.cfg.Limits.Document.MaxPages
	if len(pages) > maxPages {
		return nil, NewError("ENGINE_ERR_TOO_MANY_PAGES", StagePagination,
			p.cfg.msg("ENGINE_ERR_TOO_MANY_PAGES", "page_count", len(pages), "max_pages", maxPages))
	}

	renderer, err := NewRenderer()
	if err != nil {
		return nil, NewError("ENGINE_ERR_RENDERER_INIT", StageRender,
			p.cfg.msg("ENGINE_ERR_RENDERER_INIT", "detail", err.Error()))
	}

	pdf, err := renderer.Render(pages)
	if err != nil {
		return nil, NewError("ENGINE_ERR_RENDER_FAILED", StageRender,
			p.cfg.msg("ENGINE_ERR_RENDER_FAILED", "detail", err.Error()))
	}

	return pdf, nil
}
