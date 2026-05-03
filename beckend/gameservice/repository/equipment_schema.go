package repository

import "log"

const equipmentSlotsCheck = "slot IN ('main_hand', 'off_hand', 'helmet', 'chest', 'pants', 'boots', 'gloves', 'ring', 'amulet')"

// CreateEquipmentTables creates the database foundation for future equipment.
//
// TODO(equipment): equip, unequip, trading, selling and deleting must be done in
// one transaction. Lock the item_instances row with SELECT ... FOR UPDATE,
// verify owner_user_id/status/current_character_id/version, update version, and
// append an item_instance_events row before commit. Two-handed weapons should be
// equipped only into main_hand; the equip service must treat off_hand as
// logically blocked without inserting a duplicate character_equipment row.
func CreateEquipmentTables() {
	query := `
	CREATE EXTENSION IF NOT EXISTS pgcrypto;

	CREATE TABLE IF NOT EXISTS item_sets (
		id BIGSERIAL PRIMARY KEY,
		code TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		class_restriction TEXT NOT NULL,
		rarity TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		CONSTRAINT item_sets_class_restriction_check CHECK (class_restriction IN ('guardian', 'berserker', 'ranger', 'mystic')),
		CONSTRAINT item_sets_rarity_check CHECK (rarity IN ('green', 'blue', 'purple', 'orange')),
		CONSTRAINT item_sets_code_not_blank_check CHECK (btrim(code) <> ''),
		CONSTRAINT item_sets_name_not_blank_check CHECK (btrim(name) <> '')
	);

	CREATE TABLE IF NOT EXISTS item_templates (
		id BIGSERIAL PRIMARY KEY,
		code TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		set_id BIGINT REFERENCES item_sets(id) ON DELETE SET NULL,
		slot TEXT NOT NULL,
		item_type TEXT NOT NULL,
		handedness TEXT NOT NULL DEFAULT 'none',
		rarity TEXT NOT NULL,
		class_restriction TEXT,
		level_requirement INTEGER NOT NULL DEFAULT 1,
		image_url TEXT NOT NULL,
		attack_bonus INTEGER NOT NULL DEFAULT 0,
		defense_bonus INTEGER NOT NULL DEFAULT 0,
		mobility_bonus INTEGER NOT NULL DEFAULT 0,
		agility_bonus INTEGER NOT NULL DEFAULT 0,
		max_health_bonus INTEGER NOT NULL DEFAULT 0,
		max_energy_bonus INTEGER NOT NULL DEFAULT 0,
		sight_range_bonus INTEGER NOT NULL DEFAULT 0,
		attack_range_bonus INTEGER NOT NULL DEFAULT 0,
		npc_price INTEGER NOT NULL DEFAULT 0,
		sell_price INTEGER NOT NULL DEFAULT 0,
		tradable BOOLEAN NOT NULL DEFAULT true,
		stackable BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		CONSTRAINT item_templates_slot_check CHECK (` + equipmentSlotsCheck + `),
		CONSTRAINT item_templates_item_type_check CHECK (item_type IN ('sword', 'shield', 'staff', 'bow', 'axe', 'helmet', 'chest', 'pants', 'boots', 'gloves', 'ring', 'amulet')),
		CONSTRAINT item_templates_handedness_check CHECK (handedness IN ('none', 'one_hand', 'two_hand')),
		CONSTRAINT item_templates_rarity_check CHECK (rarity IN ('green', 'blue', 'purple', 'orange')),
		CONSTRAINT item_templates_class_restriction_check CHECK (class_restriction IS NULL OR class_restriction IN ('guardian', 'berserker', 'ranger', 'mystic')),
		CONSTRAINT item_templates_level_requirement_check CHECK (level_requirement > 0),
		CONSTRAINT item_templates_prices_check CHECK (npc_price >= 0 AND sell_price >= 0),
		CONSTRAINT item_templates_code_not_blank_check CHECK (btrim(code) <> ''),
		CONSTRAINT item_templates_name_not_blank_check CHECK (btrim(name) <> ''),
		CONSTRAINT item_templates_image_url_not_blank_check CHECK (btrim(image_url) <> ''),
		CONSTRAINT item_templates_slot_type_handedness_check CHECK (
			(item_type IN ('staff', 'bow', 'axe') AND slot = 'main_hand' AND handedness = 'two_hand')
			OR (item_type = 'sword' AND slot = 'main_hand' AND handedness = 'one_hand')
			OR (item_type = 'shield' AND slot = 'off_hand' AND handedness = 'one_hand')
			OR (item_type IN ('helmet', 'chest', 'pants', 'boots', 'gloves', 'ring', 'amulet') AND slot = item_type AND handedness = 'none')
		)
	);

	CREATE TABLE IF NOT EXISTS item_set_bonuses (
		id BIGSERIAL PRIMARY KEY,
		set_id BIGINT NOT NULL REFERENCES item_sets(id) ON DELETE CASCADE,
		pieces_required INTEGER NOT NULL,
		description TEXT NOT NULL,
		attack_bonus INTEGER NOT NULL DEFAULT 0,
		defense_bonus INTEGER NOT NULL DEFAULT 0,
		mobility_bonus INTEGER NOT NULL DEFAULT 0,
		agility_bonus INTEGER NOT NULL DEFAULT 0,
		max_health_bonus INTEGER NOT NULL DEFAULT 0,
		max_energy_bonus INTEGER NOT NULL DEFAULT 0,
		sight_range_bonus INTEGER NOT NULL DEFAULT 0,
		attack_range_bonus INTEGER NOT NULL DEFAULT 0,
		effect_code TEXT,
		effect_value INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE (set_id, pieces_required),
		CONSTRAINT item_set_bonuses_pieces_required_check CHECK (pieces_required > 0),
		CONSTRAINT item_set_bonuses_description_not_blank_check CHECK (btrim(description) <> '')
	);

	CREATE TABLE IF NOT EXISTS item_instances (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		template_id BIGINT NOT NULL REFERENCES item_templates(id),
		owner_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id) ON DELETE CASCADE,
		current_character_id INTEGER REFERENCES player_characters(id) ON DELETE SET NULL,
		status TEXT NOT NULL DEFAULT 'inventory',
		durability INTEGER,
		max_durability INTEGER,
		bound_to_user_id INTEGER REFERENCES player_profiles(user_id) ON DELETE SET NULL,
		bound_to_character_id INTEGER REFERENCES player_characters(id) ON DELETE SET NULL,
		is_soulbound BOOLEAN NOT NULL DEFAULT false,
		is_locked BOOLEAN NOT NULL DEFAULT false,
		source TEXT NOT NULL DEFAULT 'unknown',
		match_instance_id TEXT,
		acquired_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		version INTEGER NOT NULL DEFAULT 1,
		CONSTRAINT item_instances_status_check CHECK (status IN ('inventory', 'equipped', 'listed', 'trade_locked', 'deleted')),
		CONSTRAINT item_instances_source_check CHECK (source IN ('drop', 'shop', 'quest', 'craft', 'admin', 'trade', 'unknown')),
		CONSTRAINT item_instances_durability_check CHECK (durability IS NULL OR durability >= 0),
		CONSTRAINT item_instances_max_durability_check CHECK (max_durability IS NULL OR max_durability >= 0),
		CONSTRAINT item_instances_durability_bounds_check CHECK (durability IS NULL OR max_durability IS NULL OR durability <= max_durability),
		CONSTRAINT item_instances_version_check CHECK (version > 0)
	);

	ALTER TABLE item_instances ADD COLUMN IF NOT EXISTS match_instance_id TEXT;

	CREATE TABLE IF NOT EXISTS character_equipment (
		id BIGSERIAL PRIMARY KEY,
		character_id INTEGER NOT NULL REFERENCES player_characters(id) ON DELETE CASCADE,
		item_instance_id UUID NOT NULL REFERENCES item_instances(id) ON DELETE CASCADE,
		slot TEXT NOT NULL,
		equipped_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE (character_id, slot),
		UNIQUE (item_instance_id),
		CONSTRAINT character_equipment_slot_check CHECK (` + equipmentSlotsCheck + `)
	);

	CREATE TABLE IF NOT EXISTS item_instance_events (
		id BIGSERIAL PRIMARY KEY,
		item_instance_id UUID NOT NULL REFERENCES item_instances(id) ON DELETE CASCADE,
		event_type TEXT NOT NULL,
		from_user_id INTEGER REFERENCES player_profiles(user_id) ON DELETE SET NULL,
		to_user_id INTEGER REFERENCES player_profiles(user_id) ON DELETE SET NULL,
		from_character_id INTEGER REFERENCES player_characters(id) ON DELETE SET NULL,
		to_character_id INTEGER REFERENCES player_characters(id) ON DELETE SET NULL,
		metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		CONSTRAINT item_instance_events_event_type_check CHECK (event_type IN ('created', 'equipped', 'unequipped', 'traded', 'listed', 'unlisted', 'sold', 'deleted', 'locked', 'unlocked'))
	);

	CREATE INDEX IF NOT EXISTS idx_item_templates_set_id ON item_templates(set_id);
	CREATE INDEX IF NOT EXISTS idx_item_templates_code ON item_templates(code);
	CREATE INDEX IF NOT EXISTS idx_item_templates_class_restriction ON item_templates(class_restriction);
	CREATE INDEX IF NOT EXISTS idx_item_instances_owner_user_id ON item_instances(owner_user_id);
	CREATE INDEX IF NOT EXISTS idx_item_instances_template_id ON item_instances(template_id);
	CREATE INDEX IF NOT EXISTS idx_item_instances_current_character_id ON item_instances(current_character_id);
	CREATE INDEX IF NOT EXISTS idx_item_instances_status ON item_instances(status);
	CREATE INDEX IF NOT EXISTS idx_item_instances_owner_match ON item_instances(owner_user_id, match_instance_id);
	CREATE INDEX IF NOT EXISTS idx_character_equipment_character_id ON character_equipment(character_id);
	CREATE INDEX IF NOT EXISTS idx_character_equipment_item_instance_id ON character_equipment(item_instance_id);
	CREATE INDEX IF NOT EXISTS idx_item_instance_events_item_instance_id ON item_instance_events(item_instance_id);
	CREATE INDEX IF NOT EXISTS idx_item_instance_events_created_at ON item_instance_events(created_at);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания equipment schema: %v", err)
	}
}

func EnsureEquipmentSeedData() {
	if err := seedEquipmentCatalog(); err != nil {
		log.Fatalf("Ошибка сидирования equipment data: %v", err)
	}
}
