-- Structural migration only. Catalog rows are registered by EnsureCraftingSchema.
ALTER TABLE item_templates ADD COLUMN IF NOT EXISTS pocket_bonus INTEGER NOT NULL DEFAULT 0 CHECK (pocket_bonus BETWEEN 0 AND 6);
UPDATE item_templates SET pocket_bonus=CASE WHEN rarity IN ('gray','green') THEN 1 WHEN rarity='blue' THEN 2 ELSE 0 END WHERE slot IN ('chest','pants') AND pocket_bonus=0;
ALTER TABLE match_players ADD COLUMN IF NOT EXISTS craft_status JSONB NOT NULL DEFAULT '{}'::jsonb;
CREATE TABLE IF NOT EXISTS profession_craft_operations (
 user_id INTEGER NOT NULL REFERENCES player_profiles(user_id) ON DELETE CASCADE,
 command_id TEXT NOT NULL,
 character_id INTEGER NOT NULL REFERENCES player_characters(id) ON DELETE CASCADE,
 recipe_code TEXT NOT NULL,
 quantity INTEGER NOT NULL CHECK(quantity BETWEEN 1 AND 100),
 response JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,command_id)
);
