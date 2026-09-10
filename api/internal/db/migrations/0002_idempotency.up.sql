CREATE TABLE idempotency_keys (
    merchant_id TEXT NOT NULL REFERENCES merchants (id),
    key TEXT NOT NULL CHECK (char_length(key) BETWEEN 1 AND 255),
    request_hash TEXT NOT NULL,
    payment_id TEXT NOT NULL REFERENCES payments (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (merchant_id, key)
);
