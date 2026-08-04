-- 000023_crm_notes.down.sql — reverse the planner CRM migration.

DROP INDEX IF EXISTS idx_itin_req_sla_breach;
ALTER TABLE itinerary_requests DROP COLUMN IF EXISTS sla_notified_at;
DROP TABLE IF EXISTS crm_notes;
