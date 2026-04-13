-- Create a test agent
-- Agent Code: AGT-2026-000001
-- Phone: 233200000001
-- Password: Agent@123!

INSERT INTO agents_auth (
    id,
    agent_code,
    phone,
    email,
    password_hash,
    is_active,
    created_at,
    updated_at
) VALUES (
    'a1111111-1111-1111-1111-111111111111',
    'AGT-2026-000001',
    '233200000001',
    'testagent@randlottery.com',
    '$2a$10$JZ3smvPJy4spBNVTp2kgZOWpKS2s0A0YUWOtJ3josvLezj0cCluFK', -- Password: Agent@123!
    TRUE,
    NOW(),
    NOW()
) ON CONFLICT (agent_code) DO NOTHING;

-- Verify the agent was created
SELECT agent_code, phone, email, is_active FROM agents_auth WHERE agent_code = 'AGT-2026-000001';
