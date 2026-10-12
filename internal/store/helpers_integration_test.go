//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// new db per test
func newTestStore(t *testing.T) *Store {
	t.Helper()
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
	return s
}
