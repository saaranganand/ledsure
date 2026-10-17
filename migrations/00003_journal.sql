-- +goose Up
CREATE TABLE transactions (
    id          UUID PRIMARY KEY,
    seq         BIGSERIAL NOT NULL UNIQUE,
    description TEXT NOT NULL,
    posted_at   TIMESTAMPTZ NOT NULL
);

CREATE TABLE entries (
    id             UUID PRIMARY KEY,
    transaction_id UUID NOT NULL REFERENCES transactions (id),
    account_id     UUID NOT NULL REFERENCES accounts (id),
    amount         BIGINT NOT NULL CHECK (amount <> 0)
);

-- balance and history reads filter by account
CREATE INDEX entries_account_id_transaction_id_idx ON entries (account_id, transaction_id);
-- loading a transaction's entries
CREATE INDEX entries_transaction_id_idx ON entries (transaction_id);

-- +goose StatementBegin
CREATE FUNCTION reject_journal_change() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION '% is append-only: % is not allowed', TG_TABLE_NAME, TG_OP;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER transactions_append_only BEFORE UPDATE OR DELETE ON transactions
    FOR EACH ROW EXECUTE FUNCTION reject_journal_change();
CREATE TRIGGER transactions_no_truncate BEFORE TRUNCATE ON transactions
    FOR EACH STATEMENT EXECUTE FUNCTION reject_journal_change();

CREATE TRIGGER entries_append_only BEFORE UPDATE OR DELETE ON entries
    FOR EACH ROW EXECUTE FUNCTION reject_journal_change();
CREATE TRIGGER entries_no_truncate BEFORE TRUNCATE ON entries
    FOR EACH STATEMENT EXECUTE FUNCTION reject_journal_change();

-- +goose Down
DROP TABLE entries;
DROP TABLE transactions;
DROP FUNCTION reject_journal_change();
