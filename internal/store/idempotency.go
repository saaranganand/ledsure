package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrKeyReused = errors.New("idempotency key reused with a different request")

type IdempotentResponse struct {
	Status int
	Body []byte
}

func (s *Store) Idempotent(ctx context.Context, key string, requestHash []byte, now time.Time, 
	fn func(ctx context.Context) IdempotentResponse) (IdempotentResponse, error) {
	dbtx, err := s.pool.Begin(ctx)
	if err!=nil {
		return IdempotentResponse{}, fmt.Errorf("store: idempotent: begin: %w", err)
	}
	defer func() { _ = dbtx.Rollback(ctx) }()

	tag, err := s.pool.Exec(ctx, 
		`INSERT INTO idempotency_keys (key, request_hash, created_at) VALUES ($1, $2, $3)
		ON CONFLICT (key) DO NOTHING`,
		key, requestHash, now,
	)
	if err != nil {
		return IdempotentResponse{}, fmt.Errorf("storeL: idempotent: claim key: %w", err)
	}

	if tag.RowsAffected() == 0 {
		var storedHash []byte
		var stored IdempotentResponse
		err := dbtx.QueryRow(ctx,
			`SELECT request_hash, status_code, response_body::text
			FROM idempotency_keys WHERE key = $1`,
			key,
		).Scan(&storedHash, &stored.Status, &stored.Body)
		if err!=nil {
			return IdempotentResponse{}, fmt.Errorf("store: idempotent: load stored response: %w", err)
		}
		if !bytes.Equal(storedHash, requestHash) {
			return IdempotentResponse{}, ErrKeyReused
		}
		return stored, nil
	}

	resp := fn(withTx(ctx, dbtx))

	if resp.Status >= 500 {
		return resp, nil
	}

	_, err = dbtx.Exec(ctx,
		`UPDATE idempotency_keys SET status_code = $2, response_body = $3 WHERE key = $1`,
		key, resp.Status, string(resp.Body),
	)
	if err != nil {
		return IdempotentResponse{}, fmt.Errorf("store: idempotent: store response: %w", err)
	}
	if err := dbtx.Commit(ctx); err != nil {
		return IdempotentResponse{}, fmt.Errorf("store: idempotent: commit: %w", err)
	}
	
	return resp, nil
}