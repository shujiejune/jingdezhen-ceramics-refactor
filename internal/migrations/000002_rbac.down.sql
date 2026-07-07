-- Reverse of 000002_rbac.

-- Restore the legacy role column (empty; data is not recoverable).
ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'normal_user';
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('guest', 'normal_user', 'admin'));

DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
