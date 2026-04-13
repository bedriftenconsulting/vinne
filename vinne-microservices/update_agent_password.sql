-- Update agent password
-- Password: Admin@123!
UPDATE agents_auth 
SET password_hash = '$2a$10$JZ3smvPJy4spBNVTp2kgZOWpKS2s0A0YUWOtJ3josvLezj0cCluFK'
WHERE agent_code = 'AGT-2026-000001';

-- Verify
SELECT agent_code, phone, LENGTH(password_hash) as hash_length 
FROM agents_auth 
WHERE agent_code = 'AGT-2026-000001';
