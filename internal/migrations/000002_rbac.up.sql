-- =============================================================================
-- 000002_rbac: Role-Based Access Control (PRD §3.4.1, TDD §3.4)
--
-- Replaces the single users.role string with a roles/permissions model.
-- Five fixed staff roles are code-seeded (no custom roles in v1). Customers
-- are users with NO row in user_roles. Super Administrator bypasses permission
-- checks in code (middleware.RequirePermission); it is also granted every
-- permission row here so the bypass is defence-in-depth.
-- =============================================================================

CREATE TABLE roles (
    id   BIGSERIAL PRIMARY KEY,
    key  VARCHAR(40) NOT NULL UNIQUE,          -- e.g. 'super_admin', 'content_editor'
    name VARCHAR(80) NOT NULL,                 -- human-readable
    is_staff BOOLEAN NOT NULL DEFAULT TRUE     -- staff roles only; customers have no role row
);

CREATE TABLE permissions (
    id   BIGSERIAL PRIMARY KEY,
    key  VARCHAR(60) NOT NULL UNIQUE,          -- e.g. 'content.publish', 'order.refund'
    description TEXT
);

CREATE TABLE role_permissions (
    role_id       BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_by UUID REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (user_id, role_id)             -- one assignment per (user, role)
);

CREATE INDEX idx_user_roles_user_id ON user_roles (user_id);

-- --- Seed roles (PRD §3.4.1) ---------------------------------------------------

INSERT INTO roles (key, name, is_staff) VALUES
    ('super_admin',        'Super Administrator', TRUE),
    ('content_editor',     'Content Editor',      TRUE),
    ('travel_planner',     'Travel Planner',      TRUE),
    ('ecommerce_operator', 'E-commerce Operator', TRUE),
    ('customer_service',   'Customer Service',    TRUE);

-- --- Seed permissions ----------------------------------------------------------

INSERT INTO permissions (key, description) VALUES
    ('users.manage',        'Manage staff accounts and assign roles'),
    ('content.write',       'Author and submit culture/travel content'),
    ('content.publish',     'Approve and publish content (Super Admin only in v1)'),
    ('product.read',        'View products and inventory'),
    ('product.write',       'Create/edit products, SKUs, bulk import, certificates'),
    ('order.read',          'View orders'),
    ('order.write',         'Update order status, enter tracking numbers'),
    ('order.refund',        'Issue full refunds'),
    ('itinerary.read',      'View itinerary requests in CRM inbox'),
    ('itinerary.write',     'Process requests, compose quotes, assign planners'),
    ('itinerary.confirm',   'Confirm itineraries and send PDF/confirmations'),
    ('chat.handle',         'Pick up live-chat sessions in the agent console'),
    ('dashboard.view',      'View the global data dashboard'),
    ('settings.manage',     'Edit platform settings (shipping fees, FX markup, rates)');

-- --- Role → permission mappings ------------------------------------------------

-- Super Administrator: every permission (also bypassed in code).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.key = 'super_admin';

-- Content Editor: author/submit; cannot publish (Super Admin approves).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.key = 'content_editor' AND p.key IN ('content.write');

-- Travel Planner: full itinerary lifecycle + chat + itinerary read.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.key = 'travel_planner'
  AND p.key IN ('itinerary.read','itinerary.write','itinerary.confirm','chat.handle','order.read');

-- E-commerce Operator: catalog + orders incl. refunds.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.key = 'ecommerce_operator'
  AND p.key IN ('product.read','product.write','order.read','order.write','order.refund','dashboard.view');

-- Customer Service: live chat + read-only order/itinerary lookup.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.key = 'customer_service'
  AND p.key IN ('chat.handle','order.read','itinerary.read','dashboard.view');

-- --- Drop the legacy users.role column ----------------------------------------
-- The Go code reads roles via user_roles from M0 onward. Existing dev DBs must
-- be reset (see AGENTS.md / REFACTOR-TODO) — no data migration is attempted.

ALTER TABLE users DROP COLUMN IF EXISTS role;
