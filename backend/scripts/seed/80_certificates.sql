-- 80_certificates.sql — digital certificates for the 5 seeded products
-- =============================================================================
-- Each product gets one certificate (product_id UNIQUE) + a `created` provenance
-- record. The cert_code is JDZ-<6-base32>; collision-avoided by ON CONFLICT
-- (the seed is idempotent; codes are deterministic for reproducibility).
-- Exercisable via GET /certificates/:code (public QR target) +
-- GET /certificates/:code/qr (PNG).
-- =============================================================================

BEGIN;

-- Idempotent: insert a certificate for each product that doesn't have one.
-- Deterministic codes (JDZ-<productID zero-padded>) so the seed is stable.
INSERT INTO certificates (product_id, cert_code)
SELECT p.id, 'JDZ-' || lpad(p.id::text, 6, '0')
FROM products p
WHERE NOT EXISTS (SELECT 1 FROM certificates c WHERE c.product_id = p.id)
ON CONFLICT (product_id) DO NOTHING;

-- Append a `created` provenance record for each newly-seeded certificate
-- (idempotent: skip if a `created` row already exists for the cert).
INSERT INTO provenance_records (certificate_id, kind, detail)
SELECT c.id, 'created', '{"seeded": true}'::jsonb
FROM certificates c
WHERE NOT EXISTS (
    SELECT 1 FROM provenance_records pr
    WHERE pr.certificate_id = c.id AND pr.kind = 'created'
)
ON CONFLICT DO NOTHING;

COMMIT;
