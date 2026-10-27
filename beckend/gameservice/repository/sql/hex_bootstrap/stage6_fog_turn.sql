-- HEX bootstrap migration; transaction/once guard owned by bootstrap.go.
-- Apply after Stage 5 before starting the Stage 6 server.
ALTER TABLE match_players ADD COLUMN IF NOT EXISTS hex_explored JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE match_players ADD COLUMN IF NOT EXISTS hex_armor_break_stacks INTEGER NOT NULL DEFAULT 0;
ALTER TABLE match_players ADD COLUMN IF NOT EXISTS hex_armor_break_turns INTEGER NOT NULL DEFAULT 0;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS hex_turn_deadline TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS matches_hex_turn_deadline_idx ON matches(hex_turn_deadline) WHERE finished_at IS NULL;
