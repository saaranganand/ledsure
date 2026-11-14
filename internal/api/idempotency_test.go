package api

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

const aliceBody = `{"name":"alice","currency":"CAD"}`

func TestIdempotencyKeyRequired(t *testing.T) {
	fs := newFakeStore()
	h := newTestRouter(fs)

	for _, path := range []string{"/v1/accounts", "/v1/transactions"} {
		rec := doWithKey(t, h, path, "", aliceBody)
		assertError(t, rec.Code, rec.Body.Bytes(), http.StatusBadRequest, "missing_idempotency_key")
	}

	rec := doWithKey(t, h, "/v1/accounts", strings.Repeat("k", maxIdempotencyKeyLength+1), aliceBody)
	assertError(t, rec.Code, rec.Body.Bytes(), http.StatusBadRequest, "invalid_request")

	if len(fs.accounts) != 0 || len(fs.keys) != 0 {
		t.Error("a rejected key still ran the handler or stored a response")
	}
}

func TestRetryReplaysFirstResponse(t *testing.T) {
	fs := newFakeStore()
	h := newTestRouter(fs)

	first := doWithKey(t, h, "/v1/accounts", "k1", aliceBody)
	reformatted := doWithKey(t, h, "/v1/accounts", "k1", "{ \"currency\": \"CAD\",\n  \"name\": \"alice\" }")

	if first.Code != http.StatusCreated || reformatted.Code != http.StatusCreated {
		t.Fatalf("statuses = %d, %d, want 201, 201", first.Code, reformatted.Code)
	}
	if first.Body.String() != reformatted.Body.String() {
		t.Errorf("replay differs:\n%s\n%s", first.Body, reformatted.Body)
	}
	if len(fs.accounts) != 1 {
		t.Errorf("handler ran %d times, want once", len(fs.accounts))
	}
}

func TestKeyReusedForDifferentRequest(t *testing.T) {
	fs := newFakeStore()
	h := newTestRouter(fs)
	doWithKey(t, h, "/v1/accounts", "k1", aliceBody)

	tests := []struct {
		name string
		path string
		body string
	}{
		{"different body", "/v1/accounts", `{"name":"bob","currency":"CAD"}`},
		{"different endpoint", "/v1/transactions", aliceBody},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doWithKey(t, h, tc.path, "k1", tc.body)
			assertError(t, rec.Code, rec.Body.Bytes(), http.StatusConflict, "idempotency_key_reused")
		})
	}
	if len(fs.accounts) != 1 {
		t.Errorf("accounts = %d, want 1", len(fs.accounts))
	}
}

func TestRejectionsAreReplayedButServerErrorsAreNot(t *testing.T) {
	fs := newFakeStore()
	h := newTestRouter(fs)

	bad := doWithKey(t, h, "/v1/accounts", "k-bad", `{"name":"x","currency":"EUR"}`)
	badRetry := doWithKey(t, h, "/v1/accounts", "k-bad", `{"name":"x","currency":"EUR"}`)
	if bad.Code != http.StatusBadRequest || badRetry.Body.String() != bad.Body.String() {
		t.Errorf("400 not replayed: %d %s then %d %s", bad.Code, bad.Body, badRetry.Code, badRetry.Body)
	}

	fs.err = errors.New("database unavailable")
	failed := doWithKey(t, h, "/v1/accounts", "k-500", aliceBody)
	assertError(t, failed.Code, failed.Body.Bytes(), http.StatusInternalServerError, "internal_error")

	fs.err = nil
	retry := doWithKey(t, h, "/v1/accounts", "k-500", aliceBody)
	if retry.Code != http.StatusCreated {
		t.Errorf("retry after a 500 = %d, want 201 (server errors must not be stored)", retry.Code)
	}
}
