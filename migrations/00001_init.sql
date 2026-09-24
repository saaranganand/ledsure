-- +goose Up
-- intentionally a no-op: keeps go:embed happy and proves the migration path actually runs
SELECT 1;

-- +goose Down
SELECT 1;
