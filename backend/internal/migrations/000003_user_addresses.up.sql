-- =============================================================================
-- 000003_user_addresses: shipping address book (PRD §3.5, TDD §3.4)
--
-- Multiple shipping addresses per user. `country` (ISO 3166-1 alpha-2) drives
-- the shipping-fee calculator (PRD §3.2.3). At most one default per user is
-- enforced by a partial unique index; the service layer unsets the previous
-- default in a transaction when a new one is chosen.
-- =============================================================================

CREATE TABLE user_addresses (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient   VARCHAR(255) NOT NULL,            -- name on the parcel
    line1       VARCHAR(255) NOT NULL,
    line2       VARCHAR(255),
    city        VARCHAR(100) NOT NULL,
    region      VARCHAR(100),                     -- state / province
    postal_code VARCHAR(30),
    country     CHAR(2) NOT NULL,                 -- ISO 3166-1 alpha-2 (drives shipping calc)
    phone       VARCHAR(30),
    is_default  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_addresses_user_id ON user_addresses (user_id);

-- At most one default address per user.
CREATE UNIQUE INDEX idx_user_addresses_default_per_user
    ON user_addresses (user_id) WHERE is_default = TRUE;
