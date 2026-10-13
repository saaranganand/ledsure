package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/saaranganand/ledsure/internal/ledger"
)

func TestCreateAccount(t *testing.T) {
	fs := newFakeStore()
	rec := do(t, newTestRouter(fs), http.MethodPost, "/v1/accounts", `{"name":"alice","currency":"CAD"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body)
	}

	want := `{"id":"00000000-0000-7000-8000-0000000000aa","name":"alice","currency":"CAD","kind":"customer","allow_negative":false,"created_at":"2026-01-02T03:04:05Z"}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}

	if _, ok := fs.accounts[testID]; !ok {
		t.Error("account was not saved to the store")
	}
}

func TestCreateAccountRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"system kind", `{"name":"x","currency":"CAD","kind":"system"}`},
		{"allow negative", `{"name":"x","currency":"CAD","allow_negative":true}`},
		{"unsupported currency", `{"name":"x","currency":"EUR"}`},
		{"blank name", `{"name":"   ","currency":"CAD"}`},
		{"missing fields", `{}`},
		{"malformed json", `nope`},
		{"two objects", `{"name":"x","currency":"CAD"} {}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFakeStore()
			rec := do(t, newTestRouter(fs), http.MethodPost, "/v1/accounts", tc.body)

			assertError(t, rec.Code, rec.Body.Bytes(), http.StatusBadRequest, "invalid_request")
			if len(fs.accounts) != 0 {
				t.Error("rejected request still saved an account")
			}
		})
	}
}

func TestGetAccount(t *testing.T) {
	fs := newFakeStore()
	account, err := ledger.NewCustomerAccount(testID, testNow, "alice", ledger.CAD)
	if err != nil {
		t.Fatalf("NewCustomerAccount: %v", err)
	}
	fs.accounts[testID] = account
	h := newTestRouter(fs)

	rec := do(t, h, http.MethodGet, "/v1/accounts/"+testID.String(), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	var got accountResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got != toAccountResponse(account) {
		t.Errorf("got %+v, want %+v", got, toAccountResponse(account))
	}

	rec = do(t, h, http.MethodGet, "/v1/accounts/not-a-uuid", "")
	assertError(t, rec.Code, rec.Body.Bytes(), http.StatusBadRequest, "invalid_request")

	rec = do(t, h, http.MethodGet, "/v1/accounts/00000000-0000-7000-8000-0000000000ff", "")
	assertError(t, rec.Code, rec.Body.Bytes(), http.StatusNotFound, "not_found")
}

func TestListSystemAccountsEmptyIsArray(t *testing.T) {
	rec := do(t, newTestRouter(newFakeStore()), http.MethodGet, "/v1/system-accounts", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("body = %s, want []", got)
	}
}

func TestStoreFailureHidesDetails(t *testing.T) {
	fs := newFakeStore()
	fs.err = errors.New("connection refused to 10.0.0.5")

	rec := do(t, newTestRouter(fs), http.MethodGet, "/v1/system-accounts", "")

	assertError(t, rec.Code, rec.Body.Bytes(), http.StatusInternalServerError, "internal_error")
	if strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Errorf("response leaked internal error: %s", rec.Body)
	}
}

func assertError(t *testing.T, status int, body []byte, wantStatus int, wantCode string) {
	t.Helper()
	if status != wantStatus {
		t.Errorf("status = %d, want %d; body %s", status, wantStatus, body)
	}
	var resp errorResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode error body %q: %v", body, err)
	}
	if resp.Error.Code != wantCode {
		t.Errorf("error code = %q, want %q", resp.Error.Code, wantCode)
	}
}
