-- 000025_itinerary_quote_pdf: itinerary quote PDF (TDD §12, M3 #4 follow-up)
--
-- The chromedp PDF adapter (commit 0725971) renders a branded itinerary quote
-- PDF at quote-send time + stores it via the storage adapter. This adds the
-- nullable pdf_key column itinerary_quotes was missing (scoped out of 000024).
-- Mirrors certificates.pdf_key (000019): NULL until the worker renders → Set
-- via UPDATE; re-renders overwrite.
ALTER TABLE itinerary_quotes ADD COLUMN pdf_key TEXT;
