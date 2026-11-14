//go:build integration

package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/saaranganand/ledsure/internal/ledger"
)

var bankFor = map[ledger.Currency]uuid.UUID{
	ledger.CAD: uuid.MustParse("00000000-0000-7000-8000-000000000001"),
	ledger.USD: uuid.MustParse("00000000-0000-7000-8000-000000000003"),
}

// each query returns the number of rows that break one invariant, so every one must return 0
var invariantViolations = map[string]string{
	"every transaction sums to zero": `
		SELECT count(*) FROM (
			SELECT transaction_id FROM entries GROUP BY transaction_id HAVING SUM(amount) <> 0
		) bad`,
	"the ledger sums to zero per currency": `
		SELECT count(*) FROM (
			SELECT a.currency FROM entries e JOIN accounts a ON a.id = e.account_id
			GROUP BY a.currency HAVING SUM(e.amount) <> 0
		) bad`,
	"no-overdraft accounts are never negative": `
		SELECT count(*) FROM (
			SELECT a.id FROM entries e JOIN accounts a ON a.id = e.account_id
			WHERE NOT a.allow_negative GROUP BY a.id HAVING SUM(e.amount) < 0
		) bad`,
	"each transaction uses one currency": `
		SELECT count(*) FROM (
			SELECT e.transaction_id FROM entries e JOIN accounts a ON a.id = e.account_id
			GROUP BY e.transaction_id HAVING COUNT(DISTINCT a.currency) > 1
		) bad`,
	"no stored balance column exists": `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema = 'public' AND column_name LIKE '%balance%'`,
}

// random opens, deposits, transfers and withdrawals
func TestInvariantsHoldAfterEveryStep(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	rapid.Check(t, func(t *rapid.T) {
		var customers []uuid.UUID
		currencyOf := map[uuid.UUID]ledger.Currency{}
		model := map[uuid.UUID]int64{}

		post := func(entries []ledger.Entry) error {
			tx, err := ledger.NewTransaction(idGen.New(), postedAt, "", entries)
			if err != nil {
				return err
			}
			_, err = s.PostTransaction(ctx, tx)
			return err
		}
		move := func(from, to uuid.UUID, amount int64) error {
			return post([]ledger.Entry{
				{ID: idGen.New(), AccountID: from, Amount: -amount},
				{ID: idGen.New(), AccountID: to, Amount: amount},
			})
		}
		pick := func(label string) uuid.UUID {
			return rapid.SampledFrom(customers).Draw(t, label)
		}
		amount := func() int64 {
			return rapid.OneOf(rapid.Int64Range(1, 100), rapid.Int64Range(1, 1_000_000)).Draw(t, "amount")
		}

		for range rapid.IntRange(1, 25).Draw(t, "steps") {
			action := "open"
			if len(customers) > 0 {
				action = rapid.SampledFrom([]string{"open", "deposit", "transfer", "withdraw"}).Draw(t, "action")
			}

			var err, want error
			switch action {
			case "open":
				currency := rapid.SampledFrom([]ledger.Currency{ledger.CAD, ledger.USD}).Draw(t, "currency")
				account, nerr := ledger.NewCustomerAccount(idGen.New(), postedAt, "customer", currency)
				if nerr != nil {
					t.Fatalf("NewCustomerAccount: %v", nerr)
				}
				err = s.CreateAccount(ctx, account)
				customers = append(customers, account.ID)
				currencyOf[account.ID] = currency

			case "deposit":
				to, amt := pick("to"), amount()
				if err = move(bankFor[currencyOf[to]], to, amt); err == nil {
					model[to] += amt
				}

			case "withdraw":
				from, amt := pick("from"), amount()
				if model[from] < amt {
					want = ledger.ErrInsufficientFunds
				}
				if err = move(from, bankFor[currencyOf[from]], amt); err == nil {
					model[from] -= amt
				}

			case "transfer":
				from, to, amt := pick("from"), pick("to"), amount()
				switch {
				case from == to:
					want = ledger.ErrInvalidInput
				case currencyOf[from] != currencyOf[to]:
					want = ledger.ErrCurrencyMismatch
				case model[from] < amt:
					want = ledger.ErrInsufficientFunds
				}
				if err = move(from, to, amt); err == nil {
					model[from] -= amt
					model[to] += amt
				}
			}

			if want == nil && err != nil {
				t.Fatalf("%s rejected: %v", action, err)
			}
			if want != nil && !errors.Is(err, want) {
				t.Fatalf("%s: err = %v, want %v", action, err, want)
			}

			for name, query := range invariantViolations {
				var n int
				if err := s.pool.QueryRow(ctx, query).Scan(&n); err != nil {
					t.Fatalf("check %q: %v", name, err)
				}
				if n != 0 {
					t.Fatalf("after %s: invariant %q broken (%d violations)", action, name, n)
				}
			}
			for _, id := range customers {
				b, err := s.GetBalance(ctx, id)
				if err != nil {
					t.Fatalf("GetBalance: %v", err)
				}
				if b.Amount != model[id] {
					t.Fatalf("after %s: balance of %s = %d, model says %d", action, id, b.Amount, model[id])
				}
			}
		}
	})
}
