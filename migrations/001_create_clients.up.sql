CREATE TABLE clients (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    api_key_hash text NOT NULL UNIQUE,
    dlr_callback_url text,
    callback_hmac_secret text,
    credit_cents bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
