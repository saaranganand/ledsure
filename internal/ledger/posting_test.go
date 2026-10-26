package ledger

import (
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"
)

var (
	bank    = uuid.MustParse("00000000-0000-7000-8000-000000000001")
	carol   = uuid.MustParse("00000000-0000-7000-8000-0000000000c4")
	missing = uuid.MustParse("00000000-0000-7000-8000-0000000000ff")

	postingAccounts = map[uuid.UUID]Account{
		alice: {ID: alice, Currency: CAD, Kind: KindCustomer},
		bob:   {ID: bob, Currency: CAD, Kind: KindCustomer},
		carol: {ID: carol, Currency: USD, Kind: KindCustomer},
		bank:  {ID: bank, Currency: CAD, Kind: KindSystem, AllowNegative: true},
	}
)

func TestCheckPosting(t *testing.T) {
	tests := []struct {
		name     string
		entries  []Entry
		balances map[uuid.UUID]int64
		wantErr  error
	}{
		{
			name:     "transfer with enough funds",
			entries:  []Entry{entry(1, alice, -50), entry(2, bob, 50)},
			balances: map[uuid.UUID]int64{alice: 100},
		},
		{
			name:     "debit to minus one",
			entries:  []Entry{entry(1, alice, -101), entry(2, bob, 101)},
			balances: map[uuid.UUID]int64{alice: 100},
			wantErr:  ErrInsufficientFunds,
		},
		{
			name:     "debit to exactly zero",
			entries:  []Entry{entry(1, alice, -100), entry(2, bob, 100)},
			balances: map[uuid.UUID]int64{alice: 100},
		},
		{
			name:     "deposit from the bank",
			entries:  []Entry{entry(1, bank, -100), entry(2, alice, 100)},
			balances: nil,
		},
		{
			name:     "unknown account",
			entries:  []Entry{entry(1, alice, -1), entry(2, missing, 1)},
			balances: map[uuid.UUID]int64{alice: 100},
			wantErr:  ErrAccountNotFound,
		},
		{
			name:     "currency mismatch",
			entries:  []Entry{entry(1, alice, -1), entry(2, carol, 1)},
			balances: map[uuid.UUID]int64{alice: 100},
			wantErr:  ErrCurrencyMismatch,
		},
		{
			name:     "missing beats overdraft",
			entries:  []Entry{entry(1, alice, -1000), entry(2, missing, 1000)},
			balances: nil,
			wantErr:  ErrAccountNotFound,
		},
		{
			name:     "balance would overflow",
			entries:  []Entry{entry(1, bank, -10), entry(2, alice, 10)},
			balances: map[uuid.UUID]int64{alice: (math.MaxInt64 - 5)},
			wantErr:  ErrInvalidInput,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckPosting(Transaction{Entries: tc.entries}, postingAccounts, tc.balances)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
