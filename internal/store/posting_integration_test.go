//go:build integration

package store

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/saaranganand/ledsure/internal/ledger"
	"github.com/saaranganand/ledsure/internal/platform"
)

var (
	idGen    = platform.UUIDv7Generator{}
	postedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	bankCAD  = uuid.MustParse("00000000-0000-7000-8000-000000000001")
)

func newCustomer(t *testing.T, s *Store, name string, currency ledger.Currency) uuid.UUID {
	t.Helper()
	a, err := ledger.NewCustomerAccount(idGen.New(), postedAt, name, currency)
	if err != nil {
		t.Fatalf("NewCustomerAccount: %v", err)
	}
	if err := s.CreateAccount(context.Background(), a); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	return a.ID
}

// builds a transaction from (account, amount) pairs and posts it
func post(s *Store, amounts map[uuid.UUID]int64) (ledger.Transaction, error) {
	var entries []ledger.Entry
	for account, amount := range amounts {
		entries = append(entries, ledger.Entry{ID: idGen.New(), AccountID: account, Amount: amount})
	}
	tx, err := ledger.NewTransaction(idGen.New(), postedAt, "test", entries)
	if err != nil {
		return ledger.Transaction{}, err
	}
	return s.PostTransaction(context.Background(), tx)
}

func mustPost(t *testing.T, s *Store, amounts map[uuid.UUID]int64) ledger.Transaction {
	t.Helper()
	tx, err := post(s, amounts)
	if err != nil {
		t.Fatalf("post %v: %v", amounts, err)
	}
	return tx
}

func countRows(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.pool.QueryRow(context.Background(), `SELECT count(*) FROM `+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestPostAndGetTransaction(t *testing.T) {
	s := newTestStore(t)
	alice := newCustomer(t, s, "alice", ledger.CAD)
	bob := newCustomer(t, s, "bob", ledger.CAD)

	deposit := mustPost(t, s, map[uuid.UUID]int64{bankCAD: -1000, alice: 1000})
	transfer := mustPost(t, s, map[uuid.UUID]int64{alice: -300, bob: 300})

	if deposit.Seq != 1 || transfer.Seq != 2 {
		t.Errorf("seqs = %d, %d, want 1, 2", deposit.Seq, transfer.Seq)
	}

	got, err := s.GetTransaction(context.Background(), transfer.ID)
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if !reflect.DeepEqual(got, transfer) {
		t.Errorf("got %+v, want %+v", got, transfer)
	}

	if _, err := s.GetTransaction(context.Background(), idGen.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing transaction: err = %v, want ErrNotFound", err)
	}
}

func TestPostRejectsWithoutWriting(t *testing.T) {
	s := newTestStore(t)
	alice := newCustomer(t, s, "alice", ledger.CAD)
	bob := newCustomer(t, s, "bob", ledger.CAD)
	carol := newCustomer(t, s, "carol", ledger.USD)
	mustPost(t, s, map[uuid.UUID]int64{bankCAD: -100, alice: 100})

	tests := []struct {
		name    string
		amounts map[uuid.UUID]int64
		wantErr error
	}{
		{"insufficient funds", map[uuid.UUID]int64{alice: -101, bob: 101}, ledger.ErrInsufficientFunds},
		{"currency mismatch", map[uuid.UUID]int64{alice: -10, carol: 10}, ledger.ErrCurrencyMismatch},
		{"unknown account", map[uuid.UUID]int64{alice: -10, idGen.New(): 10}, ledger.ErrAccountNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := countRows(t, s, "entries")

			_, err := post(s, tc.amounts)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if after := countRows(t, s, "entries"); after != before {
				t.Errorf("entries went from %d to %d after a rejected posting", before, after)
			}
		})
	}
}

func TestConcurrentOppositeTransfers(t *testing.T) {
	s := newTestStore(t)
	alice := newCustomer(t, s, "alice", ledger.CAD)
	bob := newCustomer(t, s, "bob", ledger.CAD)
	mustPost(t, s, map[uuid.UUID]int64{bankCAD: -1000, alice: 1000})
	mustPost(t, s, map[uuid.UUID]int64{bankCAD: -1000, bob: 1000})

	const pairs = 50
	var wg sync.WaitGroup
	errs := make(chan error, 2*pairs)
	for range pairs {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := post(s, map[uuid.UUID]int64{alice: -1, bob: 1}); err != nil {
				errs <- err
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := post(s, map[uuid.UUID]int64{bob: -1, alice: 1}); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent transfer failed: %v", err)
	}
	for _, account := range []uuid.UUID{alice, bob} {
		b, err := s.GetBalance(context.Background(), account)
		if err != nil {
			t.Fatalf("GetBalance: %v", err)
		}
		if b.Amount != 1000 {
			t.Errorf("balance of %s = %d, want 1000", account, b.Amount)
		}
	}
}

func TestGetBalance(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	alice := newCustomer(t, s, "alice", ledger.CAD)
	bob := newCustomer(t, s, "bob", ledger.CAD)
	idle := newCustomer(t, s, "idle", ledger.USD)

	mustPost(t, s, map[uuid.UUID]int64{bankCAD: -1000, alice: 1000})
	transfer := mustPost(t, s, map[uuid.UUID]int64{alice: -300, bob: 300})
	mustPost(t, s, map[uuid.UUID]int64{bankCAD: -50, bob: 50})

	tests := []struct {
		name    string
		account uuid.UUID
		want    Balance
	}{
		{"after deposit and transfer", alice, Balance{AccountID: alice, Currency: ledger.CAD, Amount: 700, AsOfSeq: transfer.Seq}},
		{"external bank is negative", bankCAD, Balance{AccountID: bankCAD, Currency: ledger.CAD, Amount: -1050, AsOfSeq: 3}},
		{"no entries yet", idle, Balance{AccountID: idle, Currency: ledger.USD, Amount: 0, AsOfSeq: 0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.GetBalance(ctx, tc.account)
			if err != nil {
				t.Fatalf("GetBalance: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}

	if _, err := s.GetBalance(ctx, idGen.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing account: err = %v, want ErrNotFound", err)
	}
}

func TestListEntriesPaginates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	alice := newCustomer(t, s, "alice", ledger.CAD)
	bob := newCustomer(t, s, "bob", ledger.CAD)

	mustPost(t, s, map[uuid.UUID]int64{bankCAD: -1000, alice: 1000})
	for range 4 {
		mustPost(t, s, map[uuid.UUID]int64{alice: -10, bob: 10})
	}

	var seqs []int64
	afterSeq := int64(0)
	for {
		page, err := s.ListEntries(ctx, alice, afterSeq, 2)
		if err != nil {
			t.Fatalf("ListEntries: %v", err)
		}
		if len(page) == 0 {
			break
		}
		for _, e := range page {
			seqs = append(seqs, e.Seq)
		}
		afterSeq = page[len(page)-1].Seq
	}
	if want := []int64{1, 2, 3, 4, 5}; !reflect.DeepEqual(seqs, want) {
		t.Errorf("seqs = %v, want %v", seqs, want)
	}

	empty, err := s.ListEntries(ctx, newCustomer(t, s, "new", ledger.CAD), 0, 10)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("new account: entries = %v, err = %v, want empty non-nil slice", empty, err)
	}
	if _, err := s.ListEntries(ctx, idGen.New(), 0, 10); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing account: err = %v, want ErrNotFound", err)
	}
}
