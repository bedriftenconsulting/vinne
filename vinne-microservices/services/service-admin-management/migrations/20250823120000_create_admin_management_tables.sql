-- +goose Up
-- +goose StatementBegin

-- Create admin_users table
CREATE TABLE IF NOT EXISTS admin_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    mfa_secret VARCHAR(255),
    mfa_enabled BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    last_login TIMESTAMP,
    last_login_ip VARCHAR(45),
    ip_whitelist TEXT[],
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    version INT DEFAULT 1
);

-- Create admin_roles table
CREATE TABLE IF NOT EXISTS admin_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) UNIQUE NOT NULL,
    description TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- Create permissions table
CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(resource, action)
);

-- Create role_permissions junction table
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id UUID NOT NULL REFERENCES admin_roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, permission_id)
);

-- Create admin_user_roles junction table
CREATE TABLE IF NOT EXISTS admin_user_roles (
    user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES admin_roles(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

-- Create admin_sessions table for session management
CREATE TABLE IF NOT EXISTS admin_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    refresh_token VARCHAR(500) UNIQUE NOT NULL,
    user_agent TEXT NOT NULL,
    ip_address VARCHAR(45) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    is_active BOOLEAN DEFAULT TRUE
);

-- Create admin_audit_logs table for user management operations
CREATE TABLE IF NOT EXISTS admin_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id UUID NOT NULL REFERENCES admin_users(id),
    action VARCHAR(100) NOT NULL,
    resource VARCHAR(100),
    resource_id VARCHAR(255),
    ip_address VARCHAR(45) NOT NULL,
    user_agent TEXT NOT NULL,
    request_data JSONB,
    response_status INT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX idx_admin_users_email ON admin_users(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_admin_users_username ON admin_users(username) WHERE deleted_at IS NULL;
CREATE INDEX idx_admin_users_active ON admin_users(is_active) WHERE deleted_at IS NULL;
CREATE INDEX idx_admin_sessions_user_id ON admin_sessions(user_id);
CREATE INDEX idx_admin_sessions_refresh_token ON admin_sessions(refresh_token);
CREATE INDEX idx_admin_sessions_expires_at ON admin_sessions(expires_at) WHERE is_active = TRUE;
CREATE INDEX idx_admin_audit_logs_user_id ON admin_audit_logs(admin_user_id);
CREATE INDEX idx_admin_audit_logs_created_at ON admin_audit_logs(created_at DESC);
CREATE INDEX idx_admin_audit_logs_action ON admin_audit_logs(action);

-- Insert default roles
INSERT INTO admin_roles (id, name, description) VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'super_admin', 'Full system access with all permissions'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12', 'admin', 'Administrative access with most permissions'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13', 'manager', 'Management access for operations'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14', 'support', 'Customer support access'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15', 'viewer', 'Read-only access to system')
ON CONFLICT (name) DO NOTHING;

-- Insert default permissions
INSERT INTO permissions (resource, action, description) VALUES
    -- User management
    ('users', 'create', 'Create new users'),
    ('users', 'read', 'View user information'),
    ('users', 'update', 'Update user information'),
    ('users', 'delete', 'Delete users'),
    
    -- Game management
    ('games', 'create', 'Create new games'),
    ('games', 'read', 'View game information'),
    ('games', 'update', 'Update game settings'),
    ('games', 'delete', 'Delete games'),
    ('games', 'activate', 'Activate/deactivate games'),
    
    -- Draw management
    ('draws', 'create', 'Create new draws'),
    ('draws', 'read', 'View draw information'),
    ('draws', 'update', 'Update draw details'),
    ('draws', 'execute', 'Execute draws'),
    ('draws', 'cancel', 'Cancel draws'),
    
    -- Ticket management
    ('tickets', 'read', 'View ticket information'),
    ('tickets', 'cancel', 'Cancel tickets'),
    ('tickets', 'validate', 'Validate winning tickets'),
    
    -- Payment management
    ('payments', 'read', 'View payment information'),
    ('payments', 'process', 'Process payments'),
    ('payments', 'refund', 'Issue refunds'),
    
    -- Reports
    ('reports', 'read', 'View reports'),
    ('reports', 'export', 'Export reports'),
    
    -- System settings
    ('settings', 'read', 'View system settings'),
    ('settings', 'update', 'Update system settings'),
    
    -- Audit logs
    ('audit', 'read', 'View audit logs')
ON CONFLICT (resource, action) DO NOTHING;

-- Assign permissions to roles
-- Super Admin gets all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', id FROM permissions
ON CONFLICT DO NOTHING;

-- Admin gets most permissions (except system settings update)
INSERT INTO role_permissions (role_id, permission_id)
SELECT 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12', id FROM permissions
WHERE NOT (resource = 'settings' AND action = 'update')
ON CONFLICT DO NOTHING;

-- Manager gets operational permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13', id FROM permissions
WHERE resource IN ('games', 'draws', 'tickets', 'payments', 'reports')
  AND action IN ('read', 'update', 'execute', 'cancel', 'validate', 'export')
ON CONFLICT DO NOTHING;

-- Support gets customer service permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14', id FROM permissions
WHERE (resource IN ('users', 'tickets', 'payments') AND action = 'read')
   OR (resource = 'tickets' AND action IN ('cancel', 'validate'))
ON CONFLICT DO NOTHING;

-- Viewer gets read-only permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a15', id FROM permissions
WHERE action = 'read'
ON CONFLICT DO NOTHING;

-- Create a default super admin user (password: Admin@123!)
-- Note: In production, this should be changed immediately
INSERT INTO admin_users (
    id,
    email,
    username,
    password_hash,
    first_name,
    last_name,
    is_active
) VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a00',
    'superadmin@randco.com',
    'superadmin',
    '$2a$10$YourHashedPasswordHere', -- This should be properly hashed
    'Super',
    'Admin',
    TRUE
) ON CONFLICT (email) DO NOTHING;

-- Assign super_admin role to default user
INSERT INTO admin_user_roles (user_id, role_id)
VALUES ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a00', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11')
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop all tables in reverse dependency order
DROP TABLE IF EXISTS admin_user_roles CASCADE;
DROP TABLE IF EXISTS role_permissions CASCADE;
DROP TABLE IF EXISTS admin_audit_logs CASCADE;
DROP TABLE IF EXISTS admin_sessions CASCADE;
DROP TABLE IF EXISTS permissions CASCADE;
DROP TABLE IF EXISTS admin_roles CASCADE;
DROP TABLE IF EXISTS admin_users CASCADE;

-- +goose StatementEnd