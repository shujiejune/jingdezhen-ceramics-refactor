DROP TABLE IF EXISTS artist_translations CASCADE;
ALTER TABLE artists DROP COLUMN IF EXISTS avatar_url;
ALTER TABLE artists DROP COLUMN IF EXISTS display_order;
