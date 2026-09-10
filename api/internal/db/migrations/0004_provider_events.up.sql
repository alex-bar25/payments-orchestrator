CREATE TABLE provider_events (
    id TEXT PRIMARY KEY,
    request_hash TEXT NOT NULL,
    payment_id TEXT NOT NULL REFERENCES payments (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
