// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package pdfengine

import (
	"github.com/annavetech/annave-pdf-engine-golang/internal/engine"
)

// Engine converts documents to PDF. Create one with New and reuse it across
// conversions; it holds no per-request state.
type Engine struct {
	pipeline *engine.Pipeline
}

// New returns a ready-to-use Engine with the built-in default configuration
// (config/style.yaml, config/limits.yaml, embedded fonts).
func New() *Engine {
	return &Engine{pipeline: engine.NewPipeline()}
}

// Convert renders input as f and returns the resulting PDF bytes. Pass
// FormatAuto to detect the format from the content itself rather than
// naming it explicitly.
//
// On failure, the returned error can be inspected with errors.As into
// *Error to read the machine-readable code and the pipeline stage that
// failed.
func (e *Engine) Convert(input string, f Format, opts ...Option) ([]byte, error) {
	cfg := &options{}
	for _, o := range opts {
		o(cfg)
	}

	var runOpts []engine.RunOption
	if cfg.style != nil {
		runOpts = append(runOpts, engine.WithStyleOverride(toStyleOverride(*cfg.style)))
	}

	pdf, err := e.pipeline.Run(input, f.toInternal(), runOpts...)
	if err != nil {
		return nil, translateError(err)
	}
	return pdf, nil
}
