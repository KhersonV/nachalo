-- Stage 1, manual migration only. Apply on a verified copy before production.
-- Existing rows retain x/y and legacy JSON; no coordinate conversion occurs.
BEGIN;

-- A second application fails immediately with a duplicate-key error.
CREATE TABLE IF NOT EXISTS hex_stage1_migration_marker (
  version integer PRIMARY KEY CHECK (version = 1)
);
INSERT INTO hex_stage1_migration_marker(version) VALUES (1);

ALTER TABLE matches ADD COLUMN IF NOT EXISTS schema_version integer NOT NULL DEFAULT 1;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS geometry jsonb;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS map_revision bigint NOT NULL DEFAULT 0;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS state_revision bigint NOT NULL DEFAULT 0;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS start_positions_v2 jsonb;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS portal_position_v2 jsonb;
ALTER TABLE matches ADD CONSTRAINT matches_geometry_version_check CHECK (
  (schema_version = 1 AND (geometry IS NULL OR geometry = '{"kind":"square","coordinates":"cartesian","version":1}'::jsonb)
    AND start_positions_v2 IS NULL AND portal_position_v2 IS NULL)
  OR (schema_version = 2 AND geometry IS NOT NULL AND geometry = '{"kind":"hex","coordinates":"axial","orientation":"pointy","version":1}'::jsonb
    AND start_positions_v2 IS NOT NULL AND portal_position_v2 IS NOT NULL)
);

-- New HEX rows use q/r; legacy square columns stay untouched on existing rows.
ALTER TABLE match_map_cells ALTER COLUMN x DROP NOT NULL;
ALTER TABLE match_map_cells ALTER COLUMN y DROP NOT NULL;
ALTER TABLE match_map_cells ADD COLUMN IF NOT EXISTS q integer;
ALTER TABLE match_map_cells ADD COLUMN IF NOT EXISTS r integer;
ALTER TABLE match_map_cells ADD CONSTRAINT match_map_cells_coordinates_check CHECK (
  (x IS NOT NULL AND y IS NOT NULL AND q IS NULL AND r IS NULL)
  OR (x IS NULL AND y IS NULL AND q IS NOT NULL AND r IS NOT NULL)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_match_map_cells_instance_qr
  ON match_map_cells(instance_id, q, r);

ALTER TABLE match_monsters ALTER COLUMN pos_x DROP NOT NULL;
ALTER TABLE match_monsters ALTER COLUMN pos_y DROP NOT NULL;
ALTER TABLE match_monsters ADD COLUMN IF NOT EXISTS pos_q integer;
ALTER TABLE match_monsters ADD COLUMN IF NOT EXISTS pos_r integer;
ALTER TABLE match_monsters ADD CONSTRAINT match_monsters_coordinates_check CHECK (
  (pos_x IS NOT NULL AND pos_y IS NOT NULL AND pos_q IS NULL AND pos_r IS NULL)
  OR (pos_x IS NULL AND pos_y IS NULL AND pos_q IS NOT NULL AND pos_r IS NOT NULL)
);

ALTER TABLE match_players ADD COLUMN IF NOT EXISTS position_q integer;
ALTER TABLE match_players ADD COLUMN IF NOT EXISTS position_r integer;
ALTER TABLE match_players ADD CONSTRAINT match_players_coordinates_check CHECK (
  (position IS NOT NULL AND position_q IS NULL AND position_r IS NULL)
  OR (position IS NULL AND position_q IS NOT NULL AND position_r IS NOT NULL)
);

-- direction 0..2 is the canonical half of the six axial edge directions.
CREATE TABLE IF NOT EXISTS match_hex_edges (
  instance_id text NOT NULL REFERENCES matches(instance_id) ON DELETE CASCADE,
  q integer NOT NULL,
  r integer NOT NULL,
  direction smallint NOT NULL CHECK (direction BETWEEN 0 AND 2),
  -- Directions 0=(+1,0), 1=(+1,-1), 2=(0,-1).
  neighbor_q integer GENERATED ALWAYS AS (q + CASE direction WHEN 0 THEN 1 WHEN 1 THEN 1 ELSE 0 END) STORED,
  neighbor_r integer GENERATED ALWAYS AS (r + CASE direction WHEN 0 THEN 0 WHEN 1 THEN -1 ELSE -1 END) STORED,
  wall_kind text NOT NULL CHECK (wall_kind <> ''),
  PRIMARY KEY (instance_id, q, r, direction),
  FOREIGN KEY (instance_id, q, r)
    REFERENCES match_map_cells(instance_id, q, r) ON DELETE CASCADE,
  FOREIGN KEY (instance_id, neighbor_q, neighbor_r)
    REFERENCES match_map_cells(instance_id, q, r) ON DELETE CASCADE
);

-- Prevent a square row from being inserted into a HEX match (and vice versa).
CREATE FUNCTION check_match_coordinate_version() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE version integer;
BEGIN
  SELECT schema_version INTO version FROM matches WHERE instance_id = NEW.instance_id;
  IF version IS NULL THEN RAISE EXCEPTION 'match % does not exist', NEW.instance_id; END IF;
  IF TG_TABLE_NAME = 'match_map_cells' THEN
    IF (version = 1 AND (NEW.x IS NULL OR NEW.y IS NULL OR NEW.q IS NOT NULL OR NEW.r IS NOT NULL))
       OR (version = 2 AND (NEW.q IS NULL OR NEW.r IS NULL OR NEW.x IS NOT NULL OR NEW.y IS NOT NULL)) THEN
      RAISE EXCEPTION 'cell coordinates do not match geometry of %', NEW.instance_id;
    END IF;
  ELSIF TG_TABLE_NAME = 'match_monsters' THEN
    IF (version = 1 AND (NEW.pos_x IS NULL OR NEW.pos_y IS NULL OR NEW.pos_q IS NOT NULL OR NEW.pos_r IS NOT NULL))
       OR (version = 2 AND (NEW.pos_q IS NULL OR NEW.pos_r IS NULL OR NEW.pos_x IS NOT NULL OR NEW.pos_y IS NOT NULL)) THEN
      RAISE EXCEPTION 'monster coordinates do not match geometry of %', NEW.instance_id;
    END IF;
  ELSIF TG_TABLE_NAME = 'match_players' THEN
    IF (version = 1 AND (NEW.position IS NULL OR NEW.position_q IS NOT NULL OR NEW.position_r IS NOT NULL))
       OR (version = 2 AND (NEW.position_q IS NULL OR NEW.position_r IS NULL OR NEW.position IS NOT NULL)) THEN
      RAISE EXCEPTION 'player coordinates do not match geometry of %', NEW.instance_id;
    END IF;
  ELSIF version <> 2 THEN
    RAISE EXCEPTION 'HEX edge belongs to non-HEX match %', NEW.instance_id;
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER geometry_cells_guard BEFORE INSERT OR UPDATE ON match_map_cells
  FOR EACH ROW EXECUTE FUNCTION check_match_coordinate_version();
CREATE TRIGGER geometry_monsters_guard BEFORE INSERT OR UPDATE ON match_monsters
  FOR EACH ROW EXECUTE FUNCTION check_match_coordinate_version();
CREATE TRIGGER geometry_players_guard BEFORE INSERT OR UPDATE ON match_players
  FOR EACH ROW EXECUTE FUNCTION check_match_coordinate_version();
CREATE TRIGGER geometry_edges_guard BEFORE INSERT OR UPDATE ON match_hex_edges
  FOR EACH ROW EXECUTE FUNCTION check_match_coordinate_version();

COMMIT;
