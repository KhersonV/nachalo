-- Stage 10.2. Optional manual equivalent of startup initialization. Not executed.
BEGIN;
ALTER TABLE match_players ADD COLUMN IF NOT EXISTS profession_gather_count INTEGER NOT NULL DEFAULT 0 CHECK (profession_gather_count >= 0);
LOCK TABLE resources IN SHARE ROW EXCLUSIVE MODE;
SELECT setval(pg_get_serial_sequence('resources','id'),GREATEST(COALESCE((SELECT MAX(id) FROM resources),1),(SELECT last_value FROM resources_id_seq)),true);
INSERT INTO resources(type,description,effect,image,resource_kind,tradable,market_category,market_subcategory)
SELECT seed.code,seed.description,'{}',seed.image,'resource',true,'resource','profession_material'
FROM (VALUES
('copper_ore','Copper-bearing stone for future metalworking recipes.','/professions/gathering/ore.svg'),
('iron_ore','Dense iron-bearing ore for future metalworking recipes.','/professions/gathering/ore.svg'),
('crystalline_ore','Rare mineral crystals for future advanced recipes.','/professions/gathering/ore.svg'),
('sturdy_bark','Tough bark for future protective materials.','/professions/gathering/forest.svg'),
('forest_fiber','Flexible woodland fibers for future bindings and fabrics.','/professions/gathering/forest.svg'),
('resinous_wood','Rare resin-rich wood for future treated materials.','/professions/gathering/forest.svg'),
('bone_fragments','Bone pieces for future carved materials and powders.','/professions/gathering/remains.svg'),
('strong_tendons','Durable sinews for future bindings and reinforced materials.','/professions/gathering/remains.svg'),
('ancient_fang','A rare ancient trophy for future advanced carving recipes.','/professions/gathering/remains.svg'),
('moon_grass','Pale magical grass for future alchemical preparations.','/professions/gathering/herbs.svg'),
('cave_mushroom','A cave fungus for future alchemical reagents.','/professions/gathering/herbs.svg'),
('ethereal_flower','A rare luminous flower for future advanced alchemy.','/professions/gathering/herbs.svg')
) AS seed(code,description,image)
WHERE NOT EXISTS(SELECT 1 FROM resources r WHERE r.type=seed.code);
-- Existing maps, inventories, profession XP and resource IDs are untouched.
COMMIT;
