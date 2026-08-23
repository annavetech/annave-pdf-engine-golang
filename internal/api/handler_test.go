// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/annavetech/annave-pdf-engine-golang/internal/engine"
)

func newTestHandler() *Handler {
	return NewHandler("", "*", 0)
}

func withRequestContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), requestIDKey, "test-request-id")
	return r.WithContext(ctx)
}

func assertPDF(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q, want application/pdf", ct)
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-")) {
		t.Errorf("body does not start with a PDF header")
	}
}

func TestConvert_MultipartFile(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "input.md")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = part.Write([]byte("# Title\n\nSome body text."))
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/convert", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req = withRequestContext(req)
	rec := httptest.NewRecorder()

	newTestHandler().convert(rec, req)

	assertPDF(t, rec)
}

func TestConvert_MultipartTextField(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("text", "# Title\n\nSome body text."); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/convert", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req = withRequestContext(req)
	rec := httptest.NewRecorder()

	newTestHandler().convert(rec, req)

	assertPDF(t, rec)
}

func TestConvert_URLEncodedForm(t *testing.T) {
	form := url.Values{"text": {"# Title\n\nSome body text."}}
	req := httptest.NewRequest(http.MethodPost, "/convert", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withRequestContext(req)
	rec := httptest.NewRecorder()

	newTestHandler().convert(rec, req)

	assertPDF(t, rec)
}

func TestConvert_RawBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/convert", strings.NewReader("# Title\n\nSome body text."))
	req.Header.Set("Content-Type", "text/plain")
	req = withRequestContext(req)
	rec := httptest.NewRecorder()

	newTestHandler().convert(rec, req)

	assertPDF(t, rec)
}

func TestConvert_EmptyInputReturns400(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/convert", strings.NewReader(""))
	req.Header.Set("Content-Type", "text/plain")
	req = withRequestContext(req)
	rec := httptest.NewRecorder()

	newTestHandler().convert(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body.Error.Code != "ENGINE_ERR_EMPTY_INPUT" {
		t.Errorf("error code = %q, want ENGINE_ERR_EMPTY_INPUT", body.Error.Code)
	}
}

func TestConvert_UnknownFormatFallsBackToAuto(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/convert?format=bogus", strings.NewReader("# Title\n\nSome body text."))
	req.Header.Set("Content-Type", "text/plain")
	req = withRequestContext(req)
	rec := httptest.NewRecorder()

	newTestHandler().convert(rec, req)

	assertPDF(t, rec)
}

func TestConvert_StyleQueryApplied(t *testing.T) {
	style := url.QueryEscape(`{"paragraph":{"fontSize":18}}`)
	req := httptest.NewRequest(http.MethodPost, "/convert?style="+style, strings.NewReader("# Title\n\nSome body text."))
	req.Header.Set("Content-Type", "text/plain")
	req = withRequestContext(req)
	rec := httptest.NewRecorder()

	newTestHandler().convert(rec, req)

	assertPDF(t, rec)
}

func TestConvert_MalformedStyleJSONIsIgnoredNotFatal(t *testing.T) {
	style := url.QueryEscape(`{not valid json`)
	req := httptest.NewRequest(http.MethodPost, "/convert?style="+style, strings.NewReader("# Title\n\nSome body text."))
	req.Header.Set("Content-Type", "text/plain")
	req = withRequestContext(req)
	rec := httptest.NewRecorder()

	newTestHandler().convert(rec, req)

	assertPDF(t, rec)
}

// TestConvert_AnnaveErrorStageMapsToStatus locks the mapping from an engine
// pipeline stage to the HTTP status returned by convert: input errors are
// client errors (400), pagination overflow is unprocessable content (422).
func TestConvert_AnnaveErrorStageMapsToStatus(t *testing.T) {
	t.Run("input stage maps to 400", func(t *testing.T) {
		huge := strings.Repeat("x ", 3*1024*1024)
		req := httptest.NewRequest(http.MethodPost, "/convert", strings.NewReader(huge))
		req.Header.Set("Content-Type", "text/plain")
		req = withRequestContext(req)
		rec := httptest.NewRecorder()

		newTestHandler().convert(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
		var body errorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("response is not valid JSON: %v", err)
		}
		if body.Error.Stage != engine.StageInput {
			t.Errorf("stage = %q, want %q", body.Error.Stage, engine.StageInput)
		}
	})

	t.Run("pagination stage maps to 422", func(t *testing.T) {
		// Enough wrapped paragraphs to push layout past the configured
		// max_pages (100) while staying under max_nodes (2000) and
		// max_input_chars (500000). The page-count check runs before the
		// renderer is constructed, so this stays fast.
		sentence := strings.Repeat("word ", 80)
		var sb strings.Builder
		for i := 0; i < 1000; i++ {
			sb.WriteString(sentence)
			sb.WriteString("\n\n")
		}

		req := httptest.NewRequest(http.MethodPost, "/convert?format=md", strings.NewReader(sb.String()))
		req.Header.Set("Content-Type", "text/plain")
		req = withRequestContext(req)
		rec := httptest.NewRecorder()

		newTestHandler().convert(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		var body errorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("response is not valid JSON: %v", err)
		}
		if body.Error.Stage != engine.StagePagination {
			t.Errorf("stage = %q, want %q", body.Error.Stage, engine.StagePagination)
		}
	})
}

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	newTestHandler().health(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`status field = %q, want "ok"`, body["status"])
	}
}

func TestUI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	newTestHandler().ui(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected non-empty HTML body")
	}
}

func TestSchemaRoutes(t *testing.T) {
	mux := http.NewServeMux()
	newTestHandler().Register(mux)

	cases := []struct {
		path string
	}{
		{"/schema/error.v1"},
		{"/schema/document.v1"},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/schema+json" {
				t.Errorf("Content-Type = %q, want application/schema+json", ct)
			}
			if rec.Body.Len() == 0 {
				t.Error("expected non-empty schema body")
			}
		})
	}
}
