CREATE TABLE provider_ledger (
    payment_id TEXT PRIMARY KEY REFERENCES payments (id),
    provider_payment_id TEXT,
    status TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE discrepancies (
    id TEXT PRIMARY KEY,
    payment_id TEXT NOT NULL REFERENCES payments (id),
    internal_status TEXT NOT NULL,
    provider_status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT discrepancies_pair UNIQUE (payment_id, internal_status, provider_status)
);
