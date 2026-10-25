-- Stage 10.1. Optional manual equivalent of the existing startup initialization.
-- Do not run against an older schema before the base/match tables exist.
BEGIN;
ALTER TABLE player_base_buildings
    ADD COLUMN IF NOT EXISTS guild_level INTEGER NOT NULL DEFAULT 0;
ALTER TABLE match_players
    ADD COLUMN IF NOT EXISTS profession_unlocked BOOLEAN NOT NULL DEFAULT false;
-- No backfill from current Guild ownership: existing matches must remain ineligible.
-- No inventory, material, profession progress or existing building IDs are modified.
COMMIT;
