CREATE TABLE credit_locks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id uuid NOT NULL REFERENCES clients(id),
    message_id uuid NOT NULL REFERENCES messages(id),
    amount_cents bigint NOT NULL,
    state text NOT NULL CHECK (state IN ('HELD', 'CAPTURED', 'RELEASED')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_credit_locks_client_id ON credit_locks (client_id);
CREATE INDEX idx_credit_locks_message_id ON credit_locks (message_id);
CREATE INDEX idx_credit_locks_state ON credit_locks (state);
