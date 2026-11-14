package ledger

import (
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"pgregory.net/rapid"
)

// small pool so generated transactions often repeat one
func genAccountID(t *rapid.T, label string) uuid.UUID {
	return uuid.UUID{15: rapid.ByteRange(1, 6).Draw(t, label)}
}

func genAmount(t *rapid.T, label string) int64 {
	return rapid.OneOf(
		rapid.Int64Range(-50, 50),
		rapid.Int64Range(-1_000_000, 1_000_000),
		rapid.SampledFrom([]int64{0, 1, -1, maxAmount, -maxAmount, maxAmount + 1, -maxAmount - 1, math.MaxInt64, math.MinInt64}),
	).Draw(t, label)
}

func genEntries(t *rapid.T) []Entry {
	n := rapid.OneOf(rapid.IntRange(0, 4), rapid.IntRange(0, maxEntries+2)).Draw(t, "entries")
	entries := make([]Entry, n)
	for i := range entries {
		entries[i] = Entry{ID: uuid.UUID{14: byte(i)}, AccountID: genAccountID(t, "account"), Amount: genAmount(t, "amount")}
	}

	if n > 0 && rapid.Bool().Draw(t, "balance") {
		sum := new(big.Int)
		for _, e := range entries[:n-1] {
			sum.Add(sum, big.NewInt(e.Amount))
		}
		if neg := new(big.Int).Neg(sum); neg.IsInt64() {
			entries[n-1].Amount = neg.Int64()
		}
	}
	return entries
}

func genValidEntries(t *rapid.T) []Entry {
	ids := rapid.SliceOfNDistinct(rapid.ByteRange(1, 8), 2, 6, rapid.ID[byte]).Draw(t, "accounts")
	entries := make([]Entry, len(ids))
	var sum int64
	for i, b := range ids[:len(ids)-1] {
		amount := rapid.OneOf(rapid.Int64Range(-50, 50), rapid.Int64Range(-1_000_000, 1_000_000)).
			Filter(func(v int64) bool { return v != 0 }).Draw(t, "amount")
		entries[i] = Entry{ID: uuid.UUID{14: byte(i)}, AccountID: uuid.UUID{15: b}, Amount: amount}
		sum += amount
	}

	if sum == 0 {
		sum += entries[0].Amount
		entries[0].Amount *= 2
	}
	last := len(ids) - 1
	entries[last] = Entry{ID: uuid.UUID{14: byte(last)}, AccountID: uuid.UUID{15: ids[last]}, Amount: -sum}
	return entries
}

func expectedStructuralErr(entries []Entry) error {
	if len(entries) < 2 || len(entries) > maxEntries {
		return ErrInvalidInput
	}
	seen := map[uuid.UUID]bool{}
	sum := new(big.Int)
	for _, e := range entries {
		if e.Amount == 0 || e.Amount > maxAmount || e.Amount < -maxAmount || seen[e.AccountID] {
			return ErrInvalidInput
		}
		seen[e.AccountID] = true
		sum.Add(sum, big.NewInt(e.Amount))
	}
	if sum.Sign() != 0 {
		return ErrUnbalanced
	}
	return nil
}

func TestNewTransactionMatchesRules(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		entries := genEntries(t)

		tx, err := NewTransaction(txID, txNow, "", entries)

		want := expectedStructuralErr(entries)
		if want == nil {
			if err != nil {
				t.Fatalf("valid entries rejected: %v", err)
			}
			if !reflect.DeepEqual(tx.Entries, entries) {
				t.Fatalf("entries changed: got %v, want %v", tx.Entries, entries)
			}
			return
		}
		if !errors.Is(err, want) {
			t.Fatalf("err = %v, want %v", err, want)
		}
	})
}

func TestCheckPostingMatchesRules(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		accounts := map[uuid.UUID]Account{}
		balances := map[uuid.UUID]int64{}
		for i := byte(1); i <= 8; i++ {
			id := uuid.UUID{15: i}
			if rapid.Bool().Draw(t, "exists") {
				allowNegative := rapid.Bool().Draw(t, "allow negative")
				accounts[id] = Account{
					ID:            id,
					Currency:      rapid.SampledFrom([]Currency{CAD, USD}).Draw(t, "currency"),
					AllowNegative: allowNegative,
				}
				lo := int64(0)
				if allowNegative {
					lo = -1_000_000
				}
				balances[id] = rapid.OneOf(rapid.Int64Range(lo, 50), rapid.Int64Range(lo, 1_000_000)).Draw(t, "balance")
			}
		}

		entries := genValidEntries(t)
		if err := expectedStructuralErr(entries); err != nil {
			t.Fatalf("generator produced an invalid transaction: %v", err)
		}

		err := CheckPosting(Transaction{Entries: entries}, accounts, balances)

		var want error
		var currency Currency
		for _, e := range entries {
			a, ok := accounts[e.AccountID]
			switch {
			case !ok:
				want = ErrAccountNotFound
			case currency != "" && a.Currency != currency:
				want = ErrCurrencyMismatch
			}
			if want != nil {
				break
			}
			currency = a.Currency
		}
		if want == nil {
			for _, e := range entries {
				if balances[e.AccountID]+e.Amount < 0 && !accounts[e.AccountID].AllowNegative {
					want = ErrInsufficientFunds
					break
				}
			}
		}

		if want == nil && err != nil {
			t.Fatalf("valid posting rejected: %v", err)
		}
		if want != nil && !errors.Is(err, want) {
			t.Fatalf("err = %v, want %v", err, want)
		}
	})
}
