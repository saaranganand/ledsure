package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/saaranganand/ledsure/internal/ledger"
)

func (s *Store) GetTransaction(ctx context.Context, id uuid.UUID) (ledger.Transaction, error) {
	// load the header, no row means not found
	tx := ledger.Transaction{ID: id}
	err := s.pool.QueryRow(ctx,
		`SELECT seq, description, posted_at FROM transactions WHERE id = $1`, id,
	).Scan(&tx.Seq, &tx.Description, &tx.PostedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ledger.Transaction{}, ErrNotFound
	}
	if err != nil {
		return ledger.Transaction{}, fmt.Errorf("store: get transaction: %w", err)
	}
	tx.PostedAt = tx.PostedAt.UTC()

	// load entries, no db tx needed
	rows, err := s.pool.Query(ctx,
		`SELECT id, account_id, amount FROM entries WHERE transaction_id = $1 ORDER BY id`, id,
	)
	if err != nil {
		return ledger.Transaction{}, fmt.Errorf("store: get transaction entries: %w", err)
	}
	tx.Entries, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (ledger.Entry, error) {
		var e ledger.Entry
		err := row.Scan(&e.ID, &e.AccountID, &e.Amount)
		return e, err
	})
	if err != nil {
		return ledger.Transaction{}, fmt.Errorf("store: get transaction entries: %w", err)
	}
	return tx, nil
}

// computed from entries on every read, never stored
type Balance struct {
	AccountID uuid.UUID
	Currency  ledger.Currency
	Amount    int64
	AsOfSeq   int64
}

// newest transaction touching the acct
// never names a seq that hasnt committed
func (s *Store) GetBalance(ctx context.Context, accountID uuid.UUID) (Balance, error) {
	b := Balance{AccountID: accountID}
	err := s.pool.QueryRow(ctx, `
		SELECT a.currency, COALESCE(SUM(e.amount), 0)::bigint, COALESCE(MAX(t.seq), 0)
		FROM accounts a
		LEFT JOIN entries e ON e.account_id = a.id
		LEFT JOIN transactions t ON t.id = e.transaction_id
		WHERE a.id = $1
		GROUP BY a.id`,
		accountID,
	).Scan(&b.Currency, &b.Amount, &b.AsOfSeq)
	if errors.Is(err, pgx.ErrNoRows) {
		return Balance{}, ErrNotFound
	}
	if err != nil {
		return Balance{}, fmt.Errorf("store: get balance: %w", err)
	}
	return b, nil
}

// one line of an acct history, with its transaction's header
type AccountEntry struct {
	EntryID       uuid.UUID
	TransactionID uuid.UUID
	Seq           int64
	Amount        int64
	Description   string
	PostedAt      time.Time
}

func (s *Store) ListEntries(ctx context.Context, accountID uuid.UUID, afterSeq int64, limit int) ([]AccountEntry, error) {
	// check the acct exists, so new acct (empty list) and missing one (not found) differ
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM accounts WHERE id = $1)`, accountID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("store: list entries: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}

	// next page after the cursor, safe
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.transaction_id, t.seq, e.amount, t.description, t.posted_at
		FROM entries e
		JOIN transactions t ON t.id = e.transaction_id
		WHERE e.account_id = $1 AND t.seq > $2
		ORDER BY t.seq
		LIMIT $3`,
		accountID, afterSeq, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("store: list entries: %w", err)
	}
	entries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AccountEntry, error) {
		var e AccountEntry
		err := row.Scan(&e.EntryID, &e.TransactionID, &e.Seq, &e.Amount, &e.Description, &e.PostedAt)
		e.PostedAt = e.PostedAt.UTC()
		return e, err
	})
	if err != nil {
		return nil, fmt.Errorf("store: list entries: %w", err)
	}
	return entries, nil
}
