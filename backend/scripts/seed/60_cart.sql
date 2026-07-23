-- 60_cart.sql — customer's shopping cart (PRD §3.2.3, TDD §3.4)
-- =============================================================================
-- One server-side cart per signed-in user. The customer (00000000-...0002)
-- has 2 items in their cart: the vase (sku 1, qty 2) and the tea set (sku 2,
-- qty 1). Exercises the cart read path (locale-aware JOIN, CNY totals).
-- Stock is NOT decremented at the cart stage (advisory guard only).
-- =============================================================================

BEGIN;

-- Ensure a cart row exists for the customer (idempotent).
INSERT INTO carts (user_id)
VALUES ('00000000-0000-0000-0000-000000000002')
ON CONFLICT (user_id) DO UPDATE SET updated_at = NOW();

-- Cart items keyed on (cart_id, sku_id). Additive ON CONFLICT lets re-seeding
-- preserve an operator-edited qty if needed.
INSERT INTO cart_items (cart_id, sku_id, qty)
SELECT c.id, v.sku_id, v.qty
FROM carts c
JOIN (VALUES
    (1::bigint, 2::int),
    (2::bigint, 1::int)
) AS v(sku_id, qty) ON TRUE
ON CONFLICT (cart_id, sku_id) DO UPDATE SET qty = EXCLUDED.qty, updated_at = NOW();

COMMIT;
