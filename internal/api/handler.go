// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/annavetech/annave-pdf-engine-golang/internal/engine"
	"github.com/annavetech/annave-pdf-engine-golang/internal/parser"
	engineschema "github.com/annavetech/annave-pdf-engine-golang/schema"
	"github.com/annavetech/annave-pdf-engine-golang/ui"
)

type Handler struct {
	pipeline   *engine.Pipeline
	secret     string
	corsOrigin string
	rateLimit  int
}

func NewHandler(internalToken, corsOrigin string, rateLimit int) *Handler {
	return &Handler{
		pipeline:   engine.NewPipeline(),
		secret:     internalToken,
		corsOrigin: corsOrigin,
		rateLimit:  rateLimit,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	cors := WithCORS(h.corsOrigin)
	rl := WithRateLimit(h.rateLimit)
	base := func(next http.Handler) http.Handler {
		return WithRequestID(WithLogging(WithSecurityHeaders(cors(next))))
	}
	protected := func(next http.Handler) http.Handler {
		return base(WithInternalToken(h.secret)(rl(WithSizeLimit(next))))
	}
	mux.Handle("POST /convert", protected(http.HandlerFunc(h.convert)))
	mux.Handle("GET /health", base(http.HandlerFunc(h.health)))
	mux.Handle("GET /", base(http.HandlerFunc(h.ui)))
	mux.Handle("GET /schema/error.v1", base(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/schema+json")
		_, _ = w.Write(engineschema.ErrorV1)
	})))
	mux.Handle("GET /schema/document.v1", base(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/schema+json")
		_, _ = w.Write(engineschema.DocumentV1)
	})))
	mux.Handle("OPTIONS /convert", base(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
}

// POST /convert — accepts multipart form or raw body + query param ?format=
func (h *Handler) convert(w http.ResponseWriter, r *http.Request) {
	formatStr := strings.ToLower(r.URL.Query().Get("format"))
	format := parser.InputFormat(formatStr)
	if format == "" {
		format = parser.FormatAuto
	}

	var input string

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(6 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "INPUT_ERROR", engine.StageInput, "Failed to parse multipart form.")
			return
		}
		// File upload takes priority
		file, header, err := r.FormFile("file")
		if err == nil {
			defer func() { _ = file.Close() }()
			if format == parser.FormatAuto {
				format = parser.FormatFromExtension(header.Filename)
			}
			data, err := io.ReadAll(file)
			if err != nil {
				writeError(w, http.StatusBadRequest, "READ_ERROR", engine.StageInput, "Failed to read uploaded file.")
				return
			}
			input = string(data)
		} else {
			// Text field
			input = r.FormValue("text")
			if f := r.FormValue("format"); f != "" && format == parser.FormatAuto {
				format = parser.InputFormat(strings.ToLower(f))
			}
		}
	} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			writeError(w, http.StatusBadRequest, "INPUT_ERROR", engine.StageInput, "Failed to parse form.")
			return
		}
		input = r.FormValue("text")
		if f := r.FormValue("format"); f != "" && format == parser.FormatAuto {
			format = parser.InputFormat(strings.ToLower(f))
		}
	} else {
		// Raw body
		data, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "READ_ERROR", engine.StageInput, "Failed to read request body.")
			return
		}
		input = string(data)
	}

	if strings.TrimSpace(input) == "" {
		writeError(w, http.StatusBadRequest, "ENGINE_ERR_EMPTY_INPUT", engine.StageInput, "No input provided.")
		return
	}

	// Optional per-request style override: ?style={"paragraph":{"fontSize":14}}
	// Also accepted as a multipart/form field or URL-encoded field named "style".
	var runOpts []engine.RunOption
	styleJSON := r.URL.Query().Get("style")
	if styleJSON == "" {
		styleJSON = r.FormValue("style")
	}
	if styleJSON != "" {
		var override engine.StyleOverride
		if err := json.Unmarshal([]byte(styleJSON), &override); err == nil {
			runOpts = append(runOpts, engine.WithStyleOverride(&override))
		}
	}

	reqID, _ := r.Context().Value(requestIDKey).(string)

	pdfBytes, err := h.pipeline.Run(input, format, runOpts...)
	if err != nil {
		status := http.StatusUnprocessableEntity
		if ae, ok := err.(*engine.AnnaveError); ok {
			switch ae.Stage {
			case engine.StageInput:
				status = http.StatusBadRequest
			case engine.StagePagination:
				status = http.StatusUnprocessableEntity
			}
		}
		var ae *engine.AnnaveError
		if ok := errorAs(err, &ae); ok {
			slog.Error("convert.failed",
				"code", ae.Code, "stage", ae.Stage,
				"request_id", reqID, "format", format)
			writeError(w, status, ae.Code, ae.Stage, ae.Message)
		} else {
			slog.Error("convert.internal",
				"error", err.Error(),
				"request_id", reqID)
			writeError(w, http.StatusInternalServerError, "ENGINE_ERR_INTERNAL", engine.StageRender, err.Error())
		}
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=\"output.pdf\"")
	w.Header().Set("X-Engine-Version", engine.EngineVersion)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

func (h *Handler) ui(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(ui.HTML)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": engine.EngineVersion,
	})
}

type errorBody struct {
	Error struct {
		Code    string             `json:"code"`
		Stage   engine.EngineStage `json:"stage"`
		Message string             `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code string, stage engine.EngineStage, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := errorBody{}
	body.Error.Code = code
	body.Error.Stage = stage
	body.Error.Message = message
	_ = json.NewEncoder(w).Encode(body)
}

func errorAs(err error, target **engine.AnnaveError) bool {
	if ae, ok := err.(*engine.AnnaveError); ok {
		*target = ae
		return true
	}
	return false
}
