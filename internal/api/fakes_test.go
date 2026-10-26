package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/saaranganand/ledsure/internal/ledger"
	"github.com/saaranganand/ledsure/internal/store"
)

type fakeStore struct {
	accounts map[uuid.UUID]ledger.Account
	system   []ledger.Account
	posted   map[uuid.UUID]ledger.Transaction
	balances map[uuid.UUID]store.Balance
	entries  map[uuid.UUID][]store.AccountEntry
	err      error

	// the paging arguments of the last ListEntries call, so tests can check query parsing
	lastAfterSeq int64
	lastLimit    int
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		accounts: map[uuid.UUID]ledger.Account{},
		posted:   map[uuid.UUID]ledger.Transaction{},
		balances: map[uuid.UUID]store.Balance{},
		entries:  map[uuid.UUID][]store.AccountEntry{},
	}
}

func (f *fakeStore) CreateAccount(_ context.Context, a ledger.Account) error {
	if f.err != nil {
		return f.err
	}
	f.accounts[a.ID] = a
	return nil
}

func (f *fakeStore) GetAccount(_ context.Context, id uuid.UUID) (ledger.Account, error) {
	if f.err != nil {
		return ledger.Account{}, f.err
	}
	a, ok := f.accounts[id]
	if !ok {
		return ledger.Account{}, store.ErrNotFound
	}
	return a, nil
}

func (f *fakeStore) ListSystemAccounts(_ context.Context) ([]ledger.Account, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.system, nil
}

func (f *fakeStore) PostTransaction(_ context.Context, tx ledger.Transaction) (ledger.Transaction, error) {
	if f.err != nil {
		return ledger.Transaction{}, f.err
	}
	tx.Seq = int64(len(f.posted) + 1)
	f.posted[tx.ID] = tx
	return tx, nil
}

func (f *fakeStore) GetTransaction(_ context.Context, id uuid.UUID) (ledger.Transaction, error) {
	if f.err != nil {
		return ledger.Transaction{}, f.err
	}
	tx, ok := f.posted[id]
	if !ok {
		return ledger.Transaction{}, store.ErrNotFound
	}
	return tx, nil
}

func (f *fakeStore) GetBalance(_ context.Context, accountID uuid.UUID) (store.Balance, error) {
	if f.err != nil {
		return store.Balance{}, f.err
	}
	b, ok := f.balances[accountID]
	if !ok {
		return store.Balance{}, store.ErrNotFound
	}
	return b, nil
}

func (f *fakeStore) ListEntries(_ context.Context, accountID uuid.UUID, afterSeq int64, limit int) ([]store.AccountEntry, error) {
	f.lastAfterSeq, f.lastLimit = afterSeq, limit
	if f.err != nil {
		return nil, f.err
	}
	entries, ok := f.entries[accountID]
	if !ok {
		return nil, store.ErrNotFound
	}
	return entries, nil
}

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

// hands out testID, then testID+1, and so on, so every generated id is predictable
type fakeIDs struct{ next byte }

func (g *fakeIDs) New() uuid.UUID {
	id := testID
	id[15] += g.next
	g.next++
	return id
}

var (
	testID  = uuid.MustParse("00000000-0000-7000-8000-0000000000aa")
	testNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
)

func newTestRouter(s *fakeStore) http.Handler {
	return NewRouter(Deps{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Accounts: s,
		Journal:  s,
		Clock:    fakeClock{now: testNow},
		IDs:      &fakeIDs{},
	})
}

// sends 1 request through router and returns response
func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
