-- HEX bootstrap migration; transaction/once guard owned by bootstrap.go.
-- Apply manually after stage5_quest_artifact.sql. One durable outcome per
-- participant; the match command ledger independently prevents command replay.
ALTER TABLE matches
    ADD COLUMN IF NOT EXISTS hex_reward_events_ready BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS hex_player_results (
    instance_id TEXT NOT NULL REFERENCES matches(instance_id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL,
    result JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (instance_id, user_id),
    FOREIGN KEY (instance_id, user_id) REFERENCES match_players(instance_id, user_id) ON DELETE CASCADE
);
