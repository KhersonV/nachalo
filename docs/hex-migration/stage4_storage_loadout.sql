-- Apply manually to the game database before enabling HEX loadouts.
-- The existing player_profiles.inventory remains the account Storage.
ALTER TABLE player_profiles
  ADD COLUMN IF NOT EXISTS hex_loadout JSONB NOT NULL DEFAULT '{}'::jsonb;
