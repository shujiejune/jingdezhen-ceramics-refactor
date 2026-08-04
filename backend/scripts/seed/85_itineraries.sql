-- 85_itineraries.sql — Custom Itinerary Builder seed (PRD §3.3.2)
-- =============================================================================
-- Two submitted requests for the customer user (one pending, one cancelled —
-- exercises the customer-facing read + cancel paths) + one in-progress draft.
-- The planner CRM tables (quotes/option_rates/crm_notes) are not seeded here
-- (they belong to the CRM sub-track).
-- =============================================================================

BEGIN;

INSERT INTO itinerary_requests (
    id, user_id, status, arrival_date, duration_days, flexible, adults, children,
    interests, budget, pace, services, contact, notes, locale, sla_deadline, submitted_at
) VALUES
    (1, '00000000-0000-0000-0000-000000000002', 'pending',
     '2026-09-15', 5, FALSE, 2, 0,
     '["pottery-workshop","kiln-heritage-sites","local-food"]'::jsonb,
     '{"currency":"USD","min_minor":100000,"max_minor":200000}'::jsonb,
     'balanced',
     '{"guide":"english","hotel":true,"hotel_level":"comfort","pickup":true,"experience":true,"dietary_accessibility":""}'::jsonb,
     '{"channel":"email","whatsapp_number":"","notes":"First time in Jingdezhen"}'::jsonb,
     'Would love a studio visit with a celadon artist if possible.',
     'en-US', NOW() + INTERVAL '24 hours', NOW() - INTERVAL '2 hours'),
    (2, '00000000-0000-0000-0000-000000000002', 'cancelled',
     '2026-10-01', 3, TRUE, 1, 1,
     '["museums","ceramic-shopping"]'::jsonb,
     '{"currency":"EUR","min_minor":80000,"max_minor":120000}'::jsonb,
     'relaxed',
     '{"guide":"none","hotel":true,"hotel_level":"budget","pickup":false,"experience":false,"dietary_accessibility":"vegetarian"}'::jsonb,
     '{"channel":"whatsapp","whatsapp_number":"+441234567890","notes":""}'::jsonb,
     NULL,
     'en-US', NOW() + INTERVAL '24 hours', NOW() - INTERVAL '5 days')
ON CONFLICT (id) DO UPDATE SET
    status = EXCLUDED.status, arrival_date = EXCLUDED.arrival_date,
    duration_days = EXCLUDED.duration_days, flexible = EXCLUDED.flexible,
    adults = EXCLUDED.adults, children = EXCLUDED.children,
    interests = EXCLUDED.interests, budget = EXCLUDED.budget, pace = EXCLUDED.pace,
    services = EXCLUDED.services, contact = EXCLUDED.contact, notes = EXCLUDED.notes,
    locale = EXCLUDED.locale, sla_deadline = EXCLUDED.sla_deadline,
    submitted_at = EXCLUDED.submitted_at,
    cancel_reason = EXCLUDED.cancel_reason, cancelled_at = EXCLUDED.cancelled_at;

-- Mark the cancelled one with a reason + timestamp.
UPDATE itinerary_requests SET cancel_reason = 'Plans changed', cancelled_at = NOW() - INTERVAL '4 days'
WHERE id = 2;

-- Assign seeded request #1 to the planner (exercises the planner-CRM inbox +
-- assignment). id ...003 is seeded in 15_staff.sql.
UPDATE itinerary_requests SET assigned_to = '00000000-0000-0000-0000-000000000003'
WHERE id = 1;

-- One internal planner note on request #1 (contact history, PRD §3.3.2).
INSERT INTO crm_notes (request_id, author_id, body)
SELECT 1, '00000000-0000-0000-0000-000000000003',
       'Customer is interested in a celadon studio visit — reached out to Artist Li to check availability for Sep 15.'
WHERE NOT EXISTS (SELECT 1 FROM crm_notes WHERE request_id = 1);

SELECT setval(pg_get_serial_sequence('itinerary_requests', 'id'),
              COALESCE((SELECT MAX(id) FROM itinerary_requests), 0) + 1, false);

-- One in-progress draft for the customer (step 2 of 4).
INSERT INTO itinerary_drafts (user_id, form_state, step) VALUES
    ('00000000-0000-0000-0000-000000000002',
     '{"arrival_date":"2026-11-10","duration_days":7,"flexible":false,"adults":2,"children":0,"interests":["countryside-sanbao","photography"]}'::jsonb,
     2)
ON CONFLICT (user_id) DO UPDATE SET form_state = EXCLUDED.form_state, step = EXCLUDED.step, updated_at = NOW();

COMMIT;
