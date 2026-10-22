-- Apply manually before deploying Stage 10 backend code.
ALTER TABLE match_players ADD COLUMN IF NOT EXISTS hex_disconnected_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS match_players_hex_disconnected_idx
  ON match_players (hex_disconnected_at)
  WHERE hex_disconnected_at IS NOT NULL;
