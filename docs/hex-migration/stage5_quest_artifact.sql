-- Apply manually after stage4_storage_loadout.sql. No existing match is altered
-- beyond the default marker; legacy HEX matches with no quest_artifact_id stay unchanged.
ALTER TABLE matches
    ADD COLUMN IF NOT EXISTS hex_quest_spawned BOOLEAN NOT NULL DEFAULT FALSE;
