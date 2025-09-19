CREATE TABLE idempotency_keys (
    client_id uuid NOT NULL,
    key text NOT NULL,
    message_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (client_id, key)
);
