// ==============================
// /gameservice/repository/db.go
// ==============================

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"gameservice/game"
	"log"
	"os"
	"sort"
	"time"

	_ "github.com/lib/pq"
)

var connStr = os.Getenv("GAME_DB_DSN")

var DB *sql.DB

const dbConnectTimeout = 90 * time.Second
const dbConnectRetryInterval = 2 * time.Second
const schemaReadyTimeout = 90 * time.Second

func InitDB() {
	ConnectDB()
	RunMigrations()
	ready, err := SchemaReady()
	if err != nil {
		log.Fatalf("PostgreSQL schema is not ready for match commands: %v", err)
	}
	if !ready {
		log.Fatal("PostgreSQL base schema is not ready for match commands")
	}
}

func ConnectDB() {
	var err error

	if connStr == "" {
		log.Fatal("GAME_DB_DSN environment variable is not set")
	}
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}

	deadline := time.Now().Add(dbConnectTimeout)
	for {
		if err = DB.Ping(); err == nil {
			log.Println("Подключение к БД установлено")
			return
		}
		if time.Now().After(deadline) {
			log.Fatalf("Невозможно подключиться к БД: %v", err)
		}
		log.Printf("Ожидание готовности БД: %v", err)
		time.Sleep(dbConnectRetryInterval)
	}
}

func RunMigrations() {
	seedLock, err := DB.Conn(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer seedLock.Close()
	if _, err = seedLock.ExecContext(context.Background(), `SELECT pg_advisory_lock(711010,2)`); err != nil {
		log.Fatal(err)
	}
	defer seedLock.ExecContext(context.Background(), `SELECT pg_advisory_unlock(711010,2)`)

	CreateMonstersTable()
	CreateResourcesTable()
	CreateArtifactsTable()

	CreatePlayerProfilesTable()
	CreatePlayerCharactersTable()
	if err := EnsureProfessionsSchema(); err != nil {
		log.Fatalf("profession schema: %v", err)
	}
	CreateMatchesTable()
	CreateMatchMapCellsTable()
	CreateMatchPlayersTable()
	if err := EnsureMatchProfessionXPTable(); err != nil {
		log.Fatalf("match profession XP schema: %v", err)
	}
	EnsureHexGuardianAuraColumns()
	CreateInventoryTable()
	CreateMatchMonstersTable()
	CreatePersistedArtifactsTable()
	CreateEquipmentTables()
	EnsureStaticGameData()
	if err := EnsureMarketSchema(); err != nil {
		log.Fatal(err)
	}
	if err := EnsureProfessionMonsterLootSchema(); err != nil {
		log.Fatal(err)
	}
	EnsureEquipmentSeedData()
	if err := EnsureCraftingSchema(); err != nil {
		log.Fatal(err)
	}
	CreateMatchStatsTable()
	EnsureMatchStatsWinnerUsersColumn()
	CreateMatchPlayerStatsTable()
	EnsureMatchPlayerStatsWinnerColumn()
	EnsureMatchPlayerStatsHistoryColumns()
	EnsureMatchStatsHistoryColumns()
	CreatePlayerFriendsTable()
	CreatePlayerFriendRequestsTable()
	if err := EnsureChatSchema(); err != nil {
		log.Fatal(err)
	}
	EnsurePlayerBaseBuildingsTable()
	EnsureMatchStatsRetentionSchema()
	EnsureQuestArtifactColumn()
	if err := EnsureAutomaticHexSchema(context.Background()); err != nil {
		log.Fatalf("automatic HEX schema: %v", err)
	}
	if err := EnsureHexActionSchema(); err != nil {
		log.Fatal(err)
	}

	CreateMatchPlayerStructuresTable()
}

func EnsureSchemaReady() error {
	if DB == nil {
		return fmt.Errorf("database is not initialized")
	}

	deadline := time.Now().Add(schemaReadyTimeout)
	for {
		ready, err := SchemaReady()
		if err == nil && ready {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("schema readiness check failed: %w", err)
			}
			return fmt.Errorf("required schema is not ready before timeout")
		}
		time.Sleep(dbConnectRetryInterval)
	}
}

func PingDB() error {
	if DB == nil {
		return fmt.Errorf("database is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return DB.PingContext(ctx)
}

func SchemaReady() (bool, error) {
	if err := PingDB(); err != nil {
		return false, err
	}

	requiredTables := []string{
		"monsters",
		"resources",
		"artifacts",
		"player_profiles",
		"player_characters",
		"profession_categories",
		"profession_catalog",
		"character_professions",
		"matches",
		"match_profession_xp",
		"match_profession_monster_drops",
		"match_map_cells",
		"match_players",
		"inventory_items",
		"match_monsters",
		"persisted_artifacts", "profession_craft_operations",
		"item_sets",
		"item_templates",
		"item_set_bonuses",
		"item_instances",
		"character_equipment",
		"item_instance_events",
		"match_stats",
		"match_player_stats",
		"player_friends",
		"player_friend_requests",
		"player_base_buildings",
		"match_player_structures",
	}

	for _, tableName := range requiredTables {
		ok, err := tableExists(tableName)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	requiredColumns := map[string][]string{
		"chat_messages": {"id", "channel", "sender_user_id", "sender_character_id", "sender_nickname", "body", "party_id", "recipient_user_id", "created_at"},
		"chat_mutes":    {"viewer_user_id", "muted_user_id", "created_at"},
		"player_profiles": {
			"user_id",
			"name",
			"image",
			"balance",
			"inventory",
			"selected_character_id",
			"created_at",
			"updated_at",
		},
		"player_characters": {
			"id",
			"user_id",
			"hero_class_id",
			"image",
			"level",
			"exp",
			"max_exp",
			"max_energy",
			"max_health",
			"attack",
			"defense",
			"mobility",
			"agility",
			"sight_range",
			"is_ranged",
			"attack_range",
			"source",
			"created_at",
			"updated_at",
		},
		"match_players": {
			"combat_started", "combat_magic_shield",
			"profession_unlocked", "profession_gather_count", "craft_status", "hex_combat_balance",
			"ap", "max_ap", "mp", "max_mp", "reactions", "max_reactions",
			"personal_turn_counter", "water_available_turn", "hex_armor_break_owner",
			"character_id",
			"character_type",
			"mobility",
			"agility",
			"sight_range",
			"is_ranged",
			"attack_range",
			"image",
			"group_id",
		},
		"matches": {"quest_artifact_id", "combat_ruleset", "combat_config", "combat_rng_seed", "combat_rng_counter"},
		"match_stats": {
			"winner_user_ids",
			"mode",
			"participants",
		},
		"match_player_stats": {
			"is_winner",
			"player_name",
			"group_id",
			"character_type",
			"survived",
			"deaths",
			"damage_taken",
			"placement",
			"inventory_snapshot",
		},
		"player_base_buildings": {
			"tavern_level",
			"guild_level",
		},
		"item_sets": {
			"code",
			"name",
			"class_restriction",
			"rarity",
		},
		"item_templates": {
			"code",
			"set_id",
			"slot",
			"item_type",
			"handedness",
			"rarity",
			"class_restriction",
			"attack_bonus",
			"agility_bonus",
			"max_energy_bonus",
		},
		"item_instances": {
			"id",
			"template_id",
			"owner_user_id",
			"current_character_id",
			"status",
			"version",
		},
		"character_equipment": {
			"character_id",
			"item_instance_id",
			"slot",
		},
		"item_instance_events": {
			"item_instance_id",
			"event_type",
			"metadata",
			"created_at",
		},
	}

	for tableName, columns := range requiredColumns {
		for _, columnName := range columns {
			ok, err := columnExists(tableName, columnName)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
	}
	for _, stage := range []struct {
		name    string
		file    string
		tables  []string
		columns map[string][]string
	}{
		{
			name: "Stage 1", file: "beckend/gameservice/repository/sql/hex_bootstrap/stage1_geometry.sql",
			tables: []string{"hex_stage1_migration_marker", "match_hex_edges"},
			columns: map[string][]string{
				"hex_stage1_migration_marker": {"version"},
				"matches":                     {"schema_version", "geometry", "map_revision", "state_revision", "start_positions_v2", "portal_position_v2"},
				"match_map_cells":             {"q", "r"},
				"match_monsters":              {"pos_q", "pos_r"},
				"match_players":               {"position_q", "position_r"},
				"match_hex_edges":             {"instance_id", "q", "r", "direction", "neighbor_q", "neighbor_r", "wall_kind"},
			},
		},
		{
			name: "Stage 2", file: "beckend/gameservice/repository/sql/hex_bootstrap/stage2_commands.sql",
			tables: []string{"match_commands"},
			columns: map[string][]string{
				"matches":        {"combat_result_events", "combat_results_complete", "finished_at"},
				"match_commands": {"instance_id", "command_id", "user_id", "payload_hash", "response", "state_revision", "map_revision", "created_at"},
			},
		},
	} {
		for _, tableName := range stage.tables {
			ok, err := tableExists(tableName)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, fmt.Errorf("%s schema incomplete: missing table %s; apply %s manually", stage.name, tableName, stage.file)
			}
		}
		for tableName, columns := range stage.columns {
			for _, columnName := range columns {
				ok, err := columnExists(tableName, columnName)
				if err != nil {
					return false, err
				}
				if !ok {
					return false, fmt.Errorf("%s schema incomplete: missing column %s.%s; apply %s manually", stage.name, tableName, columnName, stage.file)
				}
			}
		}
	}

	return true, nil
}

func tableExists(tableName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exists bool
	err := DB.QueryRowContext(
		ctx,
		`SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = current_schema() AND table_name = $1
		)`,
		tableName,
	).Scan(&exists)
	return exists, err
}

func columnExists(tableName, columnName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exists bool
	err := DB.QueryRowContext(
		ctx,
		`SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = $1 AND column_name = $2
		)`,
		tableName,
		columnName,
	).Scan(&exists)
	return exists, err
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
		resource_kind TEXT NOT NULL DEFAULT 'map_object',
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

func CreatePlayerProfilesTable() {
	query := `
	CREATE TABLE IF NOT EXISTS player_profiles (
		user_id INTEGER PRIMARY KEY,
		name TEXT NOT NULL DEFAULT 'Unnamed Player',
		image TEXT NOT NULL DEFAULT '/guardian/guardian.webp',
		balance INTEGER NOT NULL DEFAULT 0,
		inventory JSONB NOT NULL DEFAULT '{}'::jsonb,
		selected_character_id INTEGER,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы player_profiles: %v", err)
	}
}

func CreatePlayerCharactersTable() {
	query := `
	CREATE TABLE IF NOT EXISTS player_characters (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL REFERENCES player_profiles(user_id) ON DELETE CASCADE,
		hero_class_id TEXT NOT NULL,
		image TEXT,
		level INTEGER NOT NULL DEFAULT 1,
		exp INTEGER NOT NULL DEFAULT 0,
		max_exp INTEGER NOT NULL DEFAULT 500,
		max_energy INTEGER NOT NULL,
		max_health INTEGER NOT NULL,
		attack INTEGER NOT NULL,
		defense INTEGER NOT NULL,
		mobility INTEGER NOT NULL,
		agility INTEGER NOT NULL,
		sight_range INTEGER NOT NULL,
		is_ranged BOOLEAN NOT NULL,
		attack_range INTEGER NOT NULL,
		source TEXT NOT NULL DEFAULT 'registration',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE (user_id, hero_class_id),
		CHECK (BTRIM(hero_class_id) <> ''),
		CHECK (hero_class_id <> 'adventurer')
	);
	CREATE INDEX IF NOT EXISTS idx_player_characters_user_id ON player_characters(user_id);
	CREATE INDEX IF NOT EXISTS idx_player_characters_hero_class_id ON player_characters(hero_class_id);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы player_characters: %v", err)
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

func CreatePlayerFriendsTable() {
	query := `
	CREATE TABLE IF NOT EXISTS player_friends (
		user_id INTEGER NOT NULL REFERENCES player_profiles(user_id) ON DELETE CASCADE,
		friend_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id) ON DELETE CASCADE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, friend_user_id),
		CHECK (user_id <> friend_user_id)
	);
	CREATE INDEX IF NOT EXISTS idx_player_friends_user_id ON player_friends(user_id);
	CREATE INDEX IF NOT EXISTS idx_player_friends_friend_user_id ON player_friends(friend_user_id);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы player_friends: %v", err)
	}
}

func CreatePlayerFriendRequestsTable() {
	query := `
	CREATE TABLE IF NOT EXISTS player_friend_requests (
		requester_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id) ON DELETE CASCADE,
		target_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id) ON DELETE CASCADE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (requester_user_id, target_user_id),
		CHECK (requester_user_id <> target_user_id)
	);
	CREATE INDEX IF NOT EXISTS idx_player_friend_requests_target ON player_friend_requests(target_user_id);
	CREATE INDEX IF NOT EXISTS idx_player_friend_requests_requester ON player_friend_requests(requester_user_id);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы player_friend_requests: %v", err)
	}
}

func CreateMatchPlayersTable() {
	query := `
	CREATE TABLE IF NOT EXISTS match_players (
		instance_id TEXT NOT NULL REFERENCES matches(instance_id) ON DELETE CASCADE,
		user_id INTEGER NOT NULL REFERENCES player_profiles(user_id) ON DELETE CASCADE,
		character_id INTEGER NOT NULL,
		name TEXT,
		character_type TEXT NOT NULL DEFAULT 'guardian',
		position JSONB,
		inventory JSONB NOT NULL DEFAULT '{}'::jsonb,
		level INTEGER NOT NULL,
		energy INTEGER NOT NULL,
		max_energy INTEGER NOT NULL,
		health INTEGER NOT NULL,
		max_health INTEGER NOT NULL,
		experience INTEGER NOT NULL,
		max_experience INTEGER NOT NULL,
		attack INTEGER NOT NULL,
		defense INTEGER NOT NULL,
		mobility INTEGER NOT NULL,
		agility INTEGER NOT NULL,
		sight_range INTEGER NOT NULL,
		is_ranged BOOLEAN NOT NULL,
		attack_range INTEGER NOT NULL,
		balance INTEGER NOT NULL DEFAULT 0,
		image TEXT,
		group_id INTEGER,
		PRIMARY KEY (instance_id, user_id)
	);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы match_players: %v", err)
	}
}

func EnsureHexGuardianAuraColumns() {
	_, err := DB.Exec(`ALTER TABLE match_players ADD COLUMN IF NOT EXISTS hex_guardian_aura_pressure INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE match_players ADD COLUMN IF NOT EXISTS hex_guardian_aura_source_user_id INTEGER NOT NULL DEFAULT 0;`)
	if err != nil {
		log.Fatalf("Ошибка добавления состояния ауры Guardian: %v", err)
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
	                    REFERENCES player_profiles(user_id)
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
                       REFERENCES player_profiles(user_id)
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
		SELECT 'water', 'Восстанавливает 1 AP, не выше Max AP. Перезарядка: два личных хода.', '{"ap_half":2}', '/main_resources/water.webp'
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
		UPDATE resources SET description = 'Восстанавливает 1 AP, не выше Max AP. Перезарядка: два личных хода.', effect = '{"ap_half":2}', image = '/main_resources/water.webp' WHERE type = 'water';
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
		SELECT 'berserker_axe', '+2 Attack. -4 Defense while carrying the quest artifact.', '{"attack":2,"defense":-4}'::jsonb, '/artifacts/berserker-axe.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'berserker_axe');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'boots_of_stealth', 'Первый HEX-шаг в личном ходу не расходует MP.', '{"first_step_free":1}'::jsonb, '/artifacts/boots_of_stealth.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'boots_of_stealth');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'crown_of_enlightenment', '+3 Attack. −15 Max Health.', '{"attack":3,"max_health":-15}'::jsonb, '/artifacts/crown-of-enlightenment.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'crown_of_enlightenment');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'dragon_eye', 'Attacks ignore 1 Defense. Lose 3 HP at the end of each turn (cannot reduce HP below 1).', '{"defense_ignore":1,"health_per_turn":-3}'::jsonb, '/artifacts/dragon-eye.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'dragon_eye');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'fire_amulet', 'Successful attacks deal +2 damage, but the carrier takes 3 damage (cannot reduce HP below 1).', '{"bonus_damage":2,"self_damage":3}'::jsonb, '/artifacts/fire-amulet.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'fire_amulet');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'gloves_of_precision', '+1 Attack. -3 Defense. -10 Max HP.', '{"attack":1,"defense":-3,"max_health":-10}'::jsonb, '/artifacts/gloves-of-precision.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'gloves_of_precision');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'guardian_shield', '+2 Defense. −0.5 Max MP.', '{"defense":2,"max_mp_half":-1}'::jsonb, '/artifacts/guardian-shield.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'guardian_shield');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'knight_sword', '+1 Attack and +1 Defense. -20 Max HP.', '{"attack":1,"defense":1,"max_health":-20}'::jsonb, '/artifacts/knight-sword.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'knight_sword');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'ring_of_wisdom', '+1 Reflex. −0.5 Max MP.', '{"agility":1,"max_mp_half":-1}'::jsonb, '/artifacts/ring-of-wisdom.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'ring_of_wisdom');

		INSERT INTO artifacts (name, description, bonus, image)
		SELECT 'titan_breastplate', '+2 Defense. −3 Attack. Каждый HEX-шаг дороже на 0.5 MP.', '{"defense":2,"attack":-3,"step_mp_half":1}'::jsonb, '/artifacts/titan-breastplate.webp'
		WHERE NOT EXISTS (SELECT 1 FROM artifacts WHERE name = 'titan_breastplate');

		UPDATE artifacts SET description = '+2 Attack. -4 Defense while carrying the quest artifact.', bonus = '{"attack":2,"defense":-4}'::jsonb, image = '/artifacts/berserker-axe.webp' WHERE name = 'berserker_axe';
		UPDATE artifacts SET description = 'Первый HEX-шаг в личном ходу не расходует MP.', bonus = '{"first_step_free":1}'::jsonb, image = '/artifacts/boots_of_stealth.webp' WHERE name = 'boots_of_stealth';
		UPDATE artifacts SET description = '+3 Attack. −15 Max Health.', bonus = '{"attack":3,"max_health":-15}'::jsonb, image = '/artifacts/crown-of-enlightenment.webp' WHERE name = 'crown_of_enlightenment';
		UPDATE artifacts SET description = 'Attacks ignore 1 Defense. Lose 3 HP at the end of each turn (cannot reduce HP below 1).', bonus = '{"defense_ignore":1,"health_per_turn":-3}'::jsonb, image = '/artifacts/dragon-eye.webp' WHERE name = 'dragon_eye';
		UPDATE artifacts SET description = 'Successful attacks deal +2 damage, but the carrier takes 3 damage (cannot reduce HP below 1).', bonus = '{"bonus_damage":2,"self_damage":3}'::jsonb, image = '/artifacts/fire-amulet.webp' WHERE name = 'fire_amulet';
		UPDATE artifacts SET description = '+1 Attack. -3 Defense. -10 Max HP.', bonus = '{"attack":1,"defense":-3,"max_health":-10}'::jsonb, image = '/artifacts/gloves-of-precision.webp' WHERE name = 'gloves_of_precision';
		UPDATE artifacts SET description = '+2 Defense. −0.5 Max MP.', bonus = '{"defense":2,"max_mp_half":-1}'::jsonb, image = '/artifacts/guardian-shield.webp' WHERE name = 'guardian_shield';
		UPDATE artifacts SET description = '+1 Attack and +1 Defense. -20 Max HP.', bonus = '{"attack":1,"defense":1,"max_health":-20}'::jsonb, image = '/artifacts/knight-sword.webp' WHERE name = 'knight_sword';
		UPDATE artifacts SET description = '+1 Reflex. −0.5 Max MP.', bonus = '{"agility":1,"max_mp_half":-1}'::jsonb, image = '/artifacts/ring-of-wisdom.webp' WHERE name = 'ring_of_wisdom';
		UPDATE artifacts SET description = '+2 Defense. −3 Attack. Каждый HEX-шаг дороже на 0.5 MP.', bonus = '{"defense":2,"attack":-3,"step_mp_half":1}'::jsonb, image = '/artifacts/titan-breastplate.webp' WHERE name = 'titan_breastplate';
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
        mode             TEXT        NOT NULL DEFAULT '',
        winner_id        INTEGER     NOT NULL,
        winner_group_id  INTEGER     NOT NULL,
		winner_user_ids  JSONB       NOT NULL DEFAULT '[]'::jsonb,
        participants     JSONB       NOT NULL DEFAULT '[]'::jsonb,
        created_at       TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
    );
    `
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы match_stats: %v", err)
	}
}

func EnsureMatchStatsWinnerUsersColumn() {
	_, err := DB.Exec(`ALTER TABLE match_stats ADD COLUMN IF NOT EXISTS winner_user_ids JSONB NOT NULL DEFAULT '[]'::jsonb`)
	if err != nil {
		log.Printf("EnsureMatchStatsWinnerUsersColumn add column: %v", err)
		return
	}

	// Backfill from per-player match stats where winner flags are available.
	_, err = DB.Exec(`
		UPDATE match_stats ms
		SET winner_user_ids = COALESCE(src.winners, '[]'::jsonb)
		FROM (
			SELECT instance_id, jsonb_agg(user_id ORDER BY user_id) AS winners
			FROM match_player_stats
			WHERE is_winner = TRUE
			GROUP BY instance_id
		) AS src
		WHERE ms.instance_id = src.instance_id
		  AND (ms.winner_user_ids IS NULL OR ms.winner_user_ids = '[]'::jsonb)
	`)
	if err != nil {
		log.Printf("EnsureMatchStatsWinnerUsersColumn backfill winners: %v", err)
	}

	// Fallback for solo winners if per-player winner flags are absent.
	_, err = DB.Exec(`
		UPDATE match_stats
		SET winner_user_ids = jsonb_build_array(winner_id)
		WHERE winner_id > 0
		  AND (winner_user_ids IS NULL OR winner_user_ids = '[]'::jsonb)
	`)
	if err != nil {
		log.Printf("EnsureMatchStatsWinnerUsersColumn backfill solo: %v", err)
	}
}

func EnsureMatchStatsHistoryColumns() {
	_, err := DB.Exec(`ALTER TABLE match_stats ADD COLUMN IF NOT EXISTS mode TEXT NOT NULL DEFAULT ''`)
	if err != nil {
		log.Printf("EnsureMatchStatsHistoryColumns mode: %v", err)
		return
	}

	_, err = DB.Exec(`ALTER TABLE match_stats ADD COLUMN IF NOT EXISTS participants JSONB NOT NULL DEFAULT '[]'::jsonb`)
	if err != nil {
		log.Printf("EnsureMatchStatsHistoryColumns participants: %v", err)
		return
	}

	_, err = DB.Exec(`
		UPDATE match_stats ms
		SET participants = COALESCE(src.participants, '[]'::jsonb)
		FROM (
			SELECT
				mps.instance_id,
				jsonb_agg(
					jsonb_build_object(
						'userId', mps.user_id,
						'name', CASE
							WHEN COALESCE(mps.player_name, '') <> '' THEN mps.player_name
							ELSE 'Player ' || mps.user_id::text
						END,
						'groupId', COALESCE(mps.group_id, 0),
						'characterType', COALESCE(NULLIF(mps.character_type, ''), 'guardian'),
						'placement', COALESCE(mps.placement, 0),
						'isWinner', COALESCE(mps.is_winner, FALSE),
						'survived', COALESCE(mps.survived, FALSE)
					)
					ORDER BY COALESCE(NULLIF(mps.placement, 0), 9999), mps.user_id
				) AS participants
			FROM match_player_stats mps
			GROUP BY mps.instance_id
		) AS src
		WHERE ms.instance_id = src.instance_id
		  AND (ms.participants IS NULL OR ms.participants = '[]'::jsonb)
	`)
	if err != nil {
		log.Printf("EnsureMatchStatsHistoryColumns backfill participants: %v", err)
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
        player_name        TEXT     NOT NULL DEFAULT '',
        group_id           INTEGER  NOT NULL DEFAULT 0,
		character_type     TEXT     NOT NULL DEFAULT 'guardian',
		is_winner          BOOLEAN  NOT NULL DEFAULT FALSE,
        survived           BOOLEAN  NOT NULL DEFAULT FALSE,
        deaths             INTEGER  NOT NULL DEFAULT 0,
        exp_gained         INTEGER  NOT NULL,
        rewards            JSONB    NOT NULL,
        player_kills       INTEGER  NOT NULL,
        monster_kills      INTEGER  NOT NULL,
        damage_total       INTEGER  NOT NULL,
        damage_to_players  INTEGER  NOT NULL,
        damage_to_monsters INTEGER  NOT NULL,
        damage_taken       INTEGER  NOT NULL DEFAULT 0,
        placement          INTEGER  NOT NULL DEFAULT 0,
        inventory_snapshot JSONB    NOT NULL DEFAULT '{}'::jsonb,
        PRIMARY KEY (instance_id, user_id)
    );
    `
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы match_player_stats: %v", err)
	}
}

func EnsureMatchPlayerStatsWinnerColumn() {
	_, err := DB.Exec(`ALTER TABLE match_player_stats ADD COLUMN IF NOT EXISTS is_winner BOOLEAN NOT NULL DEFAULT FALSE`)
	if err != nil {
		log.Printf("EnsureMatchPlayerStatsWinnerColumn add column: %v", err)
		return
	}

	// Backfill historical solo wins where winner_id was stored explicitly.
	_, err = DB.Exec(`
        UPDATE match_player_stats mps
        SET is_winner = TRUE
        FROM match_stats ms
        WHERE ms.instance_id = mps.instance_id
          AND ms.winner_id > 0
          AND ms.winner_id = mps.user_id
    `)
	if err != nil {
		log.Printf("EnsureMatchPlayerStatsWinnerColumn backfill: %v", err)
	}
}

func EnsureMatchPlayerStatsHistoryColumns() {
	commands := []string{
		`ALTER TABLE match_player_stats ADD COLUMN IF NOT EXISTS player_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE match_player_stats ADD COLUMN IF NOT EXISTS group_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE match_player_stats ADD COLUMN IF NOT EXISTS character_type TEXT NOT NULL DEFAULT 'guardian'`,
		`ALTER TABLE match_player_stats ADD COLUMN IF NOT EXISTS survived BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE match_player_stats ADD COLUMN IF NOT EXISTS deaths INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE match_player_stats ADD COLUMN IF NOT EXISTS damage_taken INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE match_player_stats ADD COLUMN IF NOT EXISTS placement INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE match_player_stats ADD COLUMN IF NOT EXISTS inventory_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb`,
	}

	for _, command := range commands {
		if _, err := DB.Exec(command); err != nil {
			log.Printf("EnsureMatchPlayerStatsHistoryColumns: %v", err)
			return
		}
	}

	_, err := DB.Exec(`
		UPDATE match_player_stats
		SET
			player_name = CASE
				WHEN COALESCE(player_name, '') <> '' THEN player_name
				ELSE 'Player ' || user_id::text
			END,
			character_type = COALESCE(NULLIF(character_type, ''), 'guardian')
	`)
	if err != nil {
		log.Printf("EnsureMatchPlayerStatsHistoryColumns backfill player info: %v", err)
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

func placementLess(a, b game.PlayerResult, defeatRank map[int]int) bool {
	if a.IsWinner != b.IsWinner {
		return a.IsWinner
	}
	if a.Survived != b.Survived {
		return a.Survived
	}
	aRank, aOK := defeatRank[a.UserID]
	bRank, bOK := defeatRank[b.UserID]
	if aOK != bOK {
		return aOK
	}
	if aOK && bOK && aRank != bRank {
		return aRank < bRank
	}
	return a.UserID < b.UserID
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func applySoloPlacements(results []game.PlayerResult, defeatedUsers []int) {
	defeatRank := make(map[int]int, len(defeatedUsers))
	for idx := len(defeatedUsers) - 1; idx >= 0; idx-- {
		userID := defeatedUsers[idx]
		if _, exists := defeatRank[userID]; exists {
			continue
		}
		defeatRank[userID] = len(defeatRank)
	}

	sorted := append([]game.PlayerResult(nil), results...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return placementLess(sorted[i], sorted[j], defeatRank)
	})

	placementByUser := make(map[int]int, len(sorted))
	for idx, result := range sorted {
		placementByUser[result.UserID] = idx + 1
	}

	for i := range results {
		results[i].Placement = placementByUser[results[i].UserID]
	}
}

func applyTeamPlacements(results []game.PlayerResult, defeatedUsers []int) {
	type groupScore struct {
		GroupID            int
		IsWinner           bool
		HasSurvivor        bool
		LastDefeatPosition int
	}

	groupMembers := make(map[int][]int)
	groupScores := make(map[int]*groupScore)
	for idx := range results {
		groupID := results[idx].GroupID
		if groupID <= 0 {
			groupID = results[idx].UserID
		}
		groupMembers[groupID] = append(groupMembers[groupID], results[idx].UserID)

		score := groupScores[groupID]
		if score == nil {
			score = &groupScore{
				GroupID:            groupID,
				LastDefeatPosition: -1,
			}
			groupScores[groupID] = score
		}
		if results[idx].IsWinner {
			score.IsWinner = true
		}
		if results[idx].Survived {
			score.HasSurvivor = true
		}
	}

	defeatPosition := make(map[int]int, len(defeatedUsers))
	for idx, userID := range defeatedUsers {
		defeatPosition[userID] = idx
	}

	for groupID, members := range groupMembers {
		score := groupScores[groupID]
		for _, userID := range members {
			if pos, ok := defeatPosition[userID]; ok && pos > score.LastDefeatPosition {
				score.LastDefeatPosition = pos
			}
		}
	}

	scores := make([]*groupScore, 0, len(groupScores))
	for _, score := range groupScores {
		scores = append(scores, score)
	}

	sort.SliceStable(scores, func(i, j int) bool {
		left := scores[i]
		right := scores[j]
		if left.IsWinner != right.IsWinner {
			return left.IsWinner
		}
		if left.HasSurvivor != right.HasSurvivor {
			return left.HasSurvivor
		}
		if left.LastDefeatPosition != right.LastDefeatPosition {
			return left.LastDefeatPosition > right.LastDefeatPosition
		}
		return left.GroupID < right.GroupID
	})

	placementByGroup := make(map[int]int, len(scores))
	for idx, score := range scores {
		placementByGroup[score.GroupID] = idx + 1
	}

	for i := range results {
		groupID := results[i].GroupID
		if groupID <= 0 {
			groupID = results[i].UserID
		}
		results[i].Placement = placementByGroup[groupID]
	}
}

func assignPlacements(results []game.PlayerResult, defeatedUsers []int) {
	groupSizes := make(map[int]int)
	teamMode := false
	for _, result := range results {
		groupSizes[result.GroupID]++
		if result.GroupID > 0 && groupSizes[result.GroupID] > 1 {
			teamMode = true
		}
	}

	if teamMode {
		applyTeamPlacements(results, defeatedUsers)
		return
	}

	applySoloPlacements(results, defeatedUsers)
}

// applyWinnerBonusesAndPlacements preserves the reward and placement rules for HEX results.
func applyWinnerBonusesAndPlacements(mr *game.MatchResults, groupByUser map[int]int, defeatedUsers []int) {
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
		mr.PlayerResults[i].IsWinner = isWinner
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

	winnerUserIDs := make([]int, 0)
	for i := range mr.PlayerResults {
		if mr.PlayerResults[i].IsWinner {
			winnerUserIDs = append(winnerUserIDs, mr.PlayerResults[i].UserID)
		}
	}
	mr.WinnerUserIDs = winnerUserIDs
	assignPlacements(mr.PlayerResults, defeatedUsers)

}
