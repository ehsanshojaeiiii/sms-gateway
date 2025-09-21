-- Seed script to create 100 test clients for massive load testing
-- Using a more compact approach with generate_series

INSERT INTO clients (id, name, credit_cents, api_key_hash, dlr_callback_url, created_at) 
SELECT 
    ('550e8400-e29b-41d4-a716-4466544' || LPAD(i::text, 4, '0'))::uuid,
    'Load Test Client ' || LPAD(i::text, 3, '0'),
    50000, -- 50,000 credits = $500 per client
    'hash_load_' || LPAD(i::text, 3, '0'),
    'https://httpbin.org/post',
    NOW()
FROM generate_series(1, 100) AS i
ON CONFLICT (id) DO NOTHING;

-- Show summary of created clients
SELECT COUNT(*) as total_load_clients, SUM(credit_cents) as total_credits_cents 
FROM clients WHERE name LIKE 'Load Test Client%';

-- Show first 5 and last 5 clients
(SELECT id, name, credit_cents FROM clients WHERE name LIKE 'Load Test Client%' ORDER BY name LIMIT 5)
UNION ALL
(SELECT id, name, credit_cents FROM clients WHERE name LIKE 'Load Test Client%' ORDER BY name DESC LIMIT 5)
ORDER BY name;