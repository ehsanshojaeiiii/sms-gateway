CREATE TABLE messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id uuid NOT NULL REFERENCES clients(id),
    to_msisdn text NOT NULL,
    from_sender text NOT NULL,
    text text NOT NULL,
    parts int NOT NULL,
    status text NOT NULL CHECK (status IN ('QUEUED', 'SENDING', 'SENT', 'DELIVERED', 'FAILED_TEMP', 'FAILED_PERM', 'CANCELLED')),
    client_reference text,
    provider text,
    provider_message_id text,
    attempts int NOT NULL DEFAULT 0,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_messages_client_id_created_at ON messages (client_id, created_at);
CREATE INDEX idx_messages_status ON messages (status);
CREATE INDEX idx_messages_provider_message_id ON messages (provider_message_id) WHERE provider_message_id IS NOT NULL;
