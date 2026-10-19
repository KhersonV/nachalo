-- HEX Ranger state. Apply only after Stage 1 and Stage 2 on an isolated test DB.
BEGIN;
ALTER TABLE match_monsters ADD COLUMN IF NOT EXISTS hex_armor_break_stacks INTEGER NOT NULL DEFAULT 0;
ALTER TABLE match_monsters ADD COLUMN IF NOT EXISTS hex_armor_break_turns INTEGER NOT NULL DEFAULT 0;
COMMIT;
