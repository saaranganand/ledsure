package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/saaranganand/ledsure/internal/ledger"
)

var ErrNotFound = errors.New("not found")

const accountColumns = `id, name, currency, kind, code, allow_negative, created_at`

func (s *Store) CreateAccount(ctx context.Context, a ledger.Account) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO accounts (`+accountColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		a.ID, a.Name, a.Currency, a.Kind, a.Code, a.AllowNegative, a.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("store: create account: %w", err)
	}
	return nil
}

func (s *Store) GetAccount(ctx context.Context, id uuid.UUID) (ledger.Account, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+accountColumns+` FROM accounts WHERE id = $1`, id)
	if err != nil {
		return ledger.Account{}, fmt.Errorf("store: get account: %w", err)
	}
	a, err := pgx.CollectExactlyOneRow(rows, scanAccount)
	if errors.Is(err, pgx.ErrNoRows) {
		return ledger.Account{}, ErrNotFound
	}
	if err != nil {
		return ledger.Account{}, fmt.Errorf("store: get account: %w", err)
	}
	return a, nil
}

// normalise to match clock
func scanAccount(row pgx.CollectableRow) (ledger.Account, error) {
	var a ledger.Account
	err := row.Scan(&a.ID, &a.Name, &a.Currency, &a.Kind, &a.Code, &a.AllowNegative, &a.CreatedAt)
	a.CreatedAt = a.CreatedAt.UTC()
	return a, err
}

func (s *Store) ListSystemAccounts(ctx context.Context) ([]ledger.Account, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+accountColumns+` FROM accounts WHERE kind = 'system' ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("store: list system accounts: %w", err)
	}
	accounts, err := pgx.CollectRows(rows, scanAccount)
	if err != nil {
		return nil, fmt.Errorf("store: list system accounts: %w", err)
	}
	return accounts, nil
}
