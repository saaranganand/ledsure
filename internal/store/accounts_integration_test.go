//go:build integration

package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/saaranganand/ledsure/internal/ledger"
)

func TestCreateAndGetAccount(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id := uuid.MustParse("00000000-0000-7000-8000-0000000000aa")
	now := time.Date(2026, 1, 2, 3, 4, 5, 123456000, time.UTC)
	want, err := ledger.NewCustomerAccount(id, now, "alice", ledger.CAD)
	if err != nil {
		t.Fatalf("NewCustomerAccount: %v", err)
	}

	if err := s.CreateAccount(ctx, want); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	got, err := s.GetAccount(ctx, id)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestGetAccountNotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetAccount(context.Background(), uuid.MustParse("00000000-0000-7000-8000-0000000000bb"))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestListSystemAccounts(t *testing.T) {
	s := newTestStore(t)

	accounts, err := s.ListSystemAccounts(context.Background())
	if err != nil {
		t.Fatalf("ListSystemAccounts: %v", err)
	}

	wantCodes := []string{
		"SYSTEM_EXTERNAL_BANK_CAD",
		"SYSTEM_EXTERNAL_BANK_USD",
		"SYSTEM_FEE_REVENUE_CAD",
		"SYSTEM_FEE_REVENUE_USD",
	}
	if len(accounts) != len(wantCodes) {
		t.Fatalf("got %d accounts, want %d", len(accounts), len(wantCodes))
	}
	for i, a := range accounts {
		if a.Kind != ledger.KindSystem {
			t.Errorf("accounts[%d].Kind = %q, want system", i, a.Kind)
		}
		if a.Code == nil || *a.Code != wantCodes[i] {
			t.Errorf("accounts[%d].Code = %v, want %s", i, a.Code, wantCodes[i])
		}
	}
}

func TestAccountConstraints(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	code := "SOME_CODE"
	valid := ledger.Account{
		ID:        uuid.MustParse("00000000-0000-7000-8000-000000000100"),
		Name:      "alice",
		Currency:  ledger.CAD,
		Kind:      ledger.KindCustomer,
		CreatedAt: now,
	}

	tests := []struct {
		name     string
		modify   func(a *ledger.Account)
		wantCode string
	}{
		{"unsupported currency", func(a *ledger.Account) { a.Currency = "EUR" }, "23514"},
		{"empty name", func(a *ledger.Account) { a.Name = "" }, "23514"},
		{"customer with code", func(a *ledger.Account) { a.Code = &code }, "23514"},
		{"system without code", func(a *ledger.Account) { a.Kind = ledger.KindSystem }, "23514"},
		{"unknown kind", func(a *ledger.Account) { a.Kind = "admin" }, "23514"},
		{"duplicate id", func(a *ledger.Account) { a.ID = uuid.MustParse("00000000-0000-7000-8000-000000000001") }, "23505"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := valid
			tc.modify(&a)

			err := s.CreateAccount(ctx, a)

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) {
				t.Fatalf("err = %v, want a postgres error", err)
			}
			if pgErr.Code != tc.wantCode {
				t.Errorf("code = %s (%s), want %s", pgErr.Code, pgErr.ConstraintName, tc.wantCode)
			}
		})
	}

	if err := s.CreateAccount(ctx, valid); err != nil {
		t.Errorf("valid account rejected: %v", err)
	}
}

