-- Stage 2 command ledger. Apply to an isolated/test database before exercising
-- the new command path. The application does not apply this file automatically.
BEGIN;
-- Existing matches and legacy combat paths may have memory-only result events.
-- Keep them explicitly incomplete until every producer is migrated or a
-- verified snapshot is persisted by a protected command.
ALTER TABLE matches ADD COLUMN IF NOT EXISTS combat_result_events JSONB NOT NULL DEFAULT '{"damageEvents":[],"killEvents":[],"defeatedUsers":[]}'::jsonb;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS combat_results_complete BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS finished_at TIMESTAMPTZ;
CREATE TABLE IF NOT EXISTS match_commands (
    instance_id TEXT NOT NULL REFERENCES matches(instance_id) ON DELETE CASCADE,
    command_id TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    payload_hash TEXT NOT NULL,
    response JSONB NOT NULL,
    state_revision BIGINT NOT NULL,
    map_revision BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (instance_id, command_id)
);
CREATE INDEX IF NOT EXISTS match_commands_created_at_idx ON match_commands (created_at);
COMMIT;
-- Finished matches and command rows remain for retries. A later retention job
-- may remove them only after the retry window and result-history retention are
-- defined (see STAGE2_NOTES.md).
