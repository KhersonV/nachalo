-- AP/MP are integer HALF points in storage and JSON: 2 = 1 AP/MP.
ALTER TABLE match_players
 ADD COLUMN IF NOT EXISTS ap INTEGER NOT NULL DEFAULT 6 CHECK(ap BETWEEN 0 AND 6),
 ADD COLUMN IF NOT EXISTS max_ap INTEGER NOT NULL DEFAULT 6 CHECK(max_ap=6),
 ADD COLUMN IF NOT EXISTS mp INTEGER NOT NULL DEFAULT 0 CHECK(mp BETWEEN 0 AND 16),
 ADD COLUMN IF NOT EXISTS max_mp INTEGER NOT NULL DEFAULT 0 CHECK(max_mp BETWEEN 0 AND 16),
 ADD COLUMN IF NOT EXISTS reactions INTEGER NOT NULL DEFAULT 1 CHECK(reactions BETWEEN 0 AND 2),
 ADD COLUMN IF NOT EXISTS max_reactions INTEGER NOT NULL DEFAULT 1 CHECK(max_reactions BETWEEN 1 AND 2),
 ADD COLUMN IF NOT EXISTS personal_turn_counter INTEGER NOT NULL DEFAULT 0,
 ADD COLUMN IF NOT EXISTS water_available_turn INTEGER NOT NULL DEFAULT 0,
 ADD COLUMN IF NOT EXISTS hex_mp_drain BOOLEAN NOT NULL DEFAULT false,
 ADD COLUMN IF NOT EXISTS hex_mystic_drain_round INTEGER NOT NULL DEFAULT -1,
 ADD COLUMN IF NOT EXISTS hex_overload_target TEXT NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS hex_overload_count INTEGER NOT NULL DEFAULT 0,
 ADD COLUMN IF NOT EXISTS hex_mystic_reflex_used BOOLEAN NOT NULL DEFAULT false,
 ADD COLUMN IF NOT EXISTS hex_aura_slow_used BOOLEAN NOT NULL DEFAULT false,
 ADD COLUMN IF NOT EXISTS hex_first_step_used BOOLEAN NOT NULL DEFAULT false,
 ADD COLUMN IF NOT EXISTS hex_applied_artifact TEXT NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS hex_armor_break_owner INTEGER NOT NULL DEFAULT 0;
ALTER TABLE match_monsters ADD COLUMN IF NOT EXISTS hex_armor_break_owner INTEGER NOT NULL DEFAULT 0;

-- Known catalog entries are explicitly converted by equipment_seed.go before this initializer.
-- Unknown energy-bearing templates must be reviewed rather than silently losing their bonus.
DO $stage9$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='stage9_item_no_energy' AND conrelid='item_templates'::regclass) THEN
  ALTER TABLE item_templates ADD CONSTRAINT stage9_item_no_energy CHECK(max_energy_bonus=0);
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='stage9_set_no_energy' AND conrelid='item_set_bonuses'::regclass) THEN
  ALTER TABLE item_set_bonuses ADD CONSTRAINT stage9_set_no_energy CHECK(max_energy_bonus=0);
 END IF;
END $stage9$;
