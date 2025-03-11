
// ==============================
// /gameservice/repository/db.go
// ==============================

package repository

import (
    "database/sql"
    "log"
    "encoding/json"
    
    _ "github.com/lib/pq"
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
    err = DB.Ping()
    if err != nil {
        log.Fatalf("Невозможно подключиться к БД: %v", err)
    }
    log.Println("Подключение к БД установлено")
}

func CreatePlayersTable() {
    query := `
    CREATE TABLE IF NOT EXISTS players (
        id SERIAL PRIMARY KEY,
        user_id INTEGER UNIQUE NOT NULL,
        name TEXT DEFAULT 'Unnamed Player',
        image TEXT DEFAULT '/default-player.webp',
        color_class TEXT DEFAULT 'default-player',
        pos_x INTEGER DEFAULT 0,
        pos_y INTEGER DEFAULT 0,
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
        inventory JSONB DEFAULT '{}',
        instance_id TEXT,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );
    `
    _, err := DB.Exec(query)
    if err != nil {
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
        active_player_id INTEGER,
        turn_order JSONB,
        turn_number INTEGER DEFAULT 1,
        map_height INTEGER NOT NULL,
        map_width INTEGER NOT NULL,
        map JSONB NOT NULL,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );
    `
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы matches: %v", err)
	}
}

func CreateMatchPlayersTable() {
    query := `
    CREATE TABLE IF NOT EXISTS match_players (
        id SERIAL PRIMARY KEY,
        match_instance_id TEXT NOT NULL REFERENCES matches(instance_id) ON DELETE CASCADE,
        player_id INTEGER NOT NULL,
        name TEXT,
        image TEXT,
        color_class TEXT,
        pos_x INTEGER,
        pos_y INTEGER,
        energy INTEGER,
        max_energy INTEGER,
        health INTEGER,
        max_health INTEGER,
        level INTEGER,
        experience INTEGER,
        max_experience INTEGER,
        attack INTEGER,
        defense INTEGER,
        speed INTEGER,
        maneuverability INTEGER,
        vision INTEGER,
        vision_range INTEGER,
        balance INTEGER,
        inventory JSONB,
        group_id INTEGER,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );
    `
    if _, err := DB.Exec(query); err != nil {
        log.Fatalf("Ошибка создания таблицы match_players: %v", err)
    }
}

// Пример: если вам нужна таблица inventory_items
func CreateInventoryTable() {
    query := `
    CREATE TABLE IF NOT EXISTS inventory_items (
        id SERIAL PRIMARY KEY,
        player_id INTEGER NOT NULL,
        item_type TEXT NOT NULL,
        item_id INTEGER NOT NULL,
        count INTEGER NOT NULL DEFAULT 0
    );
    `
    if _, err := DB.Exec(query); err != nil {
        log.Fatalf("Ошибка создания таблицы inventory_items: %v", err)
    }
}

func RestoreMatchStates() error {
    rows, err := DB.Query("SELECT instance_id, active_player_id, turn_order FROM matches")
    if err != nil {
        return err
    }
    defer rows.Close()

    for rows.Next() {
        var instanceID string
        var activePlayerID int
        var turnOrderJSON []byte
        if err := rows.Scan(&instanceID, &activePlayerID, &turnOrderJSON); err != nil {
            return err
        }

        var turnOrder []int
        if err := json.Unmarshal(turnOrderJSON, &turnOrder); err != nil {
            log.Printf("Ошибка восстановления состояния матча %s: %v", instanceID, err)
            continue
        }

        matchState := &game.MatchState{
            InstanceID:     instanceID,
            ActivePlayerID: activePlayerID,
            TurnOrder:      turnOrder,
        }
        game.MatchStatesMu.Lock()
        game.MatchStates[instanceID] = matchState
        game.MatchStatesMu.Unlock()

        log.Printf("Состояние матча %s восстановлено", instanceID)
    }
    return nil
}

// Прочие функции для инициализации и т.п.
