// ==============================
// /gameservice/repository/db.go
// ==============================

package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"gameservice/game"

	_ "github.com/lib/pq"
)

var connStr = os.Getenv("GAME_DB_DSN")

var DB *sql.DB

func InitDB() {
	var err error

	if connStr == "" {
		log.Fatal("GAME_DB_DSN environment variable is not set")
	}
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	if err = DB.Ping(); err != nil {
		log.Fatalf("Невозможно подключиться к БД: %v", err)
	}
	log.Println("Подключение к БД установлено")

	CreateMonstersTable()
	CreateResourcesTable()
	CreateArtifactsTable()

	// Существующие таблицы
	CreatePlayersTable()
	EnsurePlayersCharacterTypeColumn()
	EnsurePlayersStatColumns()
	CreateMatchesTable()
	CreateMatchPlayersTable()
	EnsureMatchPlayersStatColumns()
	CreateInventoryTable()
	// Добавляем нашу новую
	CreateMatchMonstersTable()
	CreatePersistedArtifactsTable()
	EnsureStaticGameData()
	CreateMatchStatsTable()
	CreateMatchPlayerStatsTable()
	EnsurePlayerBaseBuildingsTable()
	EnsureMatchStatsRetentionSchema()
	EnsureQuestArtifactColumn()

	// Восстанавливаем state матчей
	if err := RestoreMatchStates(); err != nil {
		log.Fatalf("Ошибка восстановления матчей: %v", err)
	}
}

func CreateMonstersTable() {
	query := `
	CREATE TABLE IF NOT EXISTS monsters (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		health INTEGER NOT NULL,
		max_health INTEGER NOT NULL,
		attack INTEGER NOT NULL,
		defense INTEGER NOT NULL,
		speed INTEGER NOT NULL,
		maneuverability INTEGER NOT NULL,
		vision INTEGER NOT NULL,
		image TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_monsters_name_unique ON monsters(name);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы monsters: %v", err)
	}
}

func CreateResourcesTable() {
	query := `
	CREATE TABLE IF NOT EXISTS resources (
		id SERIAL PRIMARY KEY,
		type TEXT NOT NULL,
		description TEXT,
		effect TEXT,
		image TEXT
	);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы resources: %v", err)
	}
}

func CreateArtifactsTable() {
	query := `
	CREATE TABLE IF NOT EXISTS artifacts (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		bonus JSONB NOT NULL DEFAULT '{}'::jsonb,
		image TEXT
	);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы artifacts: %v", err)
	}
}

func CreatePlayersTable() {
	query := `
	CREATE TABLE IF NOT EXISTS players (
		user_id SERIAL PRIMARY KEY,
		name TEXT DEFAULT 'Unnamed Player',
		image TEXT DEFAULT '/player-1.webp',
		character_type TEXT DEFAULT 'adventurer',
		energy INTEGER DEFAULT 100,
		max_energy INTEGER DEFAULT 100,
		health INTEGER DEFAULT 100,
		max_health INTEGER DEFAULT 100,
		level INTEGER DEFAULT 1,
		experience INTEGER DEFAULT 0,
		max_experience INTEGER DEFAULT 500,
		attack INTEGER DEFAULT 10,
		defense INTEGER DEFAULT 5,
		mobility INTEGER DEFAULT 3,
		agility INTEGER DEFAULT 2,
		sight_range INTEGER DEFAULT 2,
		is_ranged BOOLEAN DEFAULT FALSE,
		attack_range INTEGER DEFAULT 1,
		balance INTEGER DEFAULT 0,
		inventory JSONB DEFAULT '{}'
	);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы players: %v", err)
	}
}

func EnsurePlayersCharacterTypeColumn() {
	_, err := DB.Exec(`ALTER TABLE players ADD COLUMN IF NOT EXISTS character_type TEXT DEFAULT 'adventurer'`)
	if err != nil {
		log.Printf("EnsurePlayersCharacterTypeColumn: %v", err)
	}
}

func EnsurePlayersStatColumns() {
	_, err := DB.Exec(`ALTER TABLE players RENAME COLUMN speed TO mobility`)
	if err != nil {
		log.Printf("EnsurePlayersStatColumns rename speed->mobility: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE players RENAME COLUMN maneuverability TO agility`)
	if err != nil {
		log.Printf("EnsurePlayersStatColumns rename maneuverability->agility: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE players RENAME COLUMN vision_range TO sight_range`)
	if err != nil {
		log.Printf("EnsurePlayersStatColumns rename vision_range->sight_range: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE players RENAME COLUMN range_attack TO is_ranged`)
	if err != nil {
		log.Printf("EnsurePlayersStatColumns rename range_attack->is_ranged: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE players RENAME COLUMN range_distance TO attack_range`)
	if err != nil {
		log.Printf("EnsurePlayersStatColumns rename range_distance->attack_range: %v", err)
	}

	_, err = DB.Exec(`ALTER TABLE players ADD COLUMN IF NOT EXISTS mobility INTEGER DEFAULT 3`)
	if err != nil {
		log.Printf("EnsurePlayersStatColumns mobility: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE players ADD COLUMN IF NOT EXISTS agility INTEGER DEFAULT 2`)
	if err != nil {
		log.Printf("EnsurePlayersStatColumns agility: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE players ADD COLUMN IF NOT EXISTS sight_range INTEGER DEFAULT 2`)
	if err != nil {
		log.Printf("EnsurePlayersStatColumns sight_range: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE players ADD COLUMN IF NOT EXISTS is_ranged BOOLEAN DEFAULT FALSE`)
	if err != nil {
		log.Printf("EnsurePlayersStatColumns is_ranged: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE players ADD COLUMN IF NOT EXISTS attack_range INTEGER DEFAULT 1`)
	if err != nil {
		log.Printf("EnsurePlayersStatColumns attack_range: %v", err)
	}
}

func CreateMatchesTable() {
	query := `
	CREATE TABLE IF NOT EXISTS matches (
		instance_id TEXT PRIMARY KEY,
		mode TEXT NOT NULL,
		teams_count INTEGER NOT NULL,
		total_players INTEGER NOT NULL,
		active_user_id INTEGER,
		winner_id INTEGER,
		winner_group_id INTEGER,
		turn_order JSONB,
		turn_number INTEGER DEFAULT 1,
		start_positions JSONB,
		portal_position JSONB,
		map_height INTEGER NOT NULL,
		map_width INTEGER NOT NULL,
		map JSONB NOT NULL,
		quest_artifact_id INTEGER DEFAULT 0
	);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы matches: %v", err)
	}
}

func EnsureQuestArtifactColumn() {
	_, err := DB.Exec(`ALTER TABLE matches ADD COLUMN IF NOT EXISTS quest_artifact_id INTEGER DEFAULT 0`)
	if err != nil {
		log.Printf("EnsureQuestArtifactColumn: %v", err)
	}
}

func CreateMatchPlayersTable() {
	query := `
	CREATE TABLE IF NOT EXISTS match_players (
		instance_id TEXT NOT NULL REFERENCES matches(instance_id) ON DELETE CASCADE,
		user_id INTEGER NOT NULL,
		name TEXT,
		position JSONB,
		inventory JSONB,
		level INTEGER,
		energy INTEGER,
		max_energy INTEGER,
		health INTEGER,
		max_health INTEGER,
		experience INTEGER,
		max_experience INTEGER,
		attack INTEGER,
		defense INTEGER,
		mobility INTEGER,
		agility INTEGER,
		sight_range INTEGER,
		is_ranged BOOLEAN,
		attack_range INTEGER,
		balance INTEGER,
        image TEXT,
		group_id INTEGER,
		PRIMARY KEY (instance_id, user_id)
	);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы match_players: %v", err)
	}
}

func EnsureMatchPlayersStatColumns() {
	_, err := DB.Exec(`ALTER TABLE match_players RENAME COLUMN speed TO mobility`)
	if err != nil {
		log.Printf("EnsureMatchPlayersStatColumns rename speed->mobility: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE match_players RENAME COLUMN maneuverability TO agility`)
	if err != nil {
		log.Printf("EnsureMatchPlayersStatColumns rename maneuverability->agility: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE match_players RENAME COLUMN vision_range TO sight_range`)
	if err != nil {
		log.Printf("EnsureMatchPlayersStatColumns rename vision_range->sight_range: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE match_players RENAME COLUMN range_attack TO is_ranged`)
	if err != nil {
		log.Printf("EnsureMatchPlayersStatColumns rename range_attack->is_ranged: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE match_players RENAME COLUMN range_distance TO attack_range`)
	if err != nil {
		log.Printf("EnsureMatchPlayersStatColumns rename range_distance->attack_range: %v", err)
	}

	_, err = DB.Exec(`ALTER TABLE match_players ADD COLUMN IF NOT EXISTS mobility INTEGER DEFAULT 3`)
	if err != nil {
		log.Printf("EnsureMatchPlayersStatColumns mobility: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE match_players ADD COLUMN IF NOT EXISTS agility INTEGER DEFAULT 2`)
	if err != nil {
		log.Printf("EnsureMatchPlayersStatColumns agility: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE match_players ADD COLUMN IF NOT EXISTS sight_range INTEGER DEFAULT 2`)
	if err != nil {
		log.Printf("EnsureMatchPlayersStatColumns sight_range: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE match_players ADD COLUMN IF NOT EXISTS is_ranged BOOLEAN DEFAULT FALSE`)
	if err != nil {
		log.Printf("EnsureMatchPlayersStatColumns is_ranged: %v", err)
	}
	_, err = DB.Exec(`ALTER TABLE match_players ADD COLUMN IF NOT EXISTS attack_range INTEGER DEFAULT 1`)
	if err != nil {
		log.Printf("EnsureMatchPlayersStatColumns attack_range: %v", err)
	}
}

func CreateInventoryTable() {
	query := `
	CREATE TABLE IF NOT EXISTS inventory_items (
	  id            SERIAL PRIMARY KEY,
	  instance_id   TEXT NOT NULL
	                    REFERENCES matches(instance_id)
	                    ON DELETE CASCADE,
	  user_id       INTEGER NOT NULL
	                    REFERENCES players(user_id)
	                    ON DELETE CASCADE,
	  item_type     TEXT    NOT NULL,    -- 'resource' или 'artifact'
	  item_id       INTEGER NOT NULL,    -- ID ресурса или артефакта
	  item_name     TEXT    NOT NULL,    -- человекочитаемое название
	  image_url         TEXT,
	  item_description TEXT NOT NULL,
	  item_count    INTEGER NOT NULL DEFAULT 1,
	  base_value    NUMERIC(12,2) NOT NULL DEFAULT 0,
	  npc_price     NUMERIC(12,2) NOT NULL DEFAULT 0,
	  durability    INTEGER NOT NULL DEFAULT 100,
	  acquired_at   TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
	  expires_at    TIMESTAMP WITH TIME ZONE,
	  UNIQUE (instance_id, user_id, item_type, item_id)
	);

	CREATE INDEX IF NOT EXISTS idx_inventory_user
	  ON inventory_items(user_id);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы inventory_items: %v", err)
	}
}

func CreateMatchMonstersTable() {
	query := `
  CREATE TABLE IF NOT EXISTS match_monsters (
    instance_id TEXT NOT NULL REFERENCES matches(instance_id) ON DELETE CASCADE,
    monster_instance_id SERIAL NOT NULL,
    monster_ref_id      INT  NOT NULL,
    pos_x               INT  NOT NULL,
    pos_y               INT  NOT NULL,
    health              INT  NOT NULL,
    max_health          INT  NOT NULL,
    attack              INT  NOT NULL,
    defense             INT  NOT NULL,
    speed               INT  NOT NULL,
    maneuverability     INT  NOT NULL,
    vision              INT  NOT NULL,
    image               TEXT NOT NULL,
    PRIMARY KEY (instance_id, monster_instance_id)
  );
  `
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы match_monsters: %v", err)
	}
}

// CreatePersistedArtifactsTable создаёт таблицу для «вечных» артефактов, выходящих за рамки инстанса.
func CreatePersistedArtifactsTable() {
	query := `
  CREATE TABLE IF NOT EXISTS persisted_artifacts (
    id              SERIAL PRIMARY KEY,
    user_id         INTEGER NOT NULL
                       REFERENCES players(user_id)
                       ON DELETE CASCADE,
    artifact_type   TEXT    NOT NULL,
    artifact_id     INTEGER NOT NULL,
    description     TEXT,
    image           TEXT,
    base_value      NUMERIC(12,2) NOT NULL DEFAULT 0,
    npc_price       NUMERIC(12,2) NOT NULL DEFAULT 0,
    rarity          TEXT    NOT NULL DEFAULT 'common',
    durability      INTEGER NOT NULL DEFAULT 100,
    acquired_at     TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
    expires_at      TIMESTAMP WITH TIME ZONE
  );
  CREATE INDEX IF NOT EXISTS idx_persisted_artifacts_user
    ON persisted_artifacts(user_id);
  `
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания persisted_artifacts: %v", err)
	}

	if _, err := DB.Exec(`ALTER TABLE persisted_artifacts ADD COLUMN IF NOT EXISTS description TEXT`); err != nil {
		log.Fatalf("Ошибка миграции persisted_artifacts.description: %v", err)
	}
	if _, err := DB.Exec(`ALTER TABLE persisted_artifacts ADD COLUMN IF NOT EXISTS image TEXT`); err != nil {
		log.Fatalf("Ошибка миграции persisted_artifacts.image: %v", err)
	}
}

func EnsureStaticGameData() {
	if _, err := DB.Exec(`
		INSERT INTO monsters (name, type, health, max_health, attack, defense, speed, maneuverability, vision, image)
		SELECT 'Goblin', 'monster', 35, 35, 7, 3, 6, 6, 5, '/monsters/goblin.webp'
		WHERE NOT EXISTS (SELECT 1 FROM monsters WHERE name = 'Goblin');

		INSERT INTO monsters (name, type, health, max_health, attack, defense, speed, maneuverability, vision, image)
		SELECT 'Goblins', 'monster', 50, 50, 10, 4, 5, 5, 5, '/monsters/goblins.webp'
		WHERE NOT EXISTS (SELECT 1 FROM monsters WHERE name = 'Goblins');

		INSERT INTO monsters (name, type, health, max_health, attack, defense, speed, maneuverability, vision, image)
		SELECT 'Orc', 'monster', 75, 75, 14, 8, 3, 2, 4, '/monsters/orc.webp'
		WHERE NOT EXISTS (SELECT 1 FROM monsters WHERE name = 'Orc');

		INSERT INTO monsters (name, type, health, max_health, attack, defense, speed, maneuverability, vision, image)
		SELECT 'Troll', 'monster', 120, 120, 18, 12, 2, 1, 3, '/monsters/troll.webp'
		WHERE NOT EXISTS (SELECT 1 FROM monsters WHERE name = 'Troll');

		-- Обновляем старые записи по имени, если они уже существовали с устаревшими статами/картинкой.
		UPDATE monsters SET
			type = 'monster', health = 35, max_health = 35, attack = 7, defense = 3,
			speed = 6, maneuverability = 6, vision = 5, image = '/monsters/goblin.webp'
		WHERE name = 'Goblin';

		UPDATE monsters SET
			type = 'monster', health = 50, max_health = 50, attack = 10, defense = 4,
			speed = 5, maneuverability = 5, vision = 5, image = '/monsters/goblins.webp'
		WHERE name = 'Goblins';

		UPDATE monsters SET
			type = 'monster', health = 75, max_health = 75, attack = 14, defense = 8,
			speed = 3, maneuverability = 2, vision = 4, image = '/monsters/orc.webp'
		WHERE name = 'Orc';

		UPDATE monsters SET
			type = 'monster', health = 120, max_health = 120, attack = 18, defense = 12,
			speed = 2, maneuverability = 1, vision = 3, image = '/monsters/troll.webp'
		WHERE name = 'Troll';
	`); err != nil {
		log.Fatalf("Ошибка сидирования monsters: %v", err)
	}

	if _, err := DB.Exec(`
		INSERT INTO resources (id, type, description, effect, image)
		SELECT 6, 'barrel', 'Barrel loot container', '{}', '/main_resources/barrel.webp'
		WHERE NOT EXISTS (SELECT 1 FROM resources WHERE id = 6)
	`); err != nil {
		log.Fatalf("Ошибка сидирования barrel resource (id=6): %v", err)
	}

	if _, err := DB.Exec(`
		UPDATE resources
		SET type = 'food',
			description = 'Food restores 20 HP (not above max)',
			effect = '{"health":20}',
			image = '/main_resources/food.webp'
		WHERE type = 'potion'
		  AND NOT EXISTS (SELECT 1 FROM resources r2 WHERE r2.type = 'food' AND r2.id <> resources.id)
	`); err != nil {
		log.Fatalf("Ошибка миграции potion -> food: %v", err)
	}

	if _, err := DB.Exec(`
		INSERT INTO resources (type, description, effect, image)
		SELECT 'food', 'Food restores 20 HP (not above max)', '{"health":20}', '/main_resources/food.webp'
		WHERE NOT EXISTS (SELECT 1 FROM resources WHERE type = 'food');

		INSERT INTO resources (type, description, effect, image)
		SELECT 'water', 'Water restores 5 energy (not above max)', '{"energy":5}', '/main_resources/water.webp'
		WHERE NOT EXISTS (SELECT 1 FROM resources WHERE type = 'water');

		INSERT INTO resources (type, description, effect, image)
		SELECT 'wood', 'Wood for early base construction', '{"material_wood":1}', '/main_resources/wood.webp'
		WHERE NOT EXISTS (SELECT 1 FROM resources WHERE type = 'wood');

		INSERT INTO resources (type, description, effect, image)
		SELECT 'stone', 'Stone for early base construction', '{"material_stone":1}', '/main_resources/stone.webp'
		WHERE NOT EXISTS (SELECT 1 FROM resources WHERE type = 'stone');

		INSERT INTO resources (type, description, effect, image)
		SELECT 'iron', 'Iron for early base construction', '{"material_iron":1}', '/main_resources/iron.webp'
		WHERE NOT EXISTS (SELECT 1 FROM resources WHERE type = 'iron');

		UPDATE resources SET description = 'Barrel loot container', effect = '{}', image = '/main_resources/barrel.webp' WHERE type = 'barrel';
		UPDATE resources SET description = 'Food restores 20 HP (not above max)', effect = '{"health":20}', image = '/main_resources/food.webp' WHERE type = 'food';
		UPDATE resources SET description = 'Water restores 5 energy (not above max)', effect = '{"energy":5}', image = '/main_resources/water.webp' WHERE type = 'water';
		UPDATE resources SET description = 'Wood for early base construction', effect = '{"material_wood":1}', image = '/main_resources/wood.webp' WHERE type = 'wood';
		UPDATE resources SET description = 'Stone for early base construction', effect = '{"material_stone":1}', image = '/main_resources/stone.webp' WHERE type = 'stone';
		UPDATE resources SET description = 'Iron for early base construction', effect = '{"material_iron":1}', image = '/main_resources/iron.webp' WHERE type = 'iron';
	`); err != nil {
		log.Fatalf("Ошибка сидирования/нормализации resources: %v", err)
	}

	if _, err := DB.Exec(`
		SELECT setval(
			'resources_id_seq',
			GREATEST((SELECT COALESCE(MAX(id), 1) FROM resources), 1),
			true
		)
	`); err != nil {
		log.Fatalf("Ошибка корректировки resources_id_seq: %v", err)
	}

	if _, err := DB.Exec(`
		DELETE FROM artifacts WHERE name = 'ancient_amulet';

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'berserker_axe', 'Heavy axe that increases raw damage', '{"attack":8,"defense":-1}'::jsonb, '/artifacts/berserker-axe.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'berserker_axe');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'boots_of_stealth', 'Silent boots improving mobility and scouting', '{"speed":2,"maneuverability":3}'::jsonb, '/artifacts/boots_of_stealth.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'boots_of_stealth');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'crown_of_enlightenment', 'Crown that sharpens battlefield awareness', '{"vision":3}'::jsonb, '/artifacts/crown-of-enlightenment.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'crown_of_enlightenment');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'dragon_eye', 'Rare relic balancing offense and awareness', '{"attack":3,"vision":2}'::jsonb, '/artifacts/dragon-eye.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'dragon_eye');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'fire_amulet', 'Amulet that empowers aggressive style', '{"attack":5}'::jsonb, '/artifacts/fire-amulet.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'fire_amulet');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'gloves_of_precision', 'Precision gloves for cleaner strikes', '{"attack":2,"maneuverability":2}'::jsonb, '/artifacts/gloves-of-precision.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'gloves_of_precision');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'guardian_shield', 'Sturdy shield focused on survival', '{"defense":6,"speed":-1}'::jsonb, '/artifacts/guardian-shield.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'guardian_shield');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'knight_sword', 'Reliable sword for balanced combat', '{"attack":4,"defense":1}'::jsonb, '/artifacts/knight-sword.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'knight_sword');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'ring_of_wisdom', 'Ring improving tactical vision and control', '{"vision":2,"maneuverability":1}'::jsonb, '/artifacts/ring-of-wisdom.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'ring_of_wisdom');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'titan_breastplate', 'Massive armor with top-tier protection', '{"defense":9,"speed":-2}'::jsonb, '/artifacts/titan-breastplate.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'titan_breastplate');

		UPDATE artifacts SET description = 'Heavy axe that increases raw damage', bonus = '{"attack":8,"defense":-1}'::jsonb, image = '/artifacts/berserker-axe.webp' WHERE name = 'berserker_axe';
		UPDATE artifacts SET description = 'Silent boots improving mobility and scouting', bonus = '{"speed":2,"maneuverability":3}'::jsonb, image = '/artifacts/boots_of_stealth.webp' WHERE name = 'boots_of_stealth';
		UPDATE artifacts SET description = 'Crown that sharpens battlefield awareness', bonus = '{"vision":3}'::jsonb, image = '/artifacts/crown-of-enlightenment.webp' WHERE name = 'crown_of_enlightenment';
		UPDATE artifacts SET description = 'Rare relic balancing offense and awareness', bonus = '{"attack":3,"vision":2}'::jsonb, image = '/artifacts/dragon-eye.webp' WHERE name = 'dragon_eye';
		UPDATE artifacts SET description = 'Amulet that empowers aggressive style', bonus = '{"attack":5}'::jsonb, image = '/artifacts/fire-amulet.webp' WHERE name = 'fire_amulet';
		UPDATE artifacts SET description = 'Precision gloves for cleaner strikes', bonus = '{"attack":2,"maneuverability":2}'::jsonb, image = '/artifacts/gloves-of-precision.webp' WHERE name = 'gloves_of_precision';
		UPDATE artifacts SET description = 'Sturdy shield focused on survival', bonus = '{"defense":6,"speed":-1}'::jsonb, image = '/artifacts/guardian-shield.webp' WHERE name = 'guardian_shield';
		UPDATE artifacts SET description = 'Reliable sword for balanced combat', bonus = '{"attack":4,"defense":1}'::jsonb, image = '/artifacts/knight-sword.webp' WHERE name = 'knight_sword';
		UPDATE artifacts SET description = 'Ring improving tactical vision and control', bonus = '{"vision":2,"maneuverability":1}'::jsonb, image = '/artifacts/ring-of-wisdom.webp' WHERE name = 'ring_of_wisdom';
		UPDATE artifacts SET description = 'Massive armor with top-tier protection', bonus = '{"defense":9,"speed":-2}'::jsonb, image = '/artifacts/titan-breastplate.webp' WHERE name = 'titan_breastplate';
	`); err != nil {
		log.Fatalf("Ошибка сидирования artifacts: %v", err)
	}
}

// CreateMatchStatsTable создаёт таблицу для хранения итогов матчей.
func CreateMatchStatsTable() {
	query := `
    CREATE TABLE IF NOT EXISTS match_stats (
        instance_id      TEXT PRIMARY KEY
                           REFERENCES matches(instance_id)
                           ON DELETE CASCADE,
        winner_id        INTEGER     NOT NULL,
        winner_group_id  INTEGER     NOT NULL,
        created_at       TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
    );
    `
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы match_stats: %v", err)
	}
}

// CreateMatchPlayerStatsTable создаёт таблицу для хранения детальных результатов каждого игрока в матче.
func CreateMatchPlayerStatsTable() {
	query := `
    CREATE TABLE IF NOT EXISTS match_player_stats (
        instance_id        TEXT     NOT NULL
                                 REFERENCES match_stats(instance_id)
                                 ON DELETE CASCADE,
        user_id            INTEGER  NOT NULL,
        exp_gained         INTEGER  NOT NULL,
        rewards            JSONB    NOT NULL,
        player_kills       INTEGER  NOT NULL,
        monster_kills      INTEGER  NOT NULL,
        damage_total       INTEGER  NOT NULL,
        damage_to_players  INTEGER  NOT NULL,
        damage_to_monsters INTEGER  NOT NULL,
        PRIMARY KEY (instance_id, user_id)
    );
    `
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы match_player_stats: %v", err)
	}
}

// EnsureMatchStatsRetentionSchema убирает каскадное удаление истории матча
// из-за FK match_stats -> matches, чтобы match_stats/match_player_stats
// сохранялись после DeleteMatch.
func EnsureMatchStatsRetentionSchema() {
	query := `
	ALTER TABLE IF EXISTS match_stats
	DROP CONSTRAINT IF EXISTS match_stats_instance_id_fkey;
	`
	if _, err := DB.Exec(query); err != nil {
		log.Printf("EnsureMatchStatsRetentionSchema: %v", err)
	}
}

func RestoreMatchStates() error {
	rows, err := DB.Query("SELECT instance_id, active_user_id, turn_order FROM matches")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var instanceID string
		var activeUserID int
		var turnOrderJSON []byte
		if err := rows.Scan(&instanceID, &activeUserID, &turnOrderJSON); err != nil {
			return err
		}

		var turnOrder []int
		if err := json.Unmarshal(turnOrderJSON, &turnOrder); err != nil {
			log.Printf("Ошибка восстановления состояния матча %s: %v", instanceID, err)
			continue
		}

		matchState := &game.MatchState{
			InstanceID:   instanceID,
			ActiveUserID: activeUserID,
			TurnOrder:    turnOrder,
		}
		game.MatchStatesMu.Lock()
		game.MatchStates[instanceID] = matchState
		game.MatchStatesMu.Unlock()

		log.Printf("Состояние матча %s восстановлено", instanceID)
	}
	return nil
}

// GetMatchResults читает из БД и игровой логики всё, что нужно для финализации матча.
func GetMatchResults(instanceID string) (*game.MatchResults, error) {
	log.Printf("[GetMatchResults] fetching for match %s", instanceID)

	var mr game.MatchResults

	// 1) Победители из таблицы matches, NULL → 0 (не определён)
	if err := DB.QueryRow(
		`SELECT
            COALESCE(winner_id, 0),
            COALESCE(winner_group_id, 0)
         FROM matches
        WHERE instance_id = $1`,
		instanceID,
	).Scan(&mr.WinnerID, &mr.WinnerGroupID); err != nil {
		return nil, fmt.Errorf("GetMatchResults: не удалось прочитать победителей: %w", err)
	}

	// // 1) Победители из таблицы matches
	// if err := DB.QueryRow(
	//     `SELECT winner_id, winner_group_id
	//        FROM matches
	//       WHERE instance_id = $1`,
	//     instanceID,
	// ).Scan(&mr.WinnerID, &mr.WinnerGroupID); err != nil {
	//     return nil, fmt.Errorf("GetMatchResults: не удалось прочитать победителей: %w", err)
	// }

	// 2) Список участников
	rows, err := DB.Query(
		`SELECT user_id, COALESCE(group_id, 0), COALESCE(health, 0)
           FROM match_players
          WHERE instance_id = $1`,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetMatchResults: не удалось получить игроков: %w", err)
	}
	defer rows.Close()

	groupByUser := make(map[int]int)

	// 3) Для каждого игрока — игровая логика CalculateResults
	for rows.Next() {
		var userID, groupID, health int
		if err := rows.Scan(&userID, &groupID, &health); err != nil {
			log.Printf("GetMatchResults: пропускаем игрока из-за Scan: %v", err)
			continue
		}
		groupByUser[userID] = groupID
		survived := health > 0

		exp, rewards, killsP, killsM, dmgTotal, dmgPlayers, dmgMonsters, err :=
			game.CalculateResults(instanceID, userID, survived)
		if err != nil {
			log.Printf("GetMatchResults: CalculateResults для user %d вернул ошибку: %v", userID, err)
			continue
		}

		// Сериализуем награды
		rewardsJSON, err := json.Marshal(rewards)
		if err != nil {
			log.Printf("GetMatchResults: ошибка JSON.Marshal для user %d: %v", userID, err)
			continue
		}

		mr.PlayerResults = append(mr.PlayerResults, game.PlayerResult{
			UserID:           userID,
			ExpGained:        exp,
			RewardsData:      rewardsJSON,
			PlayerKills:      killsP,
			MonsterKills:     killsM,
			DamageTotal:      dmgTotal,
			DamageToPlayers:  dmgPlayers,
			DamageToMonsters: dmgMonsters,
		})
	}

	// 3.1) Победный бонус: +XP и +деньги победителю (или его команде).
	const winnerExpBonus = 120
	const winnerMoneyBonus = 100
	for i := range mr.PlayerResults {
		isWinner := false
		if mr.WinnerGroupID > 0 {
			isWinner = groupByUser[mr.PlayerResults[i].UserID] == mr.WinnerGroupID
		} else if mr.WinnerID > 0 {
			isWinner = mr.PlayerResults[i].UserID == mr.WinnerID
		}
		if !isWinner {
			continue
		}

		mr.PlayerResults[i].ExpGained += winnerExpBonus

		var existing []game.Reward
		if err := json.Unmarshal(mr.PlayerResults[i].RewardsData, &existing); err != nil {
			existing = []game.Reward{}
		}

		merged := false
		for j := range existing {
			if existing[j].Type == "balance" || existing[j].Type == "coin" {
				existing[j].Type = "balance"
				existing[j].Amount += winnerMoneyBonus
				merged = true
				break
			}
		}
		if !merged {
			existing = append(existing, game.Reward{Type: "balance", Amount: winnerMoneyBonus})
		}

		if b, err := json.Marshal(existing); err == nil {
			mr.PlayerResults[i].RewardsData = b
		}
	}

	log.Printf("[GetMatchResults] found %d players for match %s", len(mr.PlayerResults), instanceID)
	if len(mr.PlayerResults) == 0 {
		return nil, fmt.Errorf("GetMatchResults: не найдено ни одного игрока для матча %s", instanceID)
	}

	return &mr, nil
}
