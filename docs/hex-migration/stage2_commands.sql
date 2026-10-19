-- Stage 2 command ledger. Apply to an isolated/test database before exercising
-- the new command path. The application does not apply this file automatically.
BEGIN;
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
-- Rows disappear with the match. Operators may additionally delete rows for
-- inactive matches older than their retry window (see STAGE2_NOTES.md).
