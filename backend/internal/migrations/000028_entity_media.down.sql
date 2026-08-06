-- 000028_entity_media DOWN: drop the 3 entity gallery tables (reverse of up).
-- media_assets itself is untouched (it lives on; only the per-entity joins drop).
-- =============================================================================

DROP TABLE IF EXISTS activity_media;
DROP TABLE IF EXISTS ceramic_story_media;
DROP TABLE IF EXISTS artist_media;
