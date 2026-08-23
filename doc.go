// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

// Package pdfengine converts Markdown, plain text, JSON, HTML, CSV, YAML,
// XML, reStructuredText, Jupyter notebooks, Word documents, and raster
// images to PDF.
//
// The engine is self-contained: no headless browser, no CGO, no outbound
// network calls. Fonts and configuration are embedded at build time.
//
// Basic usage:
//
//	pdf, err := pdfengine.New().Convert(text, pdfengine.FormatMarkdown)
//	if err != nil {
//	    var pe *pdfengine.Error
//	    if errors.As(err, &pe) {
//	        // pe.Code is a stable, machine-readable identifier.
//	    }
//	    return err
//	}
//
// An Engine is safe to reuse across many calls to Convert; it holds no
// per-request state.
package pdfengine
