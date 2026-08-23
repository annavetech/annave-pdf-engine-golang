// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestWithRequestID(t *testing.T) {
	var ctxID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID, _ = r.Context().Value(requestIDKey).(string)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	WithRequestID(next).ServeHTTP(rec, req)

	header := rec.Header().Get("X-Request-Id")
	if header == "" {
		t.Fatal("expected X-Request-Id header to be set")
	}
	if header != ctxID {
		t.Errorf("header %q does not match context value %q", header, ctxID)
	}
}

// TestWithLogging captures the default slog output to verify the status code
// is recorded correctly, including the case where the handler never calls
// WriteHeader explicitly (implicit 200).
func TestWithLogging(t *testing.T) {
	cases := []struct {
		name       string
		next       http.Handler
		wantStatus int
	}{
		{
			name:       "implicit 200",
			next:       http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			wantStatus: http.StatusOK,
		},
		{
			name: "explicit 404",
			next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}),
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			defer slog.SetDefault(prev)

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			rec := httptest.NewRecorder()
			WithLogging(tc.next).ServeHTTP(rec, req)

			var line map[string]any
			if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
				t.Fatalf("log line is not valid JSON: %v (%q)", err, buf.String())
			}
			status, ok := line["status"].(float64)
			if !ok {
				t.Fatalf("log line missing numeric status field: %v", line)
			}
			if int(status) != tc.wantStatus {
				t.Errorf("logged status = %d, want %d", int(status), tc.wantStatus)
			}
		})
	}
}

func TestWithCORS_HeadersPresent(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodPost, "/convert", nil)
	rec := httptest.NewRecorder()
	WithCORS("https://example.com")(next).ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Access-Control-Allow-Methods not set")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("Access-Control-Allow-Headers not set")
	}
	if !called {
		t.Error("expected next handler to be called for POST")
	}
}

func TestWithCORS_OptionsShortCircuits(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodOptions, "/convert", nil)
	rec := httptest.NewRecorder()
	WithCORS("*")(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if called {
		t.Error("next handler should not be called for OPTIONS")
	}
}

func TestWithSizeLimit_UnderLimitPasses(t *testing.T) {
	var readErr error
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	})

	body := strings.Repeat("a", maxRequestSize-1)
	req := httptest.NewRequest(http.MethodPost, "/convert", strings.NewReader(body))
	rec := httptest.NewRecorder()
	WithSizeLimit(next).ServeHTTP(rec, req)

	if readErr != nil {
		t.Errorf("unexpected read error under the limit: %v", readErr)
	}
}

func TestWithSizeLimit_OverLimitFails(t *testing.T) {
	var readErr error
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	})

	body := strings.Repeat("a", maxRequestSize+1)
	req := httptest.NewRequest(http.MethodPost, "/convert", strings.NewReader(body))
	rec := httptest.NewRecorder()
	WithSizeLimit(next).ServeHTTP(rec, req)

	if readErr == nil {
		t.Error("expected a read error for a body over maxRequestSize")
	}
}

func TestWithSecurityHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	WithSecurityHeaders(okHandler()).ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
}

func TestWithInternalToken_EmptySecretIsNoOp(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodPost, "/convert", nil)
	rec := httptest.NewRecorder()
	WithInternalToken("")(next).ServeHTTP(rec, req)

	if !called {
		t.Error("expected next handler to be called when secret is empty")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestWithInternalToken_CorrectTokenPasses(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodPost, "/convert", nil)
	req.Header.Set("X-Internal-Token", "secret123")
	rec := httptest.NewRecorder()
	WithInternalToken("secret123")(next).ServeHTTP(rec, req)

	if !called {
		t.Error("expected next handler to be called with the correct token")
	}
}

func TestWithInternalToken_WrongTokenRejected(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodPost, "/convert", nil)
	req.Header.Set("X-Internal-Token", "wrong")
	rec := httptest.NewRecorder()
	WithInternalToken("secret123")(next).ServeHTTP(rec, req)

	if called {
		t.Error("next handler should not be called with a wrong token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestWithInternalToken_MissingHeaderRejected(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodPost, "/convert", nil)
	rec := httptest.NewRecorder()
	WithInternalToken("secret123")(next).ServeHTTP(rec, req)

	if called {
		t.Error("next handler should not be called when the header is missing")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func rateLimitedRequest(ip string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/convert", nil)
	req.RemoteAddr = ip
	return req
}

func TestWithRateLimit_UnderLimitPasses(t *testing.T) {
	mw := WithRateLimit(3)(okHandler())

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, rateLimitedRequest("198.51.100.1:1000"))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i, rec.Code, http.StatusOK)
		}
	}
}

func TestWithRateLimit_AtLimitReturns429(t *testing.T) {
	mw := WithRateLimit(3)(okHandler())
	ip := "198.51.100.2:1000"

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, rateLimitedRequest(ip))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i, rec.Code, http.StatusOK)
		}
	}

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, rateLimitedRequest(ip))
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if got := rec.Header().Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After = %q, want %q", got, "60")
	}
}

func TestWithRateLimit_DifferentIPUnaffected(t *testing.T) {
	mw := WithRateLimit(1)(okHandler())

	rec1 := httptest.NewRecorder()
	mw.ServeHTTP(rec1, rateLimitedRequest("198.51.100.3:1000"))
	if rec1.Code != http.StatusOK {
		t.Fatalf("first client status = %d, want %d", rec1.Code, http.StatusOK)
	}

	rec1b := httptest.NewRecorder()
	mw.ServeHTTP(rec1b, rateLimitedRequest("198.51.100.3:1000"))
	if rec1b.Code != http.StatusTooManyRequests {
		t.Fatalf("first client second request status = %d, want %d", rec1b.Code, http.StatusTooManyRequests)
	}

	rec2 := httptest.NewRecorder()
	mw.ServeHTTP(rec2, rateLimitedRequest("198.51.100.4:1000"))
	if rec2.Code != http.StatusOK {
		t.Errorf("second client status = %d, want %d", rec2.Code, http.StatusOK)
	}
}

func TestWithRateLimit_ZeroOrNegativeIsNoOp(t *testing.T) {
	for _, n := range []int{0, -1} {
		mw := WithRateLimit(n)(okHandler())
		ip := "198.51.100.5:1000"
		for i := 0; i < 50; i++ {
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, rateLimitedRequest(ip))
			if rec.Code != http.StatusOK {
				t.Fatalf("n=%d request %d: status = %d, want %d", n, i, rec.Code, http.StatusOK)
			}
		}
	}
}

// TestWithRateLimit_WindowExpiryRestoresAllowance proves that timestamps older
// than the one-minute sliding window are dropped, freeing up the allowance.
// The window is not configurable, so this test genuinely waits past it —
// there is no clock injected into WithRateLimit to fake elapsed time.
func TestWithRateLimit_WindowExpiryRestoresAllowance(t *testing.T) {
	mw := WithRateLimit(1)(okHandler())
	ip := "198.51.100.6:1000"

	rec1 := httptest.NewRecorder()
	mw.ServeHTTP(rec1, rateLimitedRequest(ip))
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want %d", rec1.Code, http.StatusOK)
	}

	rec2 := httptest.NewRecorder()
	mw.ServeHTTP(rec2, rateLimitedRequest(ip))
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}

	time.Sleep(61 * time.Second)

	rec3 := httptest.NewRecorder()
	mw.ServeHTTP(rec3, rateLimitedRequest(ip))
	if rec3.Code != http.StatusOK {
		t.Errorf("request after window expiry: status = %d, want %d", rec3.Code, http.StatusOK)
	}
}

// TestRateLimiter_SweepPrunesStaleEntries registers 10,000 distinct IPs with
// timestamps already outside the one-minute window, calls sweep directly,
// and asserts every entry was removed. This is Fix 1 from the remediation
// spec: without the sweeper, the client map grows without bound.
func TestRateLimiter_SweepPrunesStaleEntries(t *testing.T) {
	rl := &rateLimiter{clients: map[string]*rateEntry{}}
	stale := time.Now().Add(-2 * time.Minute)

	const ips = 10000
	for i := 0; i < ips; i++ {
		ip := fmt.Sprintf("198.51.100.%d:%d", i%256, i)
		rl.clients[ip] = &rateEntry{timestamps: []time.Time{stale}}
	}

	rl.sweep()

	if len(rl.clients) != 0 {
		t.Errorf("clients after sweep = %d, want 0", len(rl.clients))
	}
}

// TestWithRateLimit_Concurrent fires 100 goroutines at a single IP and
// asserts that exactly n succeed, exercising the mutex-guarded entry under
// -race.
func TestWithRateLimit_Concurrent(t *testing.T) {
	const n = 10
	const goroutines = 100

	mw := WithRateLimit(n)(okHandler())
	ip := "198.51.100.7:1000"

	var succeeded int64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, rateLimitedRequest(ip))
			if rec.Code == http.StatusOK {
				atomic.AddInt64(&succeeded, 1)
			}
		}()
	}
	wg.Wait()

	if succeeded != n {
		t.Errorf("succeeded = %d, want %d", succeeded, n)
	}
}
