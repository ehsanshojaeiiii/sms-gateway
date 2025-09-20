-- Seed script for demo client
-- This creates a demo client with API key "secret" and some initial credits

-- Insert demo client
INSERT INTO clients (
    id, 
    name, 
    api_key_hash, 
    credit_cents,
    dlr_callback_url,
    callback_hmac_secret
) VALUES (
    '550e8400-e29b-41d4-a716-446655440000',
    'Demo Client',
    '$2a$10$N9qo8uLOickgx2ZMRZoMye/6lrVqaOZFJl.p6pznXiKlrDVrF.6Vi', -- bcrypt hash of "secret"
    100000, -- 1000.00 in cents
    'https://httpbin.org/post',
    'demo-hmac-secret-key'
) ON CONFLICT (api_key_hash) DO NOTHING;

-- Insert second client (plaintext key for demo: "user2")
INSERT INTO clients (
    id,
    name,
    api_key_hash,
    credit_cents
) VALUES (
    '660e8400-e29b-41d4-a716-446655440000',
    'User Two',
    'user2',
    5000
) ON CONFLICT (api_key_hash) DO NOTHING;

-- Display the created client
SELECT 
    id,
    name,
    credit_cents,
    dlr_callback_url,
    created_at
FROM clients 
WHERE name = 'Demo Client';
