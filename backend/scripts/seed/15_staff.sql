-- 15_staff.sql — staff users for the Planner CRM (PRD §3.3.2, §3.4.1)
-- =============================================================================
-- Two staff users so the planner CRM can be exercised live:
--   planner@jingdezhen.test  (travel_planner)   — id ...003
--   cs@jingdezhen.test        (customer_service)  — id ...004
-- Both carry password "password123" (hash reused from the customer seed).
-- Fixed UUIDs for reproducible assignment + note authorship in tests.
--
-- Roles are seeded by migration 000002_rbac (idempotent). user_roles is the
-- join table; the upserts are idempotent (ON CONFLICT on the user_id+role_id
-- unique-ish — we use a per-pair DELETE+INSERT to stay simple).
-- =============================================================================

BEGIN;

INSERT INTO users (id, nickname, email, password_hash, is_active, auth_provider, preferred_locale, preferred_currency, created_at, updated_at)
VALUES
    ('00000000-0000-0000-0000-000000000003',
     'Test Planner',
     'planner@jingdezhen.test',
     '$2a$10$5Z9E1vghW0OM0xMakIMcoep1Uj2ll8QY5Dq5iQejdA09c5gfOQB3K', -- "password123"
     TRUE, 'email', 'en-US', 'USD', NOW(), NOW()),
    ('00000000-0000-0000-0000-000000000004',
     'Test CS Agent',
     'cs@jingdezhen.test',
     '$2a$10$5Z9E1vghW0OM0xMakIMcoep1Uj2ll8QY5Dq5iQejdA09c5gfOQB3K',
     TRUE, 'email', 'en-US', 'USD', NOW(), NOW())
ON CONFLICT (email) DO UPDATE SET
    nickname = EXCLUDED.nickname,
    password_hash = EXCLUDED.password_hash,
    is_active = EXCLUDED.is_active,
    updated_at = NOW();

-- Assign roles (idempotent: clear then insert for each pair).
DELETE FROM user_roles WHERE user_id IN ('00000000-0000-0000-0000-000000000003','00000000-0000-0000-0000-000000000004');

INSERT INTO user_roles (user_id, role_id)
SELECT '00000000-0000-0000-0000-000000000003', id FROM roles WHERE key = 'travel_planner'
ON CONFLICT DO NOTHING;
INSERT INTO user_roles (user_id, role_id)
SELECT '00000000-0000-0000-0000-000000000004', id FROM roles WHERE key = 'customer_service'
ON CONFLICT DO NOTHING;

COMMIT;
