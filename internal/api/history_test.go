package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/saaranganand/ledsure/internal/ledger"
	"github.com/saaranganand/ledsure/internal/store"
)

func TestGetBalance(t *testing.T) {
	fs := newFakeStore()
	fs.balances[aliceID] = store.Balance{AccountID: aliceID, Currency: ledger.CAD, Amount: 700, AsOfSeq: 2}
	h := newTestRouter(fs)

	rec := do(t, h, http.MethodGet, "/v1/accounts/"+aliceID.String()+"/balance", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	want := `{"account_id":"00000000-0000-7000-8000-0000000000c1","balance":700,"currency":"CAD","as_of_seq":2}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}

	rec = do(t, h, http.MethodGet, "/v1/accounts/not-a-uuid/balance", "")
	assertError(t, rec.Code, rec.Body.Bytes(), http.StatusBadRequest, "invalid_request")

	rec = do(t, h, http.MethodGet, "/v1/accounts/"+bobID.String()+"/balance", "")
	assertError(t, rec.Code, rec.Body.Bytes(), http.StatusNotFound, "not_found")
}

func TestListEntries(t *testing.T) {
	fs := newFakeStore()
	fs.entries[aliceID] = []store.AccountEntry{
		{EntryID: testID, TransactionID: testID, Seq: 3, Amount: 1000, Description: "deposit", PostedAt: testNow},
		{EntryID: testID, TransactionID: testID, Seq: 5, Amount: -300, Description: "rent", PostedAt: testNow},
	}
	fs.entries[bobID] = []store.AccountEntry{}
	h := newTestRouter(fs)

	rec := do(t, h, http.MethodGet, "/v1/accounts/"+aliceID.String()+"/entries", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if fs.lastAfterSeq != 0 || fs.lastLimit != defaultEntriesLimit {
		t.Errorf("store called with after_seq=%d limit=%d, want 0 and %d", fs.lastAfterSeq, fs.lastLimit, defaultEntriesLimit)
	}
	if !strings.Contains(rec.Body.String(), `"next_after_seq":5`) {
		t.Errorf("body %s should carry the last seq as next_after_seq", rec.Body)
	}

	do(t, h, http.MethodGet, "/v1/accounts/"+aliceID.String()+"/entries?after_seq=3&limit=10", "")
	if fs.lastAfterSeq != 3 || fs.lastLimit != 10 {
		t.Errorf("store called with after_seq=%d limit=%d, want 3 and 10", fs.lastAfterSeq, fs.lastLimit)
	}

	rec = do(t, h, http.MethodGet, "/v1/accounts/"+bobID.String()+"/entries?after_seq=9", "")
	if got := strings.TrimSpace(rec.Body.String()); got != `{"entries":[],"next_after_seq":9}` {
		t.Errorf("empty page body = %s", got)
	}

	for _, query := range []string{"limit=0", "limit=201", "limit=abc", "after_seq=-1", "after_seq=1.5"} {
		rec = do(t, h, http.MethodGet, "/v1/accounts/"+aliceID.String()+"/entries?"+query, "")
		assertError(t, rec.Code, rec.Body.Bytes(), http.StatusBadRequest, "invalid_request")
	}

	rec = do(t, h, http.MethodGet, "/v1/accounts/00000000-0000-7000-8000-0000000000ff/entries", "")
	assertError(t, rec.Code, rec.Body.Bytes(), http.StatusNotFound, "not_found")
}
