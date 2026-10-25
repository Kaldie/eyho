-- Local and demo listings (feature 002, T004). Local and demo environments ONLY.
--
-- Amsterdam listings that match the frontend's former mock data, owned by the two demo cooks from
-- demo_accounts.sql (run that file first). Pickup windows are relative to now(), so the listings
-- are claimable every time the stack starts. Idempotent: re-running refreshes the windows and
-- re-publishes the listings but never touches portions or claims.
--
-- Runs from the `seed` service in docker-compose.yml, not from the migrate step, so production
-- migrations never carry demo data.

BEGIN;

INSERT INTO listings (
    id, cook_id, food_description, allergen_info, photo_url,
    total_portions, portions_remaining, reduced_price_cents, retail_reference_price_cents,
    pickup_location, pickup_address_text, pickup_window_start, pickup_window_end, status
) VALUES
    -- Buurtkeuken Jordaan (non-profit, unverified): shown first in discovery, flagged "unverified".
    ('00000000-0000-4000-8000-0000000a0001', '00000000-0000-4000-8000-0000000c0003',
     'Vegetable lasagne (3 portions)', 'Gluten, Dairy',
     'https://images.unsplash.com/photo-1547592166-23ac45744acd?w=600&auto=format&fit=crop&q=60',
     3, 3, 350, 1100,
     ST_SetSRID(ST_MakePoint(4.8830, 52.3760), 4326)::geography, 'Egelantiersgracht 40, 1015 RE Amsterdam',
     NOW() - INTERVAL '15 minutes', NOW() + INTERVAL '2 hours 30 minutes', 'published'),
    ('00000000-0000-4000-8000-0000000a0003', '00000000-0000-4000-8000-0000000c0003',
     'Chickpea curry with rice', '',
     NULL,
     6, 6, 300, 1000,
     ST_SetSRID(ST_MakePoint(4.9100, 52.3640), 4326)::geography, 'Javastraat 41, 1094 Amsterdam',
     NOW() - INTERVAL '15 minutes', NOW() + INTERVAL '2 hours', 'published'),
    -- Keuken Katrien (professional, verified).
    ('00000000-0000-4000-8000-0000000a0002', '00000000-0000-4000-8000-0000000c0002',
     'Lentil soup with crusty bread', 'Gluten',
     NULL,
     5, 5, 200, 700,
     ST_SetSRID(ST_MakePoint(4.8895, 52.3702), 4326)::geography, 'Keizersgracht 88, 1015 CT Amsterdam',
     NOW() + INTERVAL '30 minutes', NOW() + INTERVAL '3 hours', 'published'),
    ('00000000-0000-4000-8000-0000000a0004', '00000000-0000-4000-8000-0000000c0002',
     'Pumpkin soup with sourdough', 'Gluten',
     NULL,
     4, 4, 250, 800,
     ST_SetSRID(ST_MakePoint(4.8895, 52.3702), 4326)::geography, 'Keizersgracht 88, 1015 CT Amsterdam',
     NOW() + INTERVAL '1 hour', NOW() + INTERVAL '4 hours', 'published')
ON CONFLICT (id) DO UPDATE
SET pickup_window_start = EXCLUDED.pickup_window_start,
    pickup_window_end   = EXCLUDED.pickup_window_end,
    status              = 'published',
    updated_at          = NOW();

COMMIT;
