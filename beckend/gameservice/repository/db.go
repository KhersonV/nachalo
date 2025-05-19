// ==============================
// /gameservice/repository/db.go
// ==============================

package repository

import (
	"database/sql"
	"encoding/json"
	"log"

	_ "github.com/lib/pq"
	"gameservice/models"
	"gameservice/game"
)

const connStr = "user=admin password=admin dbname=game_db sslmode=disable"

var DB *sql.DB
func InitDB() {
    var err error
    DB, err = sql.Open("postgres", connStr)
    if err != nil {
        log.Fatalf("Ошибка подключения к БД: %v", err)
    }
    if err = DB.Ping(); err != nil {
        log.Fatalf("Невозможно подключиться к БД: %v", err)
    }
    log.Println("Подключение к БД установлено")

    // Существующие таблицы
    CreatePlayersTable()
    CreateMatchesTable()
    CreateMatchPlayersTable()
    CreateInventoryTable()
    // Добавляем нашу новую
    CreateMatchMonstersTable()

    // Восстанавливаем state матчей
    if err := RestoreMatchStates(); err != nil {
        log.Fatalf("Ошибка восстановления матчей: %v", err)
    }
}


func CreatePlayersTable() {
	query := `
	CREATE TABLE IF NOT EXISTS players (
		user_id SERIAL PRIMARY KEY,
		name TEXT DEFAULT 'Unnamed Player',
		image TEXT DEFAULT '/player-1.webp',
		position JSONB DEFAULT '{"x": 0, "y": 0}',
		energy INTEGER DEFAULT 100,
		max_energy INTEGER DEFAULT 100,
		health INTEGER DEFAULT 100,
		max_health INTEGER DEFAULT 100,
		level INTEGER DEFAULT 1,
		experience INTEGER DEFAULT 0,
		max_experience INTEGER DEFAULT 500,
		attack INTEGER DEFAULT 10,
		defense INTEGER DEFAULT 5,
		speed INTEGER DEFAULT 3,
		maneuverability INTEGER DEFAULT 2,
		vision INTEGER DEFAULT 2,
		vision_range INTEGER DEFAULT 2,
		balance INTEGER DEFAULT 0,
		inventory JSONB DEFAULT '{}'
	);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы players: %v", err)
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
		turn_order JSONB,
		turn_number INTEGER DEFAULT 1,
		map_height INTEGER NOT NULL,
		map_width INTEGER NOT NULL,
		map JSONB NOT NULL,
		start_positions JSONB,
		portal_position JSONB
	);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы matches: %v", err)
	}
}

func CreateMatchPlayersTable() {
	query := `
	CREATE TABLE IF NOT EXISTS match_players (
		match_instance_id TEXT NOT NULL REFERENCES matches(instance_id) ON DELETE CASCADE,
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
		speed INTEGER,
		maneuverability INTEGER,
		vision INTEGER,
		vision_range INTEGER,
		balance INTEGER,
        image TEXT,
		group_id INTEGER,
		PRIMARY KEY (match_instance_id, user_id)
	);
	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы match_players: %v", err)
	}
}


func CreateInventoryTable() {
	query := `
	CREATE TABLE IF NOT EXISTS inventory_items (
  id SERIAL PRIMARY KEY,
  instance_id TEXT NOT NULL
    REFERENCES matches(instance_id)
    ON DELETE CASCADE,
  user_id INTEGER NOT NULL,
  item_type TEXT NOT NULL,
  item_id INTEGER NOT NULL,
  item_count INTEGER NOT NULL DEFAULT 0,
  UNIQUE (instance_id, user_id, item_type, item_id)
);

	`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы inventory_items: %v", err)
	}
}

func CreateMatchMonstersTable() {
  query := `
  CREATE TABLE IF NOT EXISTS match_monsters (
    match_instance_id TEXT NOT NULL REFERENCES matches(instance_id) ON DELETE CASCADE,
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
    PRIMARY KEY (match_instance_id, monster_instance_id)
  );
  `
  if _, err := DB.Exec(query); err != nil {
    log.Fatalf("Ошибка создания таблицы match_monsters: %v", err)
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
			InstanceID:     instanceID,
			ActiveUserID: activeUserID,
			TurnOrder:      turnOrder,
		}
		game.MatchStatesMu.Lock()
		game.MatchStates[instanceID] = matchState
		game.MatchStatesMu.Unlock()

		log.Printf("Состояние матча %s восстановлено", instanceID)
	}
	return nil
}

// Пример обновлённой функции получения данных игрока из match_players,
// которая формирует структуру PlayerResponse.

func GetMatchPlayerByUserID(userID int) (*models.PlayerResponse, error) {
	query := `
		SELECT 
			user_id,
			name, 
			image, 
			position, 
			inventory,
			level,
			energy,
			max_energy,
			health,
			max_health,
			experience,
			max_experience,
			attack,
			defense,
			speed,
			maneuverability,
			vision,
			vision_range,
			group_id,
			balance
		FROM match_players
		WHERE user_id = $1
		LIMIT 1;
	`
	var pr models.PlayerResponse
	var positionJSON []byte

	err := DB.QueryRow(query, userID).Scan(
		&pr.UserID,    // user_id
		&pr.Name,      // name
		&pr.Image,     // image
		&positionJSON, // position (JSONB)
		&pr.Inventory, // inventory (JSONB, как строка)
		&pr.Level,     // level
		&pr.Energy,    // energy
		&pr.MaxEnergy, // max_energy
		&pr.Health,    // health
		&pr.MaxHealth, // max_health
		&pr.Experience,    // experience
		&pr.MaxExperience, // max_experience
		&pr.Attack,        // attack
		&pr.Defense,       // defense
		&pr.Speed,         // speed
		&pr.Maneuverability, // maneuverability
		&pr.Vision,          // vision
		&pr.VisionRange,     // vision_range
		&pr.GroupID, 		// group_id
		&pr.Balance,         // balance
	)
	if err != nil {
		return nil, err
	}

	// Распарсить JSON из поля position в структуру Position модели
	if err := json.Unmarshal(positionJSON, &pr.Position); err != nil {
		return nil, err
	}

	return &pr, nil
}
