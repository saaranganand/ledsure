package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/saaranganand/ledsure/internal/ledger"
)

var (
	aliceID = uuid.MustParse("00000000-0000-7000-8000-0000000000c1")
	bobID   = uuid.MustParse("00000000-0000-7000-8000-0000000000c2")
)

func TestPostTransaction(t *testing.T) {
	fs := newFakeStore()
	body := `{"description":"rent","entries":[` +
		`{"account_id":"00000000-0000-7000-8000-0000000000c1","amount":-100},` +
		`{"account_id":"00000000-0000-7000-8000-0000000000c2","amount":100}]}`

	rec := do(t, newTestRouter(fs), http.MethodPost, "/v1/transactions", body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body)
	}
	want := `{"id":"00000000-0000-7000-8000-0000000000aa","seq":1,"description":"rent","posted_at":"2026-01-02T03:04:05Z","entries":[` +
		`{"id":"00000000-0000-7000-8000-0000000000ab","account_id":"00000000-0000-7000-8000-0000000000c1","amount":-100},` +
		`{"id":"00000000-0000-7000-8000-0000000000ac","account_id":"00000000-0000-7000-8000-0000000000c2","amount":100}]}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}
	if len(fs.posted) != 1 {
		t.Errorf("posted %d transactions, want 1", len(fs.posted))
	}
}

func TestPostTransactionRejects(t *testing.T) {
	const (
		alice = `"00000000-0000-7000-8000-0000000000c1"`
		bob   = `"00000000-0000-7000-8000-0000000000c2"`
	)
	entries := func(a, b string) string {
		return `{"entries":[{"account_id":` + alice + `,"amount":` + a + `},{"account_id":` + bob + `,"amount":` + b + `}]}`
	}

	tests := []struct {
		name       string
		body       string
		storeErr   error
		wantStatus int
		wantCode   string
	}{
		{"unknown field", `{"entries":[],"fee":1}`, nil, http.StatusBadRequest, "invalid_request"},
		{"fractional amount", entries("-10.5", "10.5"), nil, http.StatusBadRequest, "invalid_request"},
		{"amount as string", entries(`"-10"`, `"10"`), nil, http.StatusBadRequest, "invalid_request"},
		{"bad account id", `{"entries":[{"account_id":"nope","amount":1}]}`, nil, http.StatusBadRequest, "invalid_request"},
		{"one entry", `{"entries":[{"account_id":` + alice + `,"amount":1}]}`, nil, http.StatusBadRequest, "invalid_request"},
		{"zero amounts", entries("0", "0"), nil, http.StatusBadRequest, "invalid_request"},
		{"unbalanced", entries("-100", "99"), nil, http.StatusUnprocessableEntity, "unbalanced_transaction"},
		{"insufficient funds", entries("-100", "100"), fmt.Errorf("%w: short", ledger.ErrInsufficientFunds), http.StatusUnprocessableEntity, "insufficient_funds"},
		{"currency mismatch", entries("-100", "100"), fmt.Errorf("%w: CAD and USD", ledger.ErrCurrencyMismatch), http.StatusUnprocessableEntity, "currency_mismatch"},
		{"unknown account", entries("-100", "100"), fmt.Errorf("%w: x", ledger.ErrAccountNotFound), http.StatusNotFound, "not_found"},
		{"store failure", entries("-100", "100"), errors.New("connection reset"), http.StatusInternalServerError, "internal_error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFakeStore()
			fs.err = tc.storeErr

			rec := do(t, newTestRouter(fs), http.MethodPost, "/v1/transactions", tc.body)

			assertError(t, rec.Code, rec.Body.Bytes(), tc.wantStatus, tc.wantCode)
			if len(fs.posted) != 0 {
				t.Error("rejected request still posted a transaction")
			}
		})
	}
}

func TestGetTransaction(t *testing.T) {
	fs := newFakeStore()
	tx := ledger.Transaction{
		ID:          testID,
		Seq:         7,
		Description: "rent",
		PostedAt:    testNow,
		Entries: []ledger.Entry{
			{ID: uuid.MustParse("00000000-0000-7000-8000-0000000000e1"), AccountID: aliceID, Amount: -100},
			{ID: uuid.MustParse("00000000-0000-7000-8000-0000000000e2"), AccountID: bobID, Amount: 100},
		},
	}
	fs.posted[tx.ID] = tx
	h := newTestRouter(fs)

	rec := do(t, h, http.MethodGet, "/v1/transactions/"+tx.ID.String(), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	want := `{"id":"00000000-0000-7000-8000-0000000000aa","seq":7,"description":"rent","posted_at":"2026-01-02T03:04:05Z","entries":[` +
		`{"id":"00000000-0000-7000-8000-0000000000e1","account_id":"00000000-0000-7000-8000-0000000000c1","amount":-100},` +
		`{"id":"00000000-0000-7000-8000-0000000000e2","account_id":"00000000-0000-7000-8000-0000000000c2","amount":100}]}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}

	rec = do(t, h, http.MethodGet, "/v1/transactions/not-a-uuid", "")
	assertError(t, rec.Code, rec.Body.Bytes(), http.StatusBadRequest, "invalid_request")

	rec = do(t, h, http.MethodGet, "/v1/transactions/00000000-0000-7000-8000-0000000000ff", "")
	assertError(t, rec.Code, rec.Body.Bytes(), http.StatusNotFound, "not_found")
}
