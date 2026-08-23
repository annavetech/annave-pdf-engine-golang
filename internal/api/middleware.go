// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request_id"

const maxRequestSize = 6 * 1024 * 1024 // 6 MB — slightly above the engine's 5 MB limit

// WithRequestID injects a unique request identifier into the context and
// response header. All subsequent log lines for this request include the ID.
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WithLogging records the method, path, duration, and status code of every
// request using structured slog output.
func WithLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lw, r)

		reqID, _ := r.Context().Value(requestIDKey).(string)
		slog.Info("http.request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", lw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", reqID,
		)
	})
}

// WithCORS adds the headers required for cross-origin browser requests.
// allowedOrigin is typically "*" for development or a specific domain for production.
func WithCORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Request-Id")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WithSizeLimit caps the request body to protect against oversized uploads.
func WithSizeLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
		next.ServeHTTP(w, r)
	})
}

// WithSecurityHeaders adds standard defensive response headers.
func WithSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// WithInternalToken enforces bearer-token authentication when the
// ANNAVE_INTERNAL_TOKEN environment variable is set. If the variable is
// empty or unset, the middleware is a no-op (suitable for local development).
//
// Production deployments must set ANNAVE_INTERNAL_TOKEN to a strong random
// secret in the hosting environment (e.g. Vercel's environment variable UI).
// The secret must never appear in source code or be committed to version control.
//
// Callers supply the token in the X-Internal-Token request header.
func WithInternalToken(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secret == "" {
				// Dev mode — token enforcement is off.
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get("X-Internal-Token") != secret {
				http.Error(w, `{"error":{"code":"ENGINE_ERR_UNAUTHORIZED","stage":"input","message":"Missing or invalid internal token."}}`,
					http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rateEntry holds the recent request timestamps for one client IP.
type rateEntry struct {
	mu         sync.Mutex
	timestamps []time.Time
}

// rateLimiter holds the per-IP state for WithRateLimit and prunes entries
// that have gone idle past the rate-limit window, so that traffic from many
// distinct or rotated addresses cannot grow the map without bound.
type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*rateEntry
}

// sweep deletes entries whose newest timestamp has fallen outside the
// one-minute rate-limit window, or that have no timestamps at all.
//
// Lock ordering must match the request path exactly: the outer mu is taken
// before touching clients, and each entry's own mu only after that. Taking
// them in the reverse order here would deadlock against a request in flight.
func (rl *rateLimiter) sweep() {
	cutoff := time.Now().Add(-time.Minute)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, e := range rl.clients {
		e.mu.Lock()
		stale := len(e.timestamps) == 0 || e.timestamps[len(e.timestamps)-1].Before(cutoff)
		e.mu.Unlock()
		if stale {
			delete(rl.clients, ip)
		}
	}
}

// WithRateLimit enforces a per-IP sliding-window rate limit.
// n is the maximum number of requests allowed per minute per IP.
// If n is 0 or negative, the middleware is a no-op.
func WithRateLimit(n int) func(http.Handler) http.Handler {
	if n <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	rl := &rateLimiter{clients: map[string]*rateEntry{}}
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for range t.C {
			rl.sweep()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}

			rl.mu.Lock()
			e, ok := rl.clients[ip]
			if !ok {
				e = &rateEntry{}
				rl.clients[ip] = e
			}
			rl.mu.Unlock()

			now := time.Now()
			window := now.Add(-time.Minute)

			e.mu.Lock()
			// Drop timestamps older than 1 minute.
			trimmed := e.timestamps[:0]
			for _, t := range e.timestamps {
				if t.After(window) {
					trimmed = append(trimmed, t)
				}
			}
			e.timestamps = trimmed

			if len(e.timestamps) >= n {
				e.mu.Unlock()
				w.Header().Set("Retry-After", "60")
				http.Error(w,
					`{"error":{"code":"ENGINE_ERR_RATE_LIMITED","stage":"input","message":"Too many requests. Retry after 60 seconds."}}`,
					http.StatusTooManyRequests)
				return
			}
			e.timestamps = append(e.timestamps, now)
			e.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lw *loggingResponseWriter) WriteHeader(status int) {
	lw.status = status
	lw.ResponseWriter.WriteHeader(status)
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
