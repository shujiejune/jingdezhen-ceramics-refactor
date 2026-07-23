-- 10_users.sql — test customer
-- =============================================================================
-- A normal customer: is_active=true, no user_roles row (per the RBAC design —
-- customers are users WITHOUT a role assignment). Fixed UUID for reproducible
-- wishlist/orders later.
--
-- Password is "password123" (bcrypt hash reused from the existing admin test
-- user for consistency). Idempotent upsert so re-seeding is safe.
-- =============================================================================

BEGIN;

INSERT INTO users (id, nickname, email, password_hash, is_active, auth_provider, preferred_locale, preferred_currency, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    'Test Customer',
    'customer@jingdezhen.test',
    '$2a$10$5Z9E1vghW0OM0xMakIMcoep1Uj2ll8QY5Dq5iQejdA09c5gfOQB3K', -- "password123"
    TRUE,
    'email',
    'en-US',
    'USD',
    NOW(), NOW()
)
ON CONFLICT (email) DO UPDATE SET
    nickname = EXCLUDED.nickname,
    password_hash = EXCLUDED.password_hash,
    is_active = EXCLUDED.is_active,
    preferred_locale = EXCLUDED.preferred_locale,
    preferred_currency = EXCLUDED.preferred_currency,
    updated_at = NOW();

COMMIT;
