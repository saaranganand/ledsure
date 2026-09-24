package store

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/saaranganand/ledsure/migrations"
)

// applies pending goose migrations, safe to call every startup
// shares the existing pgx pool instead of opening a second connection
func RunMigrations(ctx context.Context, s *Store) error {
	db := stdlib.OpenDBFromPool(s.pool)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("store: close migration db handle", "error", closeErr)
		}
	}()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("store: create migration provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("store: apply migrations: %w", err)
	}

	for _, r := range results {
		slog.Info("migration applied",
			"version", r.Source.Version,
			"duration", r.Duration.String(),
		)
	}
	return nil
}
