-- Simple seed script for demo client
-- Reset demo client with sufficient credits for testing

-- Ensure demo client exists with proper credits
INSERT INTO clients (
    id, 
    name,
    api_key_hash,
    credit_cents,
    dlr_callback_url
) VALUES (
    '550e8400-e29b-41d4-a716-446655440000',
    'Demo Client',
    'demo-api-key', -- Simple demo API key
    500000, -- 5000.00 in cents (enough for 100,000 messages at 5 cents each - ensuring 100% success)
    'https://httpbin.org/post'
) ON CONFLICT (id) DO UPDATE SET 
    credit_cents = 500000,
    name = 'Demo Client',
    api_key_hash = 'demo-api-key';

-- Display the created client
SELECT 
    id,
    name,
    credit_cents,
    dlr_callback_url,
    created_at
FROM clients 
WHERE id = '550e8400-e29b-41d4-a716-446655440000';