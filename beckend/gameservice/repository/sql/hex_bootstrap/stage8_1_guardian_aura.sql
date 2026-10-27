-- HEX bootstrap migration; transaction/once guard owned by bootstrap.go.
-- Persistent Guardian aura pressure is required across HEX moves and turns.
ALTER TABLE match_players ADD COLUMN IF NOT EXISTS hex_guardian_aura_pressure INTEGER NOT NULL DEFAULT 0;
ALTER TABLE match_players ADD COLUMN IF NOT EXISTS hex_guardian_aura_source_user_id INTEGER NOT NULL DEFAULT 0;
