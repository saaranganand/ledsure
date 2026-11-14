//go:build integration

package store

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/saaranganand/ledsure/internal/ledger"
)

func respond(status int, body string) func(context.Context) IdempotentResponse {
	return func(context.Context) IdempotentResponse {
		return IdempotentResponse{Status: status, Body: []byte(body)}
	}
}

func TestIdempotentReplaysExactBytes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	var calls atomic.Int32
	fn := func(context.Context) IdempotentResponse {
		calls.Add(1)
		return IdempotentResponse{Status: 201, Body: []byte(`{"b":1,  "a":2}`)}
	}

	first, err := s.Idempotent(ctx, "k1", []byte{1}, postedAt, fn)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	second, err := s.Idempotent(ctx, "k1", []byte{1}, postedAt, fn)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if calls.Load() != 1 {
		t.Errorf("fn ran %d times, want 1", calls.Load())
	}
	if second.Status != first.Status || string(second.Body) != string(first.Body) {
		t.Errorf("replay = %d %s, want %d %s", second.Status, second.Body, first.Status, first.Body)
	}

	if _, err := s.Idempotent(ctx, "k1", []byte{2}, postedAt, fn); !errors.Is(err, ErrKeyReused) {
		t.Errorf("different hash: err = %v, want ErrKeyReused", err)
	}
}

func TestIdempotentServerErrorRollsBackEverything(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	account, err := ledger.NewCustomerAccount(idGen.New(), postedAt, "ghost", ledger.CAD)
	if err != nil {
		t.Fatalf("NewCustomerAccount: %v", err)
	}

	resp, err := s.Idempotent(ctx, "k500", []byte{1}, postedAt, func(ctx context.Context) IdempotentResponse {
		if err := s.CreateAccount(ctx, account); err != nil {
			t.Fatalf("CreateAccount inside fn: %v", err)
		}
		return IdempotentResponse{Status: 500, Body: []byte(`{}`)}
	})
	if err != nil || resp.Status != 500 {
		t.Fatalf("resp = %d, err = %v, want 500 and no error", resp.Status, err)
	}

	if _, err := s.GetAccount(ctx, account.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("account written inside a failed request survived: err = %v", err)
	}
	if n := countRows(t, s, "idempotency_keys"); n != 0 {
		t.Errorf("idempotency_keys = %d after a 500, want 0", n)
	}

	retry, err := s.Idempotent(ctx, "k500", []byte{1}, postedAt, respond(201, `{"ok":true}`))
	if err != nil || retry.Status != 201 {
		t.Errorf("retry after 500 = %d, err = %v, want a fresh 201", retry.Status, err)
	}
}

func TestIdempotentStoresRejectionThroughSavepoint(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	alice := newCustomer(t, s, "alice", ledger.CAD)
	bob := newCustomer(t, s, "bob", ledger.CAD)

	reject := func(ctx context.Context) IdempotentResponse {
		tx, err := ledger.NewTransaction(idGen.New(), postedAt, "overdraft", []ledger.Entry{
			{ID: idGen.New(), AccountID: alice, Amount: -5},
			{ID: idGen.New(), AccountID: bob, Amount: 5},
		})
		if err != nil {
			t.Fatalf("NewTransaction: %v", err)
		}
		if _, err := s.PostTransaction(ctx, tx); !errors.Is(err, ledger.ErrInsufficientFunds) {
			t.Fatalf("PostTransaction inside fn: err = %v, want ErrInsufficientFunds", err)
		}
		return IdempotentResponse{Status: 422, Body: []byte(`{"error":{"code":"insufficient_funds"}}`)}
	}

	first, err := s.Idempotent(ctx, "k422", []byte{1}, postedAt, reject)
	if err != nil {
		t.Fatalf("Idempotent: %v", err)
	}
	if n := countRows(t, s, "entries"); n != 0 {
		t.Errorf("entries = %d after a rejected posting, want 0", n)
	}

	replay, err := s.Idempotent(ctx, "k422", []byte{1}, postedAt, respond(201, `{}`))
	if err != nil || replay.Status != 422 || string(replay.Body) != string(first.Body) {
		t.Errorf("replay = %d %s, err = %v, want the stored 422", replay.Status, replay.Body, err)
	}
}

func TestIdempotentCommitsPostingWithKey(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	alice := newCustomer(t, s, "alice", ledger.CAD)
	mustPost(t, s, map[uuid.UUID]int64{bankCAD: -100, alice: 100})
	bob := newCustomer(t, s, "bob", ledger.CAD)

	transfer := func(ctx context.Context) IdempotentResponse {
		tx, err := ledger.NewTransaction(idGen.New(), postedAt, "rent", []ledger.Entry{
			{ID: idGen.New(), AccountID: alice, Amount: -40},
			{ID: idGen.New(), AccountID: bob, Amount: 40},
		})
		if err != nil {
			t.Fatalf("NewTransaction: %v", err)
		}
		if _, err := s.PostTransaction(ctx, tx); err != nil {
			t.Fatalf("PostTransaction inside fn: %v", err)
		}
		return IdempotentResponse{Status: 201, Body: []byte(`{}`)}
	}

	for range 3 {
		if _, err := s.Idempotent(ctx, "k-rent", []byte{1}, postedAt, transfer); err != nil {
			t.Fatalf("Idempotent: %v", err)
		}
	}

	b, err := s.GetBalance(ctx, bob)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if b.Amount != 40 {
		t.Errorf("bob = %d after three retries, want 40", b.Amount)
	}
}

func TestIdempotentConcurrentSameKey(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	var calls atomic.Int32
	slow := func(context.Context) IdempotentResponse {
		calls.Add(1)
		time.Sleep(100 * time.Millisecond)
		return IdempotentResponse{Status: 201, Body: []byte(`{"n":1}`)}
	}

	const callers = 10
	var wg sync.WaitGroup
	results := make([]IdempotentResponse, callers)
	errs := make([]error, callers)
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = s.Idempotent(ctx, "k-concurrent", []byte{9}, postedAt, slow)
		}()
	}
	wg.Wait()

	if calls.Load() != 1 {
		t.Errorf("fn ran %d times across %d concurrent callers, want 1", calls.Load(), callers)
	}
	for i := range callers {
		if errs[i] != nil || results[i].Status != 201 || string(results[i].Body) != `{"n":1}` {
			t.Errorf("caller %d got %d %s, err = %v", i, results[i].Status, results[i].Body, errs[i])
		}
	}
}
