
// Game_Servise


package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	// Подключаем зависимости.
	_ "github.com/lib/pq"
	"github.com/gorilla/mux"

	// Локальный импорт пакета mapgen (убедитесь, что путь правильный)
	"gameservice/mapgen"
)

// Строка подключения к базе game_db.
// Обновите user и password согласно настройкам вашей системы.
const connStr = "user=admin password=admin dbname=game_db sslmode=disable"

// Глобальная переменная для подключения к БД.
var db *sql.DB

// Карта пороговых значений опыта для каждого уровня (1-10).
var levelThresholds = map[int]int{
	1: 500,
	2: 2000,
	3: 8000,
	4: 32000,
	5: 128000,
	6: 512000,
	7: 2048000,
	8: 8192000,
	9: 32768000,
	10: 131072000,
}

// ----------------- МОДЕЛИ ------------------

type Player struct {
	ID              int            `json:"id"`
	UserID          int            `json:"user_id"`
	Name            string         `json:"name"`
	Image           string         `json:"image"`
	ColorClass      string         `json:"color_class"`
	PosX            int            `json:"pos_x"`
	PosY            int            `json:"pos_y"`
	Energy          int            `json:"energy"`
	MaxEnergy       int            `json:"max_energy"`
	Health          int            `json:"health"`
	MaxHealth       int            `json:"max_health"`
	Level           int            `json:"level"`
	Experience      int            `json:"experience"`
	MaxExperience   int            `json:"max_experience"`
	Attack          int            `json:"attack"`
	Defense         int            `json:"defense"`
	Speed           int            `json:"speed"`
	Maneuverability int            `json:"maneuverability"`
	Vision          int            `json:"vision"`
	VisionRange     int            `json:"vision_range"`
	Balance         int            `json:"balance"`
	Inventory       string         `json:"inventory"` // JSON-представление инвентаря
	InstanceID      sql.NullString `json:"instance_id"` // для привязки к матчу (если используется)
	UpdatedAt       time.Time      `json:"updated_at"`
}

type Monster struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	Health          int       `json:"health"`
	MaxHealth       int       `json:"max_health"`
	Attack          int       `json:"attack"`
	Defense         int       `json:"defense"`
	Speed           int       `json:"speed"`
	Maneuverability int       `json:"maneuverability"`
	Vision          int       `json:"vision"`
	CreatedAt       time.Time `json:"created_at"`
}

// MatchInfo – структура для хранения данных матча
type MatchInfo struct {
	InstanceID   string          `json:"instance_id"`
	Mode         string          `json:"mode"`
	TeamsCount   int             `json:"teams_count"`
	TotalPlayers int             `json:"total_players"`
	MapWidth     int             `json:"map_width"`
	MapHeight    int             `json:"map_height"`
	Map          json.RawMessage `json:"map"` // Карта в формате JSON
	CreatedAt    time.Time       `json:"created_at"`
}

// ----------------- ИНИЦИАЛИЗАЦИЯ БД ------------------

func initDB() {
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatalf("Невозможно подключиться к БД: %v", err)
	}
	log.Println("Подключение к БД установлено")
}

func createPlayersTable() {
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
		vision INTEGER DEFAULT 5,
		vision_range INTEGER DEFAULT 5,
		balance INTEGER DEFAULT 0,
		inventory JSONB DEFAULT '{}',
		instance_id TEXT,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы players: %v", err)
	}
}

func createMatchesTable() {
	query := `
	CREATE TABLE IF NOT EXISTS matches (
		instance_id TEXT PRIMARY KEY,
		mode TEXT NOT NULL,
		teams_count INTEGER NOT NULL,
		total_players INTEGER NOT NULL,
		map_width INTEGER NOT NULL,
		map_height INTEGER NOT NULL,
		map JSONB NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы matches: %v", err)
	}
}

// Создаем таблицу для копий игроков в матчах.
func createMatchPlayersTable() {
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
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(query); err != nil {
		log.Fatalf("Ошибка создания таблицы match_players: %v", err)
	}
}

// Функция для вставки копии игрока в таблицу match_players.
func createMatchPlayerCopy(matchID string, p *Player) error {
	query := `
		INSERT INTO match_players (
			match_instance_id, player_id, name, image, color_class, pos_x, pos_y, energy, max_energy,
			health, max_health, level, experience, max_experience, attack, defense, speed, maneuverability,
			vision, vision_range, balance, inventory, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15, $16, $17, $18,
			$19, $20, $21, $22, $23
		)
	`
	_, err := db.Exec(query, matchID, p.ID, p.Name, p.Image, p.ColorClass, p.PosX, p.PosY,
		p.Energy, p.MaxEnergy, p.Health, p.MaxHealth, p.Level, p.Experience, p.MaxExperience,
		p.Attack, p.Defense, p.Speed, p.Maneuverability, p.Vision, p.VisionRange, p.Balance,
		p.Inventory, p.UpdatedAt)
	return err
}

// ----------------- ОБРАБОТЧИКИ ДЛЯ МАТЧЕЙ ------------------

// Добавляем поле PlayerIDs для передачи списка идентификаторов игроков, участвующих в матче.
type RequestMatch struct {
	InstanceID   string `json:"instance_id"`
	Mode         string `json:"mode"`
	TotalPlayers int    `json:"total_players"` // Общее число игроков в матче
	TeamsCount   int    `json:"teams_count"`   // Количество команд
	PlayerIDs    []int  `json:"player_ids"`    // Список ID игроков, участвующих в матче
}

func createMatchHandler(w http.ResponseWriter, r *http.Request) {
	var req RequestMatch
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	cfg := mapgen.MapConfig{
		TotalPlayers: req.TotalPlayers,
		TeamsCount:   req.TeamsCount,
		WalkableProb: 0.8,
		ResourceProb: 0.1,
		MonsterProb:  0.05,
	}

	grid, err := mapgen.GenerateMap(cfg)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка генерации карты: %v", err), http.StatusInternalServerError)
		return
	}

	// Сохраняем матч в БД
	matchJSON, err := json.Marshal(grid)
	if err != nil {
		http.Error(w, "Ошибка маршалинга карты", http.StatusInternalServerError)
		return
	}
	createdAt := time.Now()

	// Для примера считаем, что ширина и высота карты равны размеру сгенерированной сетки.
	mapWidth := 15 * req.TotalPlayers
	mapHeight := 15 * req.TotalPlayers

	query := `
		INSERT INTO matches (instance_id, mode, teams_count, total_players, map_width, map_height, map, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING instance_id
	`
	var instanceID string
	err = db.QueryRow(query, req.InstanceID, req.Mode, req.TeamsCount, req.TotalPlayers, mapWidth, mapHeight, matchJSON, createdAt).Scan(&instanceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка сохранения матча: %v", err), http.StatusInternalServerError)
		return
	}

	// Создаем копии игроков для матча.
	for _, pid := range req.PlayerIDs {
		player, err := getPlayerByID(pid)
		if err != nil {
			log.Printf("Ошибка получения игрока с ID %d: %v", pid, err)
			continue // Можно также вернуть ошибку, если это критично
		}
		if err := createMatchPlayerCopy(instanceID, player); err != nil {
			log.Printf("Ошибка копирования игрока с ID %d для матча %s: %v", pid, instanceID, err)
		}
	}

	response := map[string]interface{}{
		"instance_id":   instanceID,
		"mode":          req.Mode,
		"map":           grid,
		"created_at":    createdAt,
		"map_width":     mapWidth,
		"map_height":    mapHeight,
		"player_ids":    req.PlayerIDs,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getMatchHandler(w http.ResponseWriter, r *http.Request) {
	instanceID := r.URL.Query().Get("instance_id")
	if instanceID == "" {
		http.Error(w, "instance_id обязателен", http.StatusBadRequest)
		return
	}

	// Получаем данные матча
	query := `
		SELECT instance_id, mode, teams_count, total_players, map_width, map_height, map, created_at
		FROM matches
		WHERE instance_id = $1
	`
	var match MatchInfo
	err := db.QueryRow(query, instanceID).Scan(
		&match.InstanceID,
		&match.Mode,
		&match.TeamsCount,
		&match.TotalPlayers,
		&match.MapWidth,
		&match.MapHeight,
		&match.Map,
		&match.CreatedAt,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Матч не найден: %v", err), http.StatusNotFound)
		return
	}

	// Получаем список игроков из таблицы players, участвующих в матче.
	playersQuery := `
		SELECT id, user_id, name, image, color_class, pos_x, pos_y, energy, max_energy, health, max_health, level, experience, max_experience,
		       attack, defense, speed, maneuverability, vision, vision_range, balance, inventory, updated_at
		FROM players
		WHERE instance_id = $1
	`
	rows, err := db.Query(playersQuery, instanceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения игроков: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var players []Player
	for rows.Next() {
		var p Player
		if err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.Name,
			&p.Image,
			&p.ColorClass,
			&p.PosX,
			&p.PosY,
			&p.Energy,
			&p.MaxEnergy,
			&p.Health,
			&p.MaxHealth,
			&p.Level,
			&p.Experience,
			&p.MaxExperience,
			&p.Attack,
			&p.Defense,
			&p.Speed,
			&p.Maneuverability,
			&p.Vision,
			&p.VisionRange,
			&p.Balance,
			&p.Inventory,
			&p.UpdatedAt,
		); err != nil {
			http.Error(w, fmt.Sprintf("Ошибка чтения данных игрока: %v", err), http.StatusInternalServerError)
			return
		}
		players = append(players, p)
	}

	response := map[string]interface{}{
		"instance_id":   match.InstanceID,
		"mode":          match.Mode,
		"teams_count":   match.TeamsCount,
		"total_players": match.TotalPlayers,
		"map_width":     match.MapWidth,
		"map_height":    match.MapHeight,
		"map":           match.Map,
		"players":       players,
		"created_at":    match.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ----------------- ОБРАБОТЧИКИ ДЛЯ PLAYERS ------------------

func createPlayerHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID int `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	query := `
		INSERT INTO players (user_id, name, image, color_class, pos_x, pos_y, energy, max_energy, health, max_health, level, experience, max_experience, attack, defense, speed, maneuverability, vision, vision_range, balance, inventory)
		VALUES ($1, 'Player', '/player-1.webp', 'red-player', 0, 0, 100, 100, 100, 100, 1, 0, 500, 10, 5, 3, 2, 5, 5, 0, '{}')
		RETURNING id, user_id, name, image, color_class, pos_x, pos_y, energy, max_energy, health, max_health, level, experience, max_experience, attack, defense, speed, maneuverability, vision, vision_range, balance, inventory, updated_at
	`
	player := &Player{}
	err := db.QueryRow(query, req.UserID).Scan(
		&player.ID,
		&player.UserID,
		&player.Name,
		&player.Image,
		&player.ColorClass,
		&player.PosX,
		&player.PosY,
		&player.Energy,
		&player.MaxEnergy,
		&player.Health,
		&player.MaxHealth,
		&player.Level,
		&player.Experience,
		&player.MaxExperience,
		&player.Attack,
		&player.Defense,
		&player.Speed,
		&player.Maneuverability,
		&player.Vision,
		&player.VisionRange,
		&player.Balance,
		&player.Inventory,
		&player.UpdatedAt,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка создания игрока: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(player)
}

func getPlayerHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Некорректный ID", http.StatusBadRequest)
		return
	}
	player, err := getPlayerByID(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения игрока: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(player)
}

func gainExperienceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Некорректный ID", http.StatusBadRequest)
		return
	}
	player, err := getPlayerByID(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения игрока: %v", err), http.StatusInternalServerError)
		return
	}
	var req struct {
		Experience int `json:"experience"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}
	player.Experience += req.Experience
	if threshold, ok := levelThresholds[player.Level]; ok && player.Experience >= threshold {
		player.Level++
		player.Experience = 0
		log.Printf("Игрок %d повышен до уровня %d", player.ID, player.Level)
	}
	err = updatePlayer(player)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка обновления игрока: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(player)
}

func getPlayerByID(id int) (*Player, error) {
	player := &Player{}
	query := `
		SELECT id, user_id, name, image, color_class, pos_x, pos_y, energy, max_energy, health, max_health, level, experience, max_experience, attack, defense, speed, maneuverability, vision, vision_range, balance, inventory, updated_at
		FROM players
		WHERE id = $1
	`
	row := db.QueryRow(query, id)
	err := row.Scan(
		&player.ID,
		&player.UserID,
		&player.Name,
		&player.Image,
		&player.ColorClass,
		&player.PosX,
		&player.PosY,
		&player.Energy,
		&player.MaxEnergy,
		&player.Health,
		&player.MaxHealth,
		&player.Level,
		&player.Experience,
		&player.MaxExperience,
		&player.Attack,
		&player.Defense,
		&player.Speed,
		&player.Maneuverability,
		&player.Vision,
		&player.VisionRange,
		&player.Balance,
		&player.Inventory,
		&player.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return player, nil
}

func updatePlayer(player *Player) error {
	query := `
		UPDATE players
		SET pos_x = $1, pos_y = $2, energy = $3, health = $4, level = $5, experience = $6,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $7
	`
	_, err := db.Exec(query, player.PosX, player.PosY, player.Energy, player.Health,
		player.Level, player.Experience, player.ID)
	return err
}

// ----------------- ОБРАБОТЧИКИ ДЛЯ ИНВЕНТАРЯ ------------------

func addInventoryItem(playerID int, itemType string, itemID int, count int) error {
	var existingCount int
	err := db.QueryRow(`
        SELECT count FROM inventory_items 
        WHERE player_id = $1 AND item_type = $2 AND item_id = $3
    `, playerID, itemType, itemID).Scan(&existingCount)
	if err != nil {
		if err == sql.ErrNoRows {
			_, err = db.Exec(`
                INSERT INTO inventory_items (player_id, item_type, item_id, count)
                VALUES ($1, $2, $3, $4)
            `, playerID, itemType, itemID, count)
			return err
		}
		return err
	}
	newCount := existingCount + count
	_, err = db.Exec(`
        UPDATE inventory_items SET count = $1 
        WHERE player_id = $2 AND item_type = $3 AND item_id = $4
    `, newCount, playerID, itemType, itemID)
	return err
}

func removeInventoryItem(playerID int, itemType string, itemID int, count int) error {
	var existingCount int
	err := db.QueryRow(`
        SELECT count FROM inventory_items 
        WHERE player_id = $1 AND item_type = $2 AND item_id = $3
    `, playerID, itemType, itemID).Scan(&existingCount)
	if err != nil {
		return err
	}
	newCount := existingCount - count
	if newCount <= 0 {
		_, err = db.Exec(`
            DELETE FROM inventory_items 
            WHERE player_id = $1 AND item_type = $2 AND item_id = $3
        `, playerID, itemType, itemID)
		return err
	}
	_, err = db.Exec(`
        UPDATE inventory_items SET count = $1 
        WHERE player_id = $2 AND item_type = $3 AND item_id = $4
    `, newCount, playerID, itemType, itemID)
	return err
}

func addInventoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
		return
	}
	var req struct {
		ItemType string `json:"item_type"`
		ItemID   int    `json:"item_id"`
		Count    int    `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}
	if req.Count <= 0 {
		req.Count = 1
	}
	err = addInventoryItem(playerID, req.ItemType, req.ItemID, req.Count)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка добавления предмета: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Предмет успешно добавлен"))
}

func useInventoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
		return
	}
	var req struct {
		ItemType string `json:"item_type"`
		ItemID   int    `json:"item_id"`
		Count    int    `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}
	if req.Count <= 0 {
		req.Count = 1
	}
	err = removeInventoryItem(playerID, req.ItemType, req.ItemID, req.Count)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка использования предмета: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Предмет успешно использован/удалён"))
}

// ----------------- ОБРАБОТЧИК ДЛЯ ПЕРЕМЕЩЕНИЯ ------------------
// Теперь мы будем получать instance_id из query-параметров и извлекать размеры карты из БД.

func moveHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
		return
	}

	var req struct {
		NewPosX int `json:"new_pos_x"`
		NewPosY int `json:"new_pos_y"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	// Получаем instance_id из query-параметра
	instanceID := r.URL.Query().Get("instance_id")
	if instanceID == "" {
		http.Error(w, "instance_id обязателен", http.StatusBadRequest)
		return
	}

	// Извлекаем размеры карты для данного матча из БД
	var mapWidth, mapHeight int
	query := `SELECT map_width, map_height FROM matches WHERE instance_id = $1`
	err = db.QueryRow(query, instanceID).Scan(&mapWidth, &mapHeight)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения данных матча: %v", err), http.StatusInternalServerError)
		return
	}

	if req.NewPosX < 0 || req.NewPosX >= mapWidth || req.NewPosY < 0 || req.NewPosY >= mapHeight {
		http.Error(w, "Новые координаты вне границ карты", http.StatusBadRequest)
		return
	}

	player, err := getPlayerByID(playerID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения игрока: %v", err), http.StatusInternalServerError)
		return
	}

	player.PosX = req.NewPosX
	player.PosY = req.NewPosY

	err = updatePlayer(player)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка обновления позиции игрока: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(player)
}

// ----------------- ОБРАБОТЧИКИ ДЛЯ МОНСТРОВ ------------------

func getMonsterByID(id int) (*Monster, error) {
	monster := &Monster{}
	query := `
		SELECT id, name, type, health, max_health, attack, defense, speed, maneuverability, vision, created_at
		FROM monsters
		WHERE id = $1
	`
	row := db.QueryRow(query, id)
	err := row.Scan(&monster.ID, &monster.Name, &monster.Type, &monster.Health, &monster.MaxHealth,
		&monster.Attack, &monster.Defense, &monster.Speed, &monster.Maneuverability, &monster.Vision, &monster.CreatedAt)
	if err != nil {
		return nil, err
	}
	return monster, nil
}

func updateMonster(monster *Monster) error {
	query := `
		UPDATE monsters
		SET health = $1
		WHERE id = $2
	`
	_, err := db.Exec(query, monster.Health, monster.ID)
	return err
}

// ----------------- ОБРАБОТЧИКИ ДЛЯ АТАКИ ------------------

type attackRequest struct {
	AttackerType string `json:"attacker_type"` // "player" или "monster"
	AttackerID   int    `json:"attacker_id"`
	TargetType   string `json:"target_type"`   // "player" или "monster"
	TargetID     int    `json:"target_id"`
}

func universalAttackHandler(w http.ResponseWriter, r *http.Request) {
	var req attackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	var attackerAttack, targetDefense int
	switch req.AttackerType {
	case "player":
		player, err := getPlayerByID(req.AttackerID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка получения атакующего игрока: %v", err), http.StatusInternalServerError)
			return
		}
		attackerAttack = player.Attack
	case "monster":
		monster, err := getMonsterByID(req.AttackerID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка получения атакующего монстра: %v", err), http.StatusInternalServerError)
			return
		}
		attackerAttack = monster.Attack
	default:
		http.Error(w, "Неверный тип атакующего", http.StatusBadRequest)
		return
	}

	var targetHealth int
	switch req.TargetType {
	case "player":
		player, err := getPlayerByID(req.TargetID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка получения цели-игрока: %v", err), http.StatusInternalServerError)
			return
		}
		targetDefense = player.Defense
		targetHealth = player.Health
	case "monster":
		monster, err := getMonsterByID(req.TargetID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка получения цели-монстра: %v", err), http.StatusInternalServerError)
			return
		}
		targetDefense = monster.Defense
		targetHealth = monster.Health
	default:
		http.Error(w, "Неверный тип цели", http.StatusBadRequest)
		return
	}

	damage := attackerAttack - targetDefense
	if damage < 1 {
		damage = 1
	}
	newHealth := targetHealth - damage
	if newHealth < 0 {
		newHealth = 0
	}

	switch req.TargetType {
	case "player":
		target, err := getPlayerByID(req.TargetID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка получения цели игрока: %v", err), http.StatusInternalServerError)
			return
		}
		target.Health = newHealth
		if err := updatePlayer(target); err != nil {
			http.Error(w, fmt.Sprintf("Ошибка обновления цели игрока: %v", err), http.StatusInternalServerError)
			return
		}
	case "monster":
		target, err := getMonsterByID(req.TargetID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка получения цели монстра: %v", err), http.StatusInternalServerError)
			return
		}
		target.Health = newHealth
		if err := updateMonster(target); err != nil {
			http.Error(w, fmt.Sprintf("Ошибка обновления цели монстра: %v", err), http.StatusInternalServerError)
			return
		}
	}

	log.Printf("Атакующий %s %d атаковал цель %s %d, нанеся %d урона", req.AttackerType, req.AttackerID, req.TargetType, req.TargetID, damage)
	response := map[string]interface{}{
		"attacker_type": req.AttackerType,
		"attacker_id":   req.AttackerID,
		"target_type":   req.TargetType,
		"target_id":     req.TargetID,
		"damage":        damage,
		"new_target_hp": newHealth,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ----------------- MAIN ------------------

func enableCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Разрешаем все домены (для разработки)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		// Разрешаем методы GET, POST, OPTIONS
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		// Разрешаем заголовки Content-Type и Authorization
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		// Если это OPTIONS-запрос, сразу возвращаем 200 OK
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	initDB()
	defer db.Close()
	createPlayersTable()
	createMatchesTable()
	createMatchPlayersTable()

	router := mux.NewRouter()
	// Эндпойнт для создания персонажа (вызывается Auth-сервисом)
	router.HandleFunc("/game/player", createPlayerHandler).Methods("POST")
	// Эндпойнты для работы с игроками.
	router.HandleFunc("/game/player/{id}", getPlayerHandler).Methods("GET")
	router.HandleFunc("/game/player/{id}/gain_experience", gainExperienceHandler).Methods("POST")
	router.HandleFunc("/game/player/{id}/move", moveHandler).Methods("POST")
	// Универсальный эндпойнт для атаки.
	router.HandleFunc("/game/attack", universalAttackHandler).Methods("POST")
	// Эндпойнты для работы с инвентарём.
	router.HandleFunc("/game/player/{id}/inventory/add", addInventoryHandler).Methods("POST")
	router.HandleFunc("/game/player/{id}/inventory/use", useInventoryHandler).Methods("POST")
	// Новый эндпойнт для создания матча, который вызывается Matchmaking‑сервисом.
	router.HandleFunc("/game/createMatch", createMatchHandler).Methods("POST")
	// Эндпойнт для получения данных матча по instance_id.
	router.HandleFunc("/game/match", getMatchHandler).Methods("GET")

	handler := enableCors(router)
	port := "8001" // Game-сервис работает на порту 8001
	log.Printf("Game-сервис запущен на порту %s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
