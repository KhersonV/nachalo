\connect game_db

CREATE TABLE IF NOT EXISTS monsters (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    health INT NOT NULL,
    max_health INT NOT NULL,
    attack INT NOT NULL,
    defense INT NOT NULL,
    speed INT NOT NULL,
    maneuverability INT NOT NULL,
    vision INT NOT NULL,
    image TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_monsters_name_unique ON monsters(name);

CREATE TABLE IF NOT EXISTS resources (
    id SERIAL PRIMARY KEY,
    type TEXT NOT NULL,
    description TEXT,
    effect TEXT,
    image TEXT
);

CREATE TABLE IF NOT EXISTS artifacts (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    bonus JSONB NOT NULL DEFAULT '{}'::jsonb,
    image TEXT
);

INSERT INTO monsters (
    name, type, health, max_health, attack, defense, speed, maneuverability, vision, image
)
SELECT 'Goblin', 'monster', 35, 35, 7, 3, 6, 6, 5, '/monsters/goblin.webp'
WHERE NOT EXISTS (SELECT 1 FROM monsters WHERE name = 'Goblin');

INSERT INTO monsters (
    name, type, health, max_health, attack, defense, speed, maneuverability, vision, image
)
SELECT 'Goblins', 'monster', 50, 50, 10, 4, 5, 5, 5, '/monsters/goblins.webp'
WHERE NOT EXISTS (SELECT 1 FROM monsters WHERE name = 'Goblins');

INSERT INTO monsters (
    name, type, health, max_health, attack, defense, speed, maneuverability, vision, image
)
SELECT 'Orc', 'monster', 75, 75, 14, 8, 3, 2, 4, '/monsters/orc.webp'
WHERE NOT EXISTS (SELECT 1 FROM monsters WHERE name = 'Orc');

INSERT INTO monsters (
    name, type, health, max_health, attack, defense, speed, maneuverability, vision, image
)
SELECT 'Troll', 'monster', 120, 120, 18, 12, 2, 1, 3, '/monsters/troll.webp'
WHERE NOT EXISTS (SELECT 1 FROM monsters WHERE name = 'Troll');

INSERT INTO resources (
    type, description, effect, image
)
SELECT 'food', 'Food restores 20 HP (not above max)', '{"health":20}', '/main_resources/food.webp'
WHERE NOT EXISTS (SELECT 1 FROM resources WHERE type = 'food');

INSERT INTO resources (
    type, description, effect, image
)
SELECT 'water', 'Water restores 5 energy (not above max)', '{"energy":5}', '/main_resources/water.webp'
WHERE NOT EXISTS (SELECT 1 FROM resources WHERE type = 'water');

INSERT INTO resources (
    type, description, effect, image
)
SELECT 'wood', 'Wood for early base construction', '{"material_wood":1}', '/main_resources/wood.webp'
WHERE NOT EXISTS (SELECT 1 FROM resources WHERE type = 'wood');

INSERT INTO resources (
    type, description, effect, image
)
SELECT 'stone', 'Stone for early base construction', '{"material_stone":1}', '/main_resources/stone.webp'
WHERE NOT EXISTS (SELECT 1 FROM resources WHERE type = 'stone');

INSERT INTO resources (
    type, description, effect, image
)
SELECT 'iron', 'Iron for early base construction', '{"material_iron":1}', '/main_resources/iron.webp'
WHERE NOT EXISTS (SELECT 1 FROM resources WHERE type = 'iron');

INSERT INTO resources (
    id, type, description, effect, image
)
SELECT 6, 'barrel', 'Barrel loot container', '{}', '/main_resources/barrel.webp'
WHERE NOT EXISTS (SELECT 1 FROM resources WHERE id = 6);

UPDATE resources SET description = 'Barrel loot container', effect = '{}', image = '/main_resources/barrel.webp' WHERE type = 'barrel';
UPDATE resources SET description = 'Food restores 20 HP (not above max)', effect = '{"health":20}', image = '/main_resources/food.webp' WHERE type = 'food';
UPDATE resources SET description = 'Water restores 5 energy (not above max)', effect = '{"energy":5}', image = '/main_resources/water.webp' WHERE type = 'water';
UPDATE resources SET description = 'Wood for early base construction', effect = '{"material_wood":1}', image = '/main_resources/wood.webp' WHERE type = 'wood';
UPDATE resources SET description = 'Stone for early base construction', effect = '{"material_stone":1}', image = '/main_resources/stone.webp' WHERE type = 'stone';
UPDATE resources SET description = 'Iron for early base construction', effect = '{"material_iron":1}', image = '/main_resources/iron.webp' WHERE type = 'iron';

SELECT setval(
    'resources_id_seq',
    GREATEST((SELECT COALESCE(MAX(id), 1) FROM resources), 1),
    true
);

INSERT INTO artifacts (
    name, description, bonus, image
)
SELECT 'berserker_axe', 'Heavy axe that increases raw damage', '{"attack":8,"defense":-1}'::jsonb, '/artifacts/berserker-axe.webp'
WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'berserker_axe');

INSERT INTO artifacts (
    name, description, bonus, image
)
SELECT 'boots_of_stealth', 'Silent boots improving mobility and scouting', '{"speed":2,"maneuverability":3}'::jsonb, '/artifacts/boots_of_stealth.webp'
WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'boots_of_stealth');

INSERT INTO artifacts (
    name, description, bonus, image
)
SELECT 'crown_of_enlightenment', 'Crown that sharpens battlefield awareness', '{"vision":3}'::jsonb, '/artifacts/crown-of-enlightenment.webp'
WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'crown_of_enlightenment');

INSERT INTO artifacts (
    name, description, bonus, image
)
SELECT 'dragon_eye', 'Rare relic balancing offense and awareness', '{"attack":3,"vision":2}'::jsonb, '/artifacts/dragon-eye.webp'
WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'dragon_eye');

INSERT INTO artifacts (
    name, description, bonus, image
)
SELECT 'fire_amulet', 'Amulet that empowers aggressive style', '{"attack":5}'::jsonb, '/artifacts/fire-amulet.webp'
WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'fire_amulet');

INSERT INTO artifacts (
    name, description, bonus, image
)
SELECT 'gloves_of_precision', 'Precision gloves for cleaner strikes', '{"attack":2,"maneuverability":2}'::jsonb, '/artifacts/gloves-of-precision.webp'
WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'gloves_of_precision');

INSERT INTO artifacts (
    name, description, bonus, image
)
SELECT 'guardian_shield', 'Sturdy shield focused on survival', '{"defense":6,"speed":-1}'::jsonb, '/artifacts/guardian-shield.webp'
WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'guardian_shield');

INSERT INTO artifacts (
    name, description, bonus, image
)
SELECT 'knight_sword', 'Reliable sword for balanced combat', '{"attack":4,"defense":1}'::jsonb, '/artifacts/knight-sword.webp'
WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'knight_sword');

INSERT INTO artifacts (
    name, description, bonus, image
)
SELECT 'ring_of_wisdom', 'Ring improving tactical vision and control', '{"vision":2,"maneuverability":1}'::jsonb, '/artifacts/ring-of-wisdom.webp'
WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'ring_of_wisdom');

INSERT INTO artifacts (
    name, description, bonus, image
)
SELECT 'titan_breastplate', 'Massive armor with top-tier protection', '{"defense":9,"speed":-2}'::jsonb, '/artifacts/titan-breastplate.webp'
WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'titan_breastplate');

DELETE FROM artifacts WHERE name = 'ancient_amulet';

ALTER TABLE monsters OWNER TO admin;
ALTER TABLE resources OWNER TO admin;
ALTER TABLE artifacts OWNER TO admin;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE monsters TO admin;
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE resources TO admin;
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE artifacts TO admin;
GRANT USAGE, SELECT ON SEQUENCE monsters_id_seq TO admin;
GRANT USAGE, SELECT ON SEQUENCE resources_id_seq TO admin;
GRANT USAGE, SELECT ON SEQUENCE artifacts_id_seq TO admin;