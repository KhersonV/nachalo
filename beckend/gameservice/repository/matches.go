// ====================================
// gameservice/repository/matches.go
// ====================================

package repository

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"gameservice/game"
	"gameservice/models"
)

// InsertMatch – сохраняет матч в таблице matches.
// InsertMatch – сохраняет или обновляет матч в таблице matches.
func InsertMatch(
	instanceID string,
	mode string,
	teamsCount, totalPlayers int,
	activeUserID int,
	turnOrder []int,
	turnNumber int,
	startPositions [][2]int,
	portalPosition [2]int,
	mapHeight, mapWidth int,
	mapJSON []byte,
) error {
	// Сериализуем JSON-поля
	turnOrderJSON, _ := json.Marshal(turnOrder)
	startPosJSON, _ := json.Marshal(startPositions)
	portalPosJSON, _ := json.Marshal(portalPosition)

	query := `
    INSERT INTO matches (
        instance_id,
        mode,
        teams_count,
        total_players,
        active_user_id,
        turn_order,
        turn_number,
        start_positions,
        portal_position,
        map_height,
        map_width,
        map
    ) VALUES (
        $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12
    )
    ON CONFLICT (instance_id) DO UPDATE
      SET
        mode            = EXCLUDED.mode,
        teams_count     = EXCLUDED.teams_count,
        total_players   = EXCLUDED.total_players,
        active_user_id  = EXCLUDED.active_user_id,
        turn_order      = EXCLUDED.turn_order,
        turn_number     = EXCLUDED.turn_number,
        start_positions = EXCLUDED.start_positions,
        portal_position = EXCLUDED.portal_position,
        map_height      = EXCLUDED.map_height,
        map_width       = EXCLUDED.map_width,
        map             = EXCLUDED.map;
    `
	_, err := DB.Exec(
		query,
		instanceID,
		mode,
		teamsCount,
		totalPlayers,
		activeUserID,
		turnOrderJSON,
		turnNumber,
		startPosJSON,
		portalPosJSON,
		mapHeight,
		mapWidth,
		mapJSON,
	)
	return err
}

// GetMatchByID – получает матч по instance_id из таблицы matches.
func GetMatchByID(instanceID string) (*models.MatchInfo, error) {
	query := `
        SELECT
            instance_id,
            mode,
            teams_count,
            total_players,
            map_width,
            map_height,
            map,
            active_user_id,
            turn_number,
            start_positions,
            portal_position
        FROM matches
        WHERE instance_id = $1
    `
	match := &models.MatchInfo{}
	err := DB.QueryRow(query, instanceID).Scan(
		&match.InstanceID,
		&match.Mode,
		&match.TeamsCount,
		&match.TotalPlayers,
		&match.MapWidth,
		&match.MapHeight,
		&match.Map,
		&match.ActiveUserID,
		&match.TurnNumber,
		&match.StartPositions, // <-- raw JSONB
		&match.PortalPosition, // <-- raw JSONB
	)
	if err != nil {
		return nil, err
	}
	return match, nil
}

// CreateMatchPlayerCopy – создает копию игрока в таблице match_players для конкретного матча.
func CreateMatchPlayerCopy(matchID string, p *models.PlayerResponse, startX, startY, groupID int) error {
	// Формируем JSON для поля position.
	position, err := json.Marshal(struct {
		X int `json:"x"`
		Y int `json:"y"`
	}{X: startX, Y: startY})
	if err != nil {
		return err
	}

	query := `
		INSERT INTO match_players (
			instance_id, user_id, name, image, position, inventory,
			level, energy, max_energy, health, max_health, experience, max_experience,
			attack, defense, speed, maneuverability, vision, vision_range, balance, group_id
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19, $20, $21
		);
	`
	res, err := DB.Exec(query,
		matchID,
		p.UserID,
		p.Name,
		p.Image,
		position,
		p.Inventory,
		p.Level,
		p.Energy,
		p.MaxEnergy,
		p.Health,
		p.MaxHealth,
		p.Experience,
		p.MaxExperience,
		p.Attack,
		p.Defense,
		p.Speed,
		p.Maneuverability,
		p.Vision,
		p.VisionRange,
		p.Balance,
		groupID,
	)
	if err != nil {
		log.Printf("CreateMatchPlayerCopy: ошибка вставки user_id=%d, matchID=%s: %v", p.UserID, matchID, err)
		return err
	}
	affected, _ := res.RowsAffected()
	log.Printf("CreateMatchPlayerCopy: успешно вставлено %d строк для user_id=%d, matchID=%s", affected, p.UserID, matchID)
	return nil
}

// GetMatchPlayerByID – получает данные игрока из match_players по instance_id и user_id.
func GetMatchPlayerByID(matchID string, userID int) (*models.PlayerResponse, error) {
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
			balance
		FROM match_players
		WHERE instance_id = $1 AND user_id = $2
		LIMIT 1;
	`
	var pr models.PlayerResponse
	var positionJSON []byte

	err := DB.QueryRow(query, matchID, userID).Scan(
		&pr.UserID,
		&pr.Name,
		&pr.Image,
		&positionJSON,
		&pr.Inventory,
		&pr.Level,
		&pr.Energy,
		&pr.MaxEnergy,
		&pr.Health,
		&pr.MaxHealth,
		&pr.Experience,
		&pr.MaxExperience,
		&pr.Attack,
		&pr.Defense,
		&pr.Speed,
		&pr.Maneuverability,
		&pr.Vision,
		&pr.VisionRange,
		&pr.Balance,
	)
	if err != nil {
		log.Printf("GetMatchPlayerByID: ошибка получения игрока matchID=%s, user_id=%d: %v", matchID, userID, err)
		return nil, err
	}

	// Распарсить JSON из поля position в структуру Position модели
	if err := json.Unmarshal(positionJSON, &pr.Position); err != nil {
		return nil, err
	}

	return &pr, nil
}

// UpdateMatchPlayer – обновляет данные игрока в таблице match_players для конкретного матча.
func UpdateMatchPlayer(instanceID string, player *models.PlayerResponse) error {
	// Формируем JSON для поля position.
	positionJSON, err := json.Marshal(player.Position)
	if err != nil {
		return err
	}

	query := `
		UPDATE match_players
		SET position = $1, energy = $2, health = $3, level = $4, experience = $5
		WHERE instance_id = $6
		AND user_id     = $7
	`
	_, err = DB.Exec(query,
		positionJSON,
		player.Energy,
		player.Health,
		player.Level,
		player.Experience,
		instanceID,
		player.UserID,
	)
	return err
}

// GetPlayersInMatch – получает список игроков для матча (по instance_id) из таблицы match_players.
func GetPlayersInMatch(matchID string) ([]models.PlayerResponse, error) {
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
			balance
		FROM match_players
		WHERE instance_id = $1
		AND health > 0
	`
	rows, err := DB.Query(query, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []models.PlayerResponse
	for rows.Next() {
		var pr models.PlayerResponse
		var positionJSON []byte
		err := rows.Scan(
			&pr.UserID,
			&pr.Name,
			&pr.Image,
			&positionJSON,
			&pr.Inventory,
			&pr.Level,
			&pr.Energy,
			&pr.MaxEnergy,
			&pr.Health,
			&pr.MaxHealth,
			&pr.Experience,
			&pr.MaxExperience,
			&pr.Attack,
			&pr.Defense,
			&pr.Speed,
			&pr.Maneuverability,
			&pr.Vision,
			&pr.VisionRange,
			&pr.Balance,
		)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(positionJSON, &pr.Position); err != nil {
			return nil, err
		}
		players = append(players, pr)
	}
	return players, nil
}

// UpdateMatchTurn – обновляет текущий ход матча в таблице matches.
func UpdateMatchTurn(instanceID string, activeUserID int, turnNumber int) error {
	query := `
		UPDATE matches
		SET active_user_id = $1, turn_number = $2
		WHERE instance_id = $3
	`
	_, err := DB.Exec(query, activeUserID, turnNumber, instanceID)
	return err
}

// GetItemEffect – получает эффект предмета из таблицы resources.
func GetItemEffect(itemID int) (map[string]int, error) {
	query := `SELECT effect FROM resources WHERE id = $1;`
	var effectJSON []byte
	err := DB.QueryRow(query, itemID).Scan(&effectJSON)
	if err != nil {
		return nil, err
	}

	var effect map[string]int
	if err := json.Unmarshal(effectJSON, &effect); err != nil {
		return nil, err
	}
	return effect, nil
}

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// ClearCellPlayerFlag снимает флаг IsPlayer в поле map таблицы matches
// для клетки oldPos, не трогая другие.
// ClearCellPlayerFlag снимает флаг IsPlayer в поле map для клетки oldPos,
// не трогая больше никаких позиций.
func ClearCellPlayerFlag(instanceID string, oldPos Position) error {
	// 1) Считываем текущее поле map из matches
	var mapJSON []byte
	if err := DB.QueryRow(
		`SELECT map FROM matches WHERE instance_id = $1`,
		instanceID,
	).Scan(&mapJSON); err != nil {
		return fmt.Errorf("ClearCellPlayerFlag: failed to load map JSON: %w", err)
	}

	// 2) Десериализуем в срез клеток
	var cells []game.FullCell
	if err := json.Unmarshal(mapJSON, &cells); err != nil {
		return fmt.Errorf("ClearCellPlayerFlag: failed to unmarshal: %w", err)
	}

	// 3) Находим oldPos и снимаем IsPlayer
	for i, c := range cells {
		if c.X == oldPos.X && c.Y == oldPos.Y {
			cells[i].IsPlayer = false
			break
		}
	}

	// 4) Сериализуем обратно
	newMap, err := json.Marshal(cells)
	if err != nil {
		return fmt.Errorf("ClearCellPlayerFlag: failed to marshal: %w", err)
	}

	// 5) Обновляем в БД
	if _, err := DB.Exec(
		`UPDATE matches SET map = $1 WHERE instance_id = $2`,
		newMap, instanceID,
	); err != nil {
		return fmt.Errorf("ClearCellPlayerFlag: failed to update DB: %w", err)
	}

	return nil
}

// UpdateCellPlayerFlags обновляет поле isPlayer в карте матча, установив false для клетки с oldPos и true для клетки с newPos.
// UpdateCellPlayerFlags устанавливает false на старой и true на новой позиции игрока.
func UpdateCellPlayerFlags(instanceID string, oldPos, newPos Position) error {
	// Извлекаем текущую карту матча.
	var mapJSON []byte
	query := `SELECT map FROM matches WHERE instance_id = $1;`
	if err := DB.QueryRow(query, instanceID).Scan(&mapJSON); err != nil {
		return err
	}

	// Десериализуем в срез FullCell
	var cells []game.FullCell
	if err := json.Unmarshal(mapJSON, &cells); err != nil {
		return err
	}

	log.Printf("[UpdateCellPlayerFlags] old=%+v new=%+v", oldPos, newPos)

	// Обновляем флаги
	for i, c := range cells {
		if c.X == oldPos.X && c.Y == oldPos.Y {
			cells[i].IsPlayer = false
		}
		if c.X == newPos.X && c.Y == newPos.Y {
			cells[i].IsPlayer = true
		}
	}

	// Сериализуем обратно и сохраняем
	newMap, err := json.Marshal(cells)
	if err != nil {
		return err
	}
	if _, err := DB.Exec(`UPDATE matches SET map = $1 WHERE instance_id = $2`, string(newMap), instanceID); err != nil {
		log.Printf("UpdateCellPlayerFlags error: %v", err)
		return err
	}
	return nil
}

func LoadMapCells(instanceID string) ([]game.FullCell, error) {
	var raw []byte
	err := DB.QueryRow(
		`SELECT map FROM matches WHERE instance_id=$1`,
		instanceID,
	).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var cells []game.FullCell
	if err := json.Unmarshal(raw, &cells); err != nil {
		return nil, err
	}
	return cells, nil
}

func SaveMapCells(instanceID string, cells []game.FullCell) error {
	raw, err := json.Marshal(cells)
	if err != nil {
		return err
	}
	_, err = DB.Exec(
		`UPDATE matches SET map=$1 WHERE instance_id=$2`,
		raw, instanceID,
	)
	return err
}

// repository/match.go

// DeleteMatch удаляет матч и всё по нему в БД
func DeleteMatch(instanceID string) error {
	// 1) Удаляем всех игроков
	if _, err := DB.Exec(`
        DELETE FROM match_players
        WHERE instance_id = $1
    `, instanceID); err != nil {
		return fmt.Errorf("DeleteMatch: failed to delete match_players: %w", err)
	}

	// 2) Удаляем связанные детали (если есть table match_player_stats)
	if _, err := DB.Exec(`
        DELETE FROM match_player_stats
        WHERE instance_id = $1
    `, instanceID); err != nil {
		return fmt.Errorf("DeleteMatch: failed to delete match_player_stats: %w", err)
	}

	// 3) Наконец, удаляем сам матч
	if _, err := DB.Exec(`
        DELETE FROM matches
        WHERE instance_id = $1
    `, instanceID); err != nil {
		return fmt.Errorf("DeleteMatch: failed to delete match: %w", err)
	}

	return nil
}

func SaveMatchStats(stats *models.MatchInfo) error {
	query := `
        INSERT INTO match_stats (instance_id, winner_id, winner_group_id, created_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (instance_id) DO UPDATE
          SET winner_id       = EXCLUDED.winner_id,
              winner_group_id = EXCLUDED.winner_group_id,
              created_at      = EXCLUDED.created_at;
    `
	// используем NOW(), но чтобы тестировать, передаём время из Go
	now := time.Now().UTC()
	if _, err := DB.Exec(query,
		stats.InstanceID,
		stats.WinnerID,
		stats.WinnerGroupID,
		now,
	); err != nil {
		return fmt.Errorf("SaveMatchStats: exec insert: %w", err)
	}
	return nil
}

// SaveMatchPlayerStats сохраняет агрегированные результаты по каждому игроку в таблице match_player_stats.
func SaveMatchPlayerStats(instanceID string, results []game.PlayerResult) error {
	log.Printf("[SaveMatchPlayerStats] start for match %s, %d players", instanceID, len(results))

	tx, err := DB.Begin()
	if err != nil {
		log.Printf("[SaveMatchPlayerStats] begin tx error: %v", err)
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
      INSERT INTO match_player_stats (
        instance_id,
        user_id,
        exp_gained,
        rewards,
        player_kills,
        monster_kills,
        damage_total,
        damage_to_players,
        damage_to_monsters
      ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
      ON CONFLICT (instance_id, user_id) DO UPDATE SET
        exp_gained         = EXCLUDED.exp_gained,
        rewards            = EXCLUDED.rewards,
        player_kills       = EXCLUDED.player_kills,
        monster_kills      = EXCLUDED.monster_kills,
        damage_total       = EXCLUDED.damage_total,
        damage_to_players  = EXCLUDED.damage_to_players,
        damage_to_monsters = EXCLUDED.damage_to_monsters;
    `)
	if err != nil {
		log.Printf("[SaveMatchPlayerStats] prepare stmt error: %v", err)
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for i, pr := range results {
		log.Printf("[SaveMatchPlayerStats] exec for user %d (%d/%d): exp=%d, pk=%d, mk=%d, dt=%d, dp=%d, dm=%d",
			pr.UserID, i+1, len(results),
			pr.ExpGained, pr.PlayerKills, pr.MonsterKills,
			pr.DamageTotal, pr.DamageToPlayers, pr.DamageToMonsters,
		)
		if _, err := stmt.Exec(
			instanceID,
			pr.UserID,
			pr.ExpGained,
			pr.RewardsData,
			pr.PlayerKills,
			pr.MonsterKills,
			pr.DamageTotal,
			pr.DamageToPlayers,
			pr.DamageToMonsters,
		); err != nil {
			log.Printf("[SaveMatchPlayerStats] exec error for user %d: %v", pr.UserID, err)
			return fmt.Errorf("exec for user %d: %w", pr.UserID, err)
		}
		log.Printf("[SaveMatchPlayerStats] exec succeeded for user %d", pr.UserID)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[SaveMatchPlayerStats] commit error: %v", err)
		return fmt.Errorf("commit: %w", err)
	}
	log.Printf("[SaveMatchPlayerStats] commit succeeded for match %s", instanceID)
	return nil
}

func CleanupMatchPlayers(instanceID string) error {
	_, err := DB.Exec(`
        DELETE FROM match_players
        WHERE instance_id = $1
    `, instanceID)
	return err
}

// LoadMatchStats загружает из БД информацию о режиме и победителе матча
func LoadMatchStats(instanceID string) (*models.MatchInfo, error) {
	var mi models.MatchInfo

	// Джоиним match_stats и matches, чтобы достать mode из matches
	query := `
    SELECT 
      ms.instance_id,
      m.mode,
      ms.winner_id,
      ms.winner_group_id
    FROM match_stats ms
    JOIN matches m ON m.instance_id = ms.instance_id
    WHERE ms.instance_id = $1
    `
	row := DB.QueryRow(query, instanceID)
	if err := row.Scan(
		&mi.InstanceID,
		&mi.Mode,
		&mi.WinnerID,
		&mi.WinnerGroupID,
	); err != nil {
		return nil, fmt.Errorf("LoadMatchStats: scan error: %w", err)
	}

	return &mi, nil
}

// LoadMatchPlayerStats возвращает все записи из match_player_stats для данного матча
func LoadMatchPlayerStats(instanceID string) ([]models.PlayerMatchStat, error) {
	query := `
    SELECT 
      instance_id,
      user_id,
      exp_gained,
      rewards,
      player_kills,
      monster_kills,
      damage_total,
      damage_to_players,
      damage_to_monsters
    FROM match_player_stats
    WHERE instance_id = $1
    ORDER BY user_id
  `
	rows, err := DB.Query(query, instanceID)
	if err != nil {
		return nil, fmt.Errorf("LoadMatchPlayerStats: query error: %w", err)
	}
	defer rows.Close()

	var stats []models.PlayerMatchStat
	for rows.Next() {
		var s models.PlayerMatchStat
		if err := rows.Scan(
			&s.InstanceID,
			&s.UserID,
			&s.ExpGained,
			&s.Rewards,
			&s.PlayerKills,
			&s.MonsterKills,
			&s.DamageTotal,
			&s.DamageToPlayers,
			&s.DamageToMonsters,
		); err != nil {
			return nil, fmt.Errorf("LoadMatchPlayerStats: scan error: %w", err)
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("LoadMatchPlayerStats: rows error: %w", err)
	}
	return stats, nil
}
