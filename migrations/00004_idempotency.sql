-- +goose Up
CREATE TABLE idempotency_keys (
    key           TEXT PRIMARY KEY CHECK (char_length(key) BETWEEN 1 AND 255),
    request_hash  BYTEA NOT NULL,
    -- null only inside the transaction that claimed the key, which no one else can see
    status_code   INT,
    -- json keeps the exact bytes
    response_body JSON,
    created_at    TIMESTAMPTZ NOT NULL,
    CONSTRAINT idempotency_keys_response_complete CHECK ((status_code IS NULL) = (response_body IS NULL))
);

-- +goose Down
DROP TABLE idempotency_keys;
