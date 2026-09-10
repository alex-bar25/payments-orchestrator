CREATE TABLE merchants (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO merchants (id, name) VALUES ('merch_demo', 'Demo Merchant');

CREATE TABLE payments (
    id TEXT PRIMARY KEY,
    merchant_id TEXT NOT NULL REFERENCES merchants (id),
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency CHAR(3) NOT NULL,
    status TEXT NOT NULL CHECK (status IN (
        'requires_payment',
        'processing',
        'authorized',
        'captured',
        'refunded',
        'failed',
        'cancelled'
    )),
    version INT NOT NULL DEFAULT 1 CHECK (version > 0),
    provider_payment_id TEXT UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX payments_merchant_created_idx ON payments (merchant_id, created_at DESC);

CREATE TABLE payment_events (
    id TEXT PRIMARY KEY,
    payment_id TEXT NOT NULL REFERENCES payments (id),
    type TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    request_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX payment_events_payment_created_idx ON payment_events (payment_id, created_at);
