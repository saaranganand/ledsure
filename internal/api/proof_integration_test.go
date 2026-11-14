//go:build integration

package api

import (
	"context"
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

	"github.com/saaranganand/ledsure/internal/platform"
	"github.com/saaranganand/ledsure/internal/testdb"
)

const bankCAD = "00000000-0000-7000-8000-000000000001"

func newProofServer(t *testing.T) (*httptest.Server, testdb.DB) {
	t.Helper()
	db := testdb.New(t)
	srv := httptest.NewServer(NewRouter(Deps{
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Accounts:    db.Store,
		Journal:     db.Store,
		Idempotency: db.Store,
		Clock:       platform.SystemClock{},
		IDs:         platform.UUIDv7Generator{},
	}))
	t.Cleanup(srv.Close)
	return srv, db
}

func call(t *testing.T, srv *httptest.Server, method, path, key, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Errorf("build request: %v", err)
		return 0, ""
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Errorf("%s %s: %v", method, path, err)
		return 0, ""
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("read body: %v", err)
	}
	return resp.StatusCode, string(b)
}

func field(t *testing.T, body, name string) any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	return m[name]
}

func errorCode(t *testing.T, body string) string {
	t.Helper()
	e, _ := field(t, body, "error").(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func createAccount(t *testing.T, srv *httptest.Server, name, currency string) string {
	t.Helper()
	status, body := call(t, srv, http.MethodPost, "/v1/accounts", "create-"+name,
		fmt.Sprintf(`{"name":%q,"currency":%q}`, name, currency))
	if status != http.StatusCreated {
		t.Fatalf("create %s: %d %s", name, status, body)
	}
	return field(t, body, "id").(string)
}

func transferBody(from, to string, amount int64) string {
	return fmt.Sprintf(`{"description":"test","entries":[{"account_id":%q,"amount":%d},{"account_id":%q,"amount":%d}]}`,
		from, -amount, to, amount)
}

func balanceOf(t *testing.T, srv *httptest.Server, account string) int64 {
	t.Helper()
	status, body := call(t, srv, http.MethodGet, "/v1/accounts/"+account+"/balance", "", "")
	if status != http.StatusOK {
		t.Errorf("balance %s: %d %s", account, status, body)
		return 0
	}
	return int64(field(t, body, "balance").(float64))
}

func countTransactions(t *testing.T, db testdb.DB) int {
	t.Helper()
	var n int
	if err := db.Pool.QueryRow(context.Background(), `SELECT count(*) FROM transactions`).Scan(&n); err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	return n
}

func TestProofConcurrentRetriesPostOnce(t *testing.T) {
	srv, db := newProofServer(t)
	alice := createAccount(t, srv, "alice", "CAD")
	deposit := transferBody(bankCAD, alice, 10_000)

	const retries = 100
	statuses := make([]int, retries)
	bodies := make([]string, retries)
	var wg sync.WaitGroup
	for i := range retries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i], bodies[i] = call(t, srv, http.MethodPost, "/v1/transactions", "deposit-1", deposit)
		}()
	}
	wg.Wait()

	for i := range retries {
		if statuses[i] != http.StatusCreated || bodies[i] != bodies[0] {
			t.Fatalf("response %d = %d %s, want the same 201 as response 0: %s", i, statuses[i], bodies[i], bodies[0])
		}
	}
	if n := countTransactions(t, db); n != 1 {
		t.Errorf("transactions = %d after %d identical requests, want 1", n, retries)
	}
	if b := balanceOf(t, srv, alice); b != 10_000 {
		t.Errorf("alice = %d, want 10000", b)
	}
}

func TestProofKeyReuseIsRejected(t *testing.T) {
	srv, db := newProofServer(t)
	alice := createAccount(t, srv, "alice", "CAD")

	if status, body := call(t, srv, http.MethodPost, "/v1/transactions", "k1", transferBody(bankCAD, alice, 100)); status != http.StatusCreated {
		t.Fatalf("first request: %d %s", status, body)
	}
	status, body := call(t, srv, http.MethodPost, "/v1/transactions", "k1", transferBody(bankCAD, alice, 200))

	if status != http.StatusConflict || errorCode(t, body) != "idempotency_key_reused" {
		t.Errorf("reused key with a different body = %d %s, want 409 idempotency_key_reused", status, body)
	}
	if n := countTransactions(t, db); n != 1 {
		t.Errorf("transactions = %d, want 1", n)
	}
}

func TestProofOverdraftRace(t *testing.T) {
	srv, db := newProofServer(t)
	alice := createAccount(t, srv, "alice", "CAD")
	if status, body := call(t, srv, http.MethodPost, "/v1/transactions", "fund", transferBody(bankCAD, alice, 100)); status != http.StatusCreated {
		t.Fatalf("fund alice: %d %s", status, body)
	}

	var done atomic.Bool
	lowest := int64(100)
	var poll sync.WaitGroup
	poll.Add(1)
	go func() {
		defer poll.Done()
		for !done.Load() {
			if b := balanceOf(t, srv, alice); b < lowest {
				lowest = b
			}
		}
	}()

	const withdrawals = 50
	statuses := make([]int, withdrawals)
	bodies := make([]string, withdrawals)
	var wg sync.WaitGroup
	for i := range withdrawals {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i], bodies[i] = call(t, srv, http.MethodPost, "/v1/transactions",
				fmt.Sprintf("withdraw-%d", i), transferBody(alice, bankCAD, 10))
		}()
	}
	wg.Wait()
	done.Store(true)
	poll.Wait()

	succeeded, rejected := 0, 0
	for i := range withdrawals {
		switch {
		case statuses[i] == http.StatusCreated:
			succeeded++
		case statuses[i] == http.StatusUnprocessableEntity && errorCode(t, bodies[i]) == "insufficient_funds":
			rejected++
		default:
			t.Errorf("withdrawal %d = %d %s", i, statuses[i], bodies[i])
		}
	}
	if succeeded != 10 || rejected != 40 {
		t.Errorf("succeeded = %d, rejected = %d, want 10 and 40", succeeded, rejected)
	}
	if b := balanceOf(t, srv, alice); b != 0 {
		t.Errorf("final balance = %d, want 0", b)
	}
	if lowest < 0 {
		t.Errorf("balance was observed at %d during the race", lowest)
	}
	
	var minRunning int64
	err := db.Pool.QueryRow(context.Background(), `
		SELECT COALESCE(MIN(running), 0) FROM (
			SELECT SUM(e.amount) OVER (ORDER BY t.seq) AS running
			FROM entries e JOIN transactions t ON t.id = e.transaction_id
			WHERE e.account_id = $1
		) states`, alice).Scan(&minRunning)
	if err != nil {
		t.Fatalf("running balance: %v", err)
	}
	if minRunning < 0 {
		t.Errorf("a committed state had alice at %d", minRunning)
	}
}

func TestProofEveryEndpoint(t *testing.T) {
	srv, _ := newProofServer(t)
	expect := func(wantStatus int, method, path, key, body string) string {
		t.Helper()
		status, resp := call(t, srv, method, path, key, body)
		if status != wantStatus {
			t.Fatalf("%s %s = %d %s, want %d", method, path, status, resp, wantStatus)
		}
		return resp
	}

	expect(http.StatusOK, http.MethodGet, "/healthz", "", "")

	sys := expect(http.StatusOK, http.MethodGet, "/v1/system-accounts", "", "")
	var system []map[string]any
	if err := json.Unmarshal([]byte(sys), &system); err != nil || len(system) != 4 {
		t.Fatalf("system accounts = %s, want 4", sys)
	}
	resp := expect(http.StatusBadRequest, http.MethodPost, "/v1/accounts", "sneaky", `{"name":"x","currency":"CAD","kind":"system"}`)
	if errorCode(t, resp) != "invalid_request" {
		t.Errorf("creating a system account: %s", resp)
	}

	alice := createAccount(t, srv, "alice", "CAD")
	bob := createAccount(t, srv, "bob", "CAD")
	carl := createAccount(t, srv, "carl", "USD")
	if field(t, expect(http.StatusOK, http.MethodGet, "/v1/accounts/"+alice, "", ""), "kind") != "customer" {
		t.Error("API-created account is not a customer account")
	}

	expect(http.StatusCreated, http.MethodPost, "/v1/transactions", "dep", transferBody(bankCAD, alice, 10_000))
	tx := expect(http.StatusCreated, http.MethodPost, "/v1/transactions", "rent", transferBody(alice, bob, 2_500))
	txID := field(t, tx, "id").(string)
	got := expect(http.StatusOK, http.MethodGet, "/v1/transactions/"+txID, "", "")
	if got != tx {
		t.Errorf("GET transaction = %s, want the posted %s", got, tx)
	}

	if b := balanceOf(t, srv, alice); b != 7_500 {
		t.Errorf("alice = %d, want 7500", b)
	}
	if b := balanceOf(t, srv, bankCAD); b != -10_000 {
		t.Errorf("external bank = %d, want -10000", b)
	}

	page := expect(http.StatusOK, http.MethodGet, "/v1/accounts/"+alice+"/entries?limit=1", "", "")
	next := int64(field(t, page, "next_after_seq").(float64))
	rest := expect(http.StatusOK, http.MethodGet, fmt.Sprintf("/v1/accounts/%s/entries?after_seq=%d", alice, next), "", "")
	if n := len(field(t, page, "entries").([]any)) + len(field(t, rest, "entries").([]any)); n != 2 {
		t.Errorf("alice has %d entries across two pages, want 2", n)
	}

	resp = expect(http.StatusUnprocessableEntity, http.MethodPost, "/v1/transactions", "mixed", transferBody(alice, carl, 1))
	if errorCode(t, resp) != "currency_mismatch" {
		t.Errorf("CAD to USD: %s", resp)
	}
	resp = expect(http.StatusBadRequest, http.MethodPost, "/v1/transactions", "", transferBody(alice, bob, 1))
	if errorCode(t, resp) != "missing_idempotency_key" {
		t.Errorf("no key: %s", resp)
	}
}
