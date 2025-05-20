
// ====================================
// gameservice/repository/matches.go
// ====================================


package repository

import (
	"encoding/json"
	"log"

	"gameservice/game"
	"gameservice/models"
)

// InsertMatch – сохраняет матч в таблице matches.
func InsertMatch(instanceID, mode string, teamsCount, totalPlayers, mapWidth, mapHeight int, mapJSON []byte) (string, error) {
	query := `
        INSERT INTO matches (instance_id, mode, teams_count, total_players, map_width, map_height, map, turn_number)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING instance_id;
    `
	var returnedID string
	// turn_number задаем равным 1 по умолчанию.
	err := DB.QueryRow(query, instanceID, mode, teamsCount, totalPlayers, mapWidth, mapHeight, mapJSON, 1).Scan(&returnedID)
	if err != nil {
		return "", err
	}
	return returnedID, nil
}

// GetMatchByID – получает матч по instance_id из таблицы matches.
func GetMatchByID(instanceID string) (*models.MatchInfo, error) {
	query := `
        SELECT instance_id, mode, teams_count, total_players, map_width, map_height, map, active_user_id, turn_number
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
			match_instance_id, user_id, name, image, position, inventory,
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

// GetMatchPlayerByID – получает данные игрока из match_players по match_instance_id и user_id.
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
		WHERE match_instance_id = $1 AND user_id = $2
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
func UpdateMatchPlayer(player *models.PlayerResponse) error {
	// Формируем JSON для поля position.
	positionJSON, err := json.Marshal(player.Position)
	if err != nil {
		return err
	}

	query := `
		UPDATE match_players
		SET position = $1, energy = $2, health = $3, level = $4, experience = $5
		WHERE user_id = $6
	`
	_, err = DB.Exec(query,
		positionJSON,
		player.Energy,
		player.Health,
		player.Level,
		player.Experience,
		player.UserID,
	)
	return err
}

// GetPlayersInMatch – получает список игроков для матча (по match_instance_id) из таблицы match_players.
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
		WHERE match_instance_id = $1
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
