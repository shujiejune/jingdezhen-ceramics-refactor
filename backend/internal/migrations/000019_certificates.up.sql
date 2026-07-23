-- =============================================================================
-- 000019_certificates: digital certificates + provenance (PRD §3.2.1/§5.4, TDD §3.4/§8)
--
-- Each product gets one certificate (product_id UNIQUE) with a unique cert_code
-- + QR target. Auto-generated at product creation; operators can regenerate.
-- Provenance records append the authenticity chain: `created` at issue, `sold`
-- at order-paid (TDD §8), `transferred` later. Public QR target page:
-- GET /certificates/:code (no auth, PRD §3.2.1).
--
-- qr_key/pdf_key are nullable OSS object keys — the QR is served on-demand
-- (GET /certificates/:code/qr) until the storage adapter lands; the printable
-- PDF is deferred pending the TDD §12 engine decision.
--
-- Reserved blockchain integration (PRD §5.4): the certificate service calls a
-- certchain.Chain adapter at issue/sale; Noop for v1 (a vendor plugs in later).
-- =============================================================================

CREATE TABLE certificates (
    id          BIGSERIAL PRIMARY KEY,
    product_id  BIGINT NOT NULL UNIQUE REFERENCES products(id) ON DELETE CASCADE,
    cert_code   VARCHAR(32) NOT NULL UNIQUE,   -- JDZ-<6-base32>, URL-safe
    qr_key      TEXT,                           -- OSS object key (nullable; on-demand until storage lands)
    pdf_key     TEXT,                           -- OSS object key (nullable; deferred pending PDF engine)
    issued_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_certificates_cert_code ON certificates (cert_code);

CREATE TABLE provenance_records (
    id             BIGSERIAL PRIMARY KEY,
    certificate_id BIGINT NOT NULL REFERENCES certificates(id) ON DELETE CASCADE,
    kind           VARCHAR(20) NOT NULL CHECK (kind IN ('created','sold','transferred')),
    detail         JSONB,                         -- e.g. {"order_id":N,"regenerated":true}
    at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_provenance_cert_at ON provenance_records (certificate_id, at);

-- --- RBAC: certificate.manage permission (E-commerce Operators + Super Admin) ---
INSERT INTO permissions (key, description)
VALUES ('certificate.manage', 'List/view/regenerate product certificates')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.key IN ('super_admin','ecommerce_operator') AND p.key = 'certificate.manage'
ON CONFLICT DO NOTHING;
