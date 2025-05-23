// ==============================
// /gameservice/repository/db.go
// ==============================

package repository

import (
	"database/sql"
	"encoding/json"
	"log"
	"fmt"

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
	CreatePersistedArtifactsTable()
	CreateMatchStatsTable()
	CreateMatchPlayerStatsTable()

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
		winner_id INTEGER,
		winner_group_id INTEGER,
		turn_order JSONB,
		turn_number INTEGER DEFAULT 1,
		start_positions JSONB,
		portal_position JSONB,
		map_height INTEGER NOT NULL,
		map_width INTEGER NOT NULL,
		map JSONB NOT NULL
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


// GetMatchResults читает из БД и игровой логики всё, что нужно для финализации матча.
func GetMatchResults(instanceID string) (*game.MatchResults, error) {
	log.Printf("[GetMatchResults] fetching for match %s", instanceID)

    var mr game.MatchResults



	// Предполагаем, что в game.MatchResults поля WinnerID и WinnerGroupID — это plain int,
    // и отсутствующий победитель будет обозначаться как -1.
    
    // 1) Победители из таблицы matches, NULL → -1
    if err := DB.QueryRow(
        `SELECT
            COALESCE(winner_id, -1),
            COALESCE(winner_group_id, -1)
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
        `SELECT user_id
           FROM match_players
          WHERE match_instance_id = $1`,
        instanceID,
    )
    if err != nil {
        return nil, fmt.Errorf("GetMatchResults: не удалось получить игроков: %w", err)
    }
    defer rows.Close()

    // 3) Для каждого игрока — игровая логика CalculateResults
    for rows.Next() {
        var userID int
        if err := rows.Scan(&userID); err != nil {
            log.Printf("GetMatchResults: пропускаем игрока из-за Scan: %v", err)
            continue
        }

        exp, rewards, killsP, killsM, dmgTotal, dmgPlayers, dmgMonsters, err := 
            game.CalculateResults(instanceID, userID)
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
log.Printf("[GetMatchResults] found %d players for match %s", len(mr.PlayerResults), instanceID)
    if len(mr.PlayerResults) == 0 {
        return nil, fmt.Errorf("GetMatchResults: не найдено ни одного игрока для матча %s", instanceID)
    }

    return &mr, nil
}

