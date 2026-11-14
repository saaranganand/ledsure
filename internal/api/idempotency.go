package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/saaranganand/ledsure/internal/store"
)

const maxIdempotencyKeyLength = 255

// wraps a POST handler so a retried request replays its first response instead of running again
func (s *server) idempotent(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			writeError(w, http.StatusBadRequest, "missing_idempotency_key", "the Idempotency-Key header is required")
			return
		}
		if utf8.RuneCountInString(key) > maxIdempotencyKeyLength {
			writeError(w, http.StatusBadRequest, "invalid_request",
				fmt.Sprintf("Idempotency-Key must be at most %d characters", maxIdempotencyKeyLength))
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body is unreadable or too large")
			return
		}

		resp, err := s.idempotency.Idempotent(r.Context(), key, requestHash(r.Method, r.URL.Path, body), s.clock.Now(),
			func(ctx context.Context) store.IdempotentResponse {
				rec := &captureWriter{header: http.Header{}, status: http.StatusOK}
				req := r.WithContext(ctx)
				req.Body = io.NopCloser(bytes.NewReader(body))
				next(rec, req)
				return store.IdempotentResponse{Status: rec.status, Body: rec.body.Bytes()}
			},
		)
		if errors.Is(err, store.ErrKeyReused) {
			writeError(w, http.StatusConflict, "idempotency_key_reused", "this Idempotency-Key was already used for a different request")
			return
		}
		if err != nil {
			s.logger.Error("api: idempotent request failed", "method", r.Method, "path", r.URL.Path, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(resp.Status)
		if _, err := w.Write(resp.Body); err != nil {
			s.logger.Error("api: write response", "error", err)
		}
	}
}

// same json with different spacing or key order hashes the same, so a reformatted retry still replays
func requestHash(method, path string, body []byte) []byte {
	h := sha256.New()
	h.Write([]byte(method + "\n" + path + "\n"))
	h.Write(canonicalJSON(body))
	return h.Sum(nil)
}

// bodies that aren't a single json value are hashed as sent
func canonicalJSON(body []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil || dec.More() {
		return body
	}
	out, err := json.Marshal(v)
	if err != nil {
		return body
	}
	return out
}

// buffers a handler's response so nothing reaches the client before it has been committed
type captureWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (c *captureWriter) Header() http.Header         { return c.header }
func (c *captureWriter) WriteHeader(status int)      { c.status = status }
func (c *captureWriter) Write(b []byte) (int, error) { return c.body.Write(b) }
