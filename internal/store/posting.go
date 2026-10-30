package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/saaranganand/ledsure/internal/ledger"
)

func (s *Store) PostTransaction(ctx context.Context, tx ledger.Transaction) (ledger.Transaction, error) {
	// begin a db transaction
	dbtx, err := s.db(ctx).Begin(ctx)
	if err != nil {
		return ledger.Transaction{}, fmt.Errorf("store: post transaction: begin: %w", err)
	}
	defer func() { _ = dbtx.Rollback(ctx) }() // without this, early return later would leave tx open

	// acc id of every entry
	ids := make([]uuid.UUID, 0, len(tx.Entries))
	for _, entry := range tx.Entries {
		ids = append(ids, entry.AccountID)
	}

	// lock accts, load into map (key id)
	rows, err := dbtx.Query(ctx,
		`SELECT `+accountColumns+` FROM accounts WHERE id = ANY($1) ORDER BY id FOR UPDATE`,
		ids,
	)
	if err != nil {
		return ledger.Transaction{}, fmt.Errorf("store: post transaction: lock accounts: %w", err)
	}
	locked, err := pgx.CollectRows(rows, scanAccount)
	if err != nil {
		return ledger.Transaction{}, fmt.Errorf("store: post transaction: lock accounts: %w", err)
	}

	accounts := make(map[uuid.UUID]ledger.Account, len(locked))
	for _, a := range locked {
		accounts[a.ID] = a
	}

	// read current balances into map
	balances := make(map[uuid.UUID]int64, len(ids))
	rows, err = dbtx.Query(ctx,
		`SELECT account_id, SUM(amount)::bigint FROM entries WHERE account_id = ANY($1) GROUP BY account_id`,
		ids,
	)
	if err != nil {
		return ledger.Transaction{}, fmt.Errorf("store: post transaction: read balances: %w", err)
	}

	var id uuid.UUID
	var balance int64

	_, err = pgx.ForEachRow(rows, []any{&id, &balance}, func() error {
		balances[id] = balance
		return nil
	})
	if err != nil {
		return ledger.Transaction{}, fmt.Errorf("store: post transaction: read balances: %w", err)
	}

	// check posting
	err = ledger.CheckPosting(tx, accounts, balances)
	if err != nil {
		return ledger.Transaction{}, err
	}

	err = dbtx.QueryRow(ctx,
		`INSERT INTO transactions (id, description, posted_at) VALUES ($1, $2, $3) RETURNING seq`,
		tx.ID, tx.Description, tx.PostedAt,
	).Scan(&tx.Seq)
	if err != nil {
		return ledger.Transaction{}, fmt.Errorf("store: post transaction: insert transaction: %w", err)
	}

	for _, entry := range tx.Entries {
		_, err = dbtx.Exec(ctx,
			`INSERT INTO entries (id, transaction_id, account_id, amount) VALUES ($1, $2, $3, $4)`,
			entry.ID, tx.ID, entry.AccountID, entry.Amount,
		)
		if err != nil {
			return ledger.Transaction{}, fmt.Errorf("store: post transaction: insert entry: %w", err)
		}
	}

	if err := dbtx.Commit(ctx); err != nil {
		return ledger.Transaction{}, fmt.Errorf("store: post transaction: commit: %w", err)
	}

	return tx, nil
}
