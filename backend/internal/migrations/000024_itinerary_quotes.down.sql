-- 000024_itinerary_quotes.down.sql — reverse the quote builder + deposit migration.

DROP INDEX IF EXISTS idx_payments_itin_quote_id;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_exactly_one_owner;
ALTER TABLE payments DROP COLUMN IF EXISTS itinerary_quote_id;
-- Restore order_id NOT NULL. Existing order rows all have order_id set; any
-- stray NULL (a deposit payment) would have been dropped with the quotes table
-- above (ON DELETE SET NULL, then the rows are orphans we don't expect here).
ALTER TABLE payments ALTER COLUMN order_id SET NOT NULL;

DROP INDEX IF EXISTS idx_itin_quotes_status;
DROP INDEX IF EXISTS idx_itin_quotes_request_id;
DROP TABLE IF EXISTS itinerary_quotes;
DROP TABLE IF EXISTS option_rates;
