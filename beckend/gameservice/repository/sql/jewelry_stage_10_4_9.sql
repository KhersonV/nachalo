-- Applied only by the normal startup schema registration.
CREATE TABLE IF NOT EXISTS artifact_dismantle_operations (
 user_id INTEGER NOT NULL REFERENCES player_profiles(user_id) ON DELETE CASCADE,
 command_id TEXT NOT NULL CHECK(length(command_id) BETWEEN 8 AND 128),
 artifact_id INTEGER NOT NULL,
 quantity INTEGER NOT NULL CHECK(quantity BETWEEN 1 AND 100),
 response JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,command_id)
);
CREATE TABLE IF NOT EXISTS storage_purchase_operations (
 user_id INTEGER NOT NULL REFERENCES player_profiles(user_id) ON DELETE CASCADE,
 command_id TEXT NOT NULL CHECK(length(command_id) BETWEEN 8 AND 128),
 item_key TEXT NOT NULL,
 quantity INTEGER NOT NULL,
 total_cost INTEGER NOT NULL,
 PRIMARY KEY(user_id,command_id)
);
-- Trading transfers existing UUIDs. Every first creation of a jewelry item must be crafted.
CREATE OR REPLACE FUNCTION require_jewelry_craft_source() RETURNS trigger AS $$
BEGIN
 IF NEW.source <> 'craft' AND EXISTS (
  SELECT 1 FROM item_templates t JOIN item_sets s ON s.id=t.set_id
  WHERE t.id=NEW.template_id AND left(s.code,8)='jewelry_'
 ) THEN RAISE EXCEPTION 'Jewelry is craft-only'; END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS item_instances_jewelry_craft_only ON item_instances;
CREATE TRIGGER item_instances_jewelry_craft_only BEFORE INSERT ON item_instances
FOR EACH ROW EXECUTE FUNCTION require_jewelry_craft_source();
