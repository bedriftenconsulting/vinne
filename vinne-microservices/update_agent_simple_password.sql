-- Update agent with simple test password
-- Password: Test123!
UPDATE agents_auth 
SET password_hash = '$2a$10$SdpvAIY9S7ir8LhVBNPDL.y0ffVxIJkYYiz3PqJ/Eov0tjl9Tv.MK',
    failed_login_attempts = 0,
    locked_until = NULL
WHERE agent_code = 'AGT-2026-000001';

-- Verify
SELECT agent_code, phone, email, LENGTH(password_hash) as hash_len, failed_login_attempts, is_active 
FROM agents_auth 
WHERE agent_code = 'AGT-2026-000001';
