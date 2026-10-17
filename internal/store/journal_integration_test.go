//go:build integration

package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// db itself must reject edits to the journal
func TestJournalIsAppendOnly(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO transactions (id, description, posted_at)
		VALUES ('00000000-0000-7000-8000-000000000a01', 'deposit', now());
		INSERT INTO entries (id, transaction_id, account_id, amount) VALUES
			('00000000-0000-7000-8000-000000000e01', '00000000-0000-7000-8000-000000000a01', '00000000-0000-7000-8000-000000000001', -100),
			('00000000-0000-7000-8000-000000000e02', '00000000-0000-7000-8000-000000000a01', '00000000-0000-7000-8000-000000000002', 100);
	`)
	if err != nil {
		t.Fatalf("seed journal: %v", err)
	}

	statements := []string{
		`UPDATE transactions SET description = 'edited'`,
		`DELETE FROM transactions`,
		`TRUNCATE transactions CASCADE`,
		`UPDATE entries SET amount = 1`,
		`DELETE FROM entries`,
		`TRUNCATE entries`,
	}
	for _, stmt := range statements {
		t.Run(stmt, func(t *testing.T) {
			_, err := s.pool.Exec(ctx, stmt)

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) {
				t.Fatalf("err = %v, want a postgres error", err)
			}
			if pgErr.Code != "P0001" {
				t.Errorf("code = %s (%s), want P0001", pgErr.Code, pgErr.Message)
			}
		})
	}

	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM entries`).Scan(&count); err != nil {
		t.Fatalf("count entries: %v", err)
	}
	if count != 2 {
		t.Errorf("entries = %d after rejected changes, want 2", count)
	}
}
