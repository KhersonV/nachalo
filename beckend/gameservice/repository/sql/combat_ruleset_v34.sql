-- Schema only: old and non-HEX matches default to legacy. No inventories,
-- characters, templates, instances, recipes or item stats are rewritten.
ALTER TABLE matches
 ADD COLUMN IF NOT EXISTS combat_ruleset TEXT NOT NULL DEFAULT 'legacy'
 CHECK (combat_ruleset IN ('legacy','experimental_v34')),
 ADD COLUMN IF NOT EXISTS combat_config JSONB NOT NULL DEFAULT '{}'::jsonb,
 ADD COLUMN IF NOT EXISTS combat_rng_seed BIGINT NOT NULL DEFAULT 0,
 ADD COLUMN IF NOT EXISTS combat_rng_counter BIGINT NOT NULL DEFAULT 0 CHECK(combat_rng_counter>=0);

ALTER TABLE match_players
 ADD COLUMN IF NOT EXISTS combat_started BOOLEAN NOT NULL DEFAULT false,
 ADD COLUMN IF NOT EXISTS combat_magic_shield INTEGER NOT NULL DEFAULT 0 CHECK(combat_magic_shield>=0);
