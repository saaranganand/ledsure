//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestStoreMigrate(t *testing.T) {
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("ledsure"),
		postgres.WithUsername("ledsure"),
		postgres.WithPassword("ledsure"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	s, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(s.Close)

	if err := RunMigrations(ctx, s); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	var version int64
	row := s.pool.QueryRow(ctx, `SELECT version_id FROM goose_db_version ORDER BY id DESC LIMIT 1`)
	if err := row.Scan(&version); err != nil {
		t.Fatalf("query goose_db_version: %v", err)
	}
	if version != 1 {
		t.Errorf("goose_db_version = %d, want 1", version)
	}

	// running again should be a no-op: no pending migrations, no error
	if err := RunMigrations(ctx, s); err != nil {
		t.Fatalf("RunMigrations (second call): %v", err)
	}
}
