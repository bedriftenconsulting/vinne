UPDATE admin_users 
SET password_hash = '$2a$10$JZ3smvPJy4spBNVTp2kgZOWpKS2s0A0YUWOtJ3josvLezj0cCluFK'
WHERE username IN ('superadmin', 'surajadmin');

SELECT username, email, LENGTH(password_hash) as hash_length 
FROM admin_users;
