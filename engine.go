// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package pdfengine

import (
	"context"
	"fmt"

	"github.com/annavetech/annave-pdf-engine-golang/internal/engine"
)

// Engine converts documents to PDF. Create one with New and reuse it across
// conversions; it is safe for concurrent use and holds no per-conversion state.
type Engine struct {
	pipeline *engine.Pipeline
}

// New builds an Engine from the built-in default configuration, applying
// any Option given, and returns an error if a config file fails to load.
func New(opts ...Option) (*Engine, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	var configOpts []engine.ConfigOption
	if o.stylePath != "" {
		configOpts = append(configOpts, engine.WithStyleFile(o.stylePath))
	}
	if o.limitsPath != "" {
		configOpts = append(configOpts, engine.WithLimitsFile(o.limitsPath))
	}
	if o.style != nil {
		configOpts = append(configOpts, engine.WithStyleOverride(toStyleOverride(*o.style)))
	}

	cfg, err := engine.LoadConfig(configOpts...)
	if err != nil {
		return nil, fmt.Errorf("pdfengine: %w", err)
	}

	return &Engine{pipeline: engine.NewPipeline(cfg)}, nil
}

// Convert renders data as f and returns the resulting PDF bytes. Pass
// FormatAuto to detect the format from the content instead of naming it.
func (e *Engine) Convert(ctx context.Context, data []byte, f Format) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pdf, err := e.pipeline.Run(string(data), f.toInternal())
	if err != nil {
		return nil, translateError(err)
	}
	return pdf, nil
}
