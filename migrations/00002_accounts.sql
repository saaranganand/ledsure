-- +goose Up
CREATE TABLE accounts (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL CHECK (name <> ''),
    currency CHAR(3) NOT NULL CHECK (currency IN ('CAD','USD')),
    kind TEXT NOT NULL CHECK (kind IN ('customer','system')),
    code TEXT UNIQUE,
    allow_negative BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL,
    -- system accts addressed by code, customer accts arent
    CONSTRAINT accounts_code_only_for_system CHECK ((kind = 'system') = (code IS NOT NULL))
);

INSERT INTO accounts (id, name, currency, kind, code, allow_negative, created_at) VALUES
    ('00000000-0000-7000-8000-000000000001', 'External bank (CAD)', 'CAD', 'system', 'SYSTEM_EXTERNAL_BANK_CAD', true,  now()),
    ('00000000-0000-7000-8000-000000000002', 'Fee revenue (CAD)', 'CAD', 'system', 'SYSTEM_FEE_REVENUE_CAD', false, now()),
    ('00000000-0000-7000-8000-000000000003', 'External bank (USD)', 'USD', 'system', 'SYSTEM_EXTERNAL_BANK_USD', true,  now()),
    ('00000000-0000-7000-8000-000000000004', 'Fee revenue (USD)', 'USD', 'system', 'SYSTEM_FEE_REVENUE_USD', false, now());

-- +goose Down
DROP TABLE accounts;