// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"strings"
	"testing"
)

// pngHeaderOnly returns a PNG signature and IHDR chunk declaring w x h, with no pixel data.
func pngHeaderOnly(w, h uint32) []byte {
	chunk := []byte("IHDR")
	chunk = binary.BigEndian.AppendUint32(chunk, w)
	chunk = binary.BigEndian.AppendUint32(chunk, h)
	chunk = append(chunk, 8, 2, 0, 0, 0) // 8-bit RGB, default compression, filter, no interlace
	out := []byte("\x89PNG\r\n\x1a\n")
	out = binary.BigEndian.AppendUint32(out, 13)
	out = append(out, chunk...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(chunk))
}

func TestImageParser_Parse_RejectsOversizedDimensions(t *testing.T) {
	cases := []struct {
		name string
		w, h uint32
	}{
		{"wide", 5000, 10},
		{"tall", 10, 5000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := pngHeaderOnly(tc.w, tc.h)
			cfg, _, err := image.DecodeConfig(bytes.NewReader(input))
			if err != nil || cfg.Width != int(tc.w) || cfg.Height != int(tc.h) {
				t.Fatalf("fixture header not readable as %dx%d: cfg=%+v err=%v", tc.w, tc.h, cfg, err)
			}
			_, err = (&ImageParser{}).Parse(input)
			if err == nil {
				t.Fatal("Parse() returned no error for an image over the size cap")
			}
			if !strings.HasPrefix(err.Error(), "image: ") || !strings.Contains(err.Error(), "exceeds the limit of 4000 px") {
				t.Errorf("Parse() error = %q, want the image size cap error", err)
			}
		})
	}
}

// TestImageParser_Parse_DimensionBoundary pins the cap at exactly 4000 px per side.
func TestImageParser_Parse_DimensionBoundary(t *testing.T) {
	cases := []struct {
		name    string
		w, h    uint32
		wantErr bool
	}{
		{"at limit", 4000, 4000, false},
		{"one wider", 4001, 1, true},
		{"one taller", 1, 4001, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := (&ImageParser{}).Parse(pngHeaderOnly(tc.w, tc.h))
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "exceeds the limit of 4000 px") {
					t.Fatalf("Parse() error = %v, want the image size cap error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			if len(doc.Children) != 1 || doc.Children[0].NaturalWidth != float64(tc.w) || doc.Children[0].NaturalHeight != float64(tc.h) {
				t.Fatalf("expected one %dx%d image node, got %+v", tc.w, tc.h, doc.Children)
			}
		})
	}
}

func TestImageParser_Parse_NormalImage(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 10, 10))); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	doc, err := (&ImageParser{}).Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(doc.Children) != 1 || doc.Children[0].NaturalWidth != 10 || doc.Children[0].NaturalHeight != 10 {
		t.Fatalf("expected one 10x10 image node, got %+v", doc.Children)
	}
}
