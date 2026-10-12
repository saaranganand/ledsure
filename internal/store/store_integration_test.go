//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/saaranganand/ledsure/migrations"
)

func TestStoreMigrate(t *testing.T) {
	ctx := context.Background()

	s := newTestStore(t)

	// var version int64
	// row := s.pool.QueryRow(ctx, `SELECT version_id FROM goose_db_version ORDER BY id DESC LIMIT 1`)
	// if err := row.Scan(&version); err != nil {
	// 	t.Fatalf("query goose_db_version: %v", err)
	// }
	// if version != 1 {
	// 	t.Errorf("goose_db_version = %d, want 1", version)
	// }

	db := stdlib.OpenDBFromPool(s.pool)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("close migration db handle: %v", err)
		}
	})

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		t.Fatalf("create migration provider: %v", err)
	}
	pending, err := provider.HasPending(ctx)
	if err != nil {
		t.Fatalf("check pending migrations: %v", err)
	}
	if pending {
		t.Error("migrations still pending after RunMigrations")
	}

	var systemAccounts int
	row := s.pool.QueryRow(ctx, `SELECT count(*) FROM accounts WHERE kind = 'system'`)
	if err := row.Scan(&systemAccounts); err != nil {
		t.Fatalf("count system accounts: %v", err)
	}
	if systemAccounts != 4 {
		t.Errorf("system accounts = %d, want 4", systemAccounts)
	}

	// running again should be a no-op: no pending migrations, no error
	if err := RunMigrations(ctx, s); err != nil {
		t.Fatalf("RunMigrations (second call): %v", err)
	}
}
