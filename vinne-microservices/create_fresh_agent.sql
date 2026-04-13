-- Delete old agent and create fresh one
DELETE FROM agents_auth WHERE agent_code = 'AGT-2026-000001';

-- Create fresh agent with known password
-- Password: Admin@123!
INSERT INTO agents_auth (
    id,
    agent_code,
    phone,
    email,
    password_hash,
    is_active
) VALUES (
    'b2222222-2222-2222-2222-222222222222',
    'AGT-2026-000001',
    '+233200000001',
    'testagent@randlottery.com',
    '$2a$10$JZ3smvPJy4spBNVTp2kgZOWpKS2s0A0YUWOtJ3josvLezj0cCluFK',
    TRUE
);

-- Verify
SELECT agent_code, phone, email, LENGTH(password_hash) as hash_len, is_active 
FROM agents_auth 
WHERE agent_code = 'AGT-2026-000001';
