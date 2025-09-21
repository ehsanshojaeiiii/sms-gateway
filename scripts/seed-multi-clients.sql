-- Create multiple test clients for load testing
-- Each client gets 50,000 credits (enough for 10,000 messages at 5 credits each)

INSERT INTO clients (id, name, api_key_hash, credit_cents, dlr_callback_url) VALUES
('11111111-1111-1111-1111-111111111111', 'Load Test Client 01', 'test-key-01', 50000, 'https://httpbin.org/post'),
('22222222-2222-2222-2222-222222222222', 'Load Test Client 02', 'test-key-02', 50000, 'https://httpbin.org/post'),
('33333333-3333-3333-3333-333333333333', 'Load Test Client 03', 'test-key-03', 50000, 'https://httpbin.org/post'),
('44444444-4444-4444-4444-444444444444', 'Load Test Client 04', 'test-key-04', 50000, 'https://httpbin.org/post'),
('55555555-5555-5555-5555-555555555555', 'Load Test Client 05', 'test-key-05', 50000, 'https://httpbin.org/post'),
('66666666-6666-6666-6666-666666666666', 'Load Test Client 06', 'test-key-06', 50000, 'https://httpbin.org/post'),
('77777777-7777-7777-7777-777777777777', 'Load Test Client 07', 'test-key-07', 50000, 'https://httpbin.org/post'),
('88888888-8888-8888-8888-888888888888', 'Load Test Client 08', 'test-key-08', 50000, 'https://httpbin.org/post'),
('99999999-9999-9999-9999-999999999999', 'Load Test Client 09', 'test-key-09', 50000, 'https://httpbin.org/post'),
('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'Load Test Client 10', 'test-key-10', 50000, 'https://httpbin.org/post');

-- Verify clients were created
SELECT 'Created clients:' as info;
SELECT id, name, credit_cents FROM clients WHERE name LIKE 'Load Test Client%' ORDER BY name;
