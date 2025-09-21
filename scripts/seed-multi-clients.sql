-- Seed script to create 10 test clients for multi-client load testing
-- Each client gets 50,000 credits (500 USD) which is enough for extensive testing

INSERT INTO clients (id, name, credit_cents, api_key_hash, dlr_callback_url, created_at) VALUES
('550e8400-e29b-41d4-a716-446655440001', 'Test Client 1', 50000, 'hash_client_001', 'https://httpbin.org/post', NOW()),
('550e8400-e29b-41d4-a716-446655440002', 'Test Client 2', 50000, 'hash_client_002', 'https://httpbin.org/post', NOW()),
('550e8400-e29b-41d4-a716-446655440003', 'Test Client 3', 50000, 'hash_client_003', 'https://httpbin.org/post', NOW()),
('550e8400-e29b-41d4-a716-446655440004', 'Test Client 4', 50000, 'hash_client_004', 'https://httpbin.org/post', NOW()),
('550e8400-e29b-41d4-a716-446655440005', 'Test Client 5', 50000, 'hash_client_005', 'https://httpbin.org/post', NOW()),
('550e8400-e29b-41d4-a716-446655440006', 'Test Client 6', 50000, 'hash_client_006', 'https://httpbin.org/post', NOW()),
('550e8400-e29b-41d4-a716-446655440007', 'Test Client 7', 50000, 'hash_client_007', 'https://httpbin.org/post', NOW()),
('550e8400-e29b-41d4-a716-446655440008', 'Test Client 8', 50000, 'hash_client_008', 'https://httpbin.org/post', NOW()),
('550e8400-e29b-41d4-a716-446655440009', 'Test Client 9', 50000, 'hash_client_009', 'https://httpbin.org/post', NOW()),
('550e8400-e29b-41d4-a716-446655440010', 'Test Client 10', 50000, 'hash_client_010', 'https://httpbin.org/post', NOW())
ON CONFLICT (id) DO NOTHING;

-- Show the created clients
SELECT id, name, credit_cents FROM clients WHERE name LIKE 'Test Client%' ORDER BY name;
