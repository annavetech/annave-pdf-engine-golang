// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/annavetech/annave-pdf-engine-golang/internal/ast"
	_ "golang.org/x/image/webp"
)

// ImageParser handles raster image uploads: PNG, JPEG, GIF, WebP.
type ImageParser struct{}

func (p *ImageParser) CanParse(input string) bool {
	return isImageBytes([]byte(input))
}

func (p *ImageParser) Parse(input string) (*ast.DocumentNode, error) {
	b := []byte(input)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("image: decode config: %w", err)
	}
	return &ast.DocumentNode{
		Type: ast.TypeDocument,
		Children: []ast.Node{{
			Type:          ast.TypeImage,
			Alt:           "image",
			NaturalWidth:  float64(cfg.Width),
			NaturalHeight: float64(cfg.Height),
			Data:          b,
		}},
	}, nil
}

func isImageBytes(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	// PNG: \x89PNG
	if b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G' {
		return true
	}
	// JPEG: \xff\xd8\xff
	if b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff {
		return true
	}
	// GIF: GIF8
	if b[0] == 'G' && b[1] == 'I' && b[2] == 'F' && b[3] == '8' {
		return true
	}
	// WebP: RIFF????WEBP
	if len(b) >= 12 && b[0] == 'R' && b[1] == 'I' && b[2] == 'F' && b[3] == 'F' &&
		b[8] == 'W' && b[9] == 'E' && b[10] == 'B' && b[11] == 'P' {
		return true
	}
	return false
}
