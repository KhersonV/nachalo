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

func LoadMatchMonsters(instanceID string) ([]MatchMonster, error) {
	rows, err := DB.Query(`
		SELECT
			instance_id,
			monster_instance_id,
			monster_ref_id,
			pos_x,
			pos_y,
			health,
			max_health,
			attack,
			defense,
			speed,
			maneuverability,
			vision,
			image
		FROM match_monsters
		WHERE instance_id = $1
		ORDER BY monster_instance_id
	`, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monsters []MatchMonster

	for rows.Next() {
		var m MatchMonster

		err := rows.Scan(
			&m.InstanceID,
			&m.MonsterInstanceID,
			&m.RefID,
			&m.X,
			&m.Y,
			&m.Health,
			&m.MaxHealth,
			&m.Attack,
			&m.Defense,
			&m.Speed,
			&m.Maneuverability,
			&m.Vision,
			&m.Image,
		)
		if err != nil {
			return nil, err
		}

		monsters = append(monsters, m)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return monsters, nil
}

func LoadMatchPlayers(instanceID string) ([]models.PlayerResponse, error) {
	rows, err := DB.Query(`
		SELECT
			mp.user_id,
			mp.name,
			mp.image,
			COALESCE(p.character_type, 'adventurer') AS character_type,
			mp.position,
			mp.energy,
			mp.max_energy,
			mp.health,
			mp.max_health,
			mp.level,
			mp.experience,
			mp.max_experience,
			mp.attack,
			mp.defense,
			mp.mobility,
			mp.agility,
			mp.sight_range,
			mp.is_ranged,
			mp.attack_range,
			mp.balance,
			mp.group_id,
			mp.inventory
		FROM match_players mp
		LEFT JOIN players p ON p.user_id = mp.user_id
		WHERE mp.instance_id = $1
		ORDER BY mp.user_id
	`, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []models.PlayerResponse

	for rows.Next() {
		var p models.PlayerResponse
		var positionRaw []byte
		var inventoryRaw []byte

		err := rows.Scan(
			&p.UserID,
			&p.Name,
			&p.Image,
			&p.CharacterType,
			&positionRaw,
			&p.Energy,
			&p.MaxEnergy,
			&p.Health,
			&p.MaxHealth,
			&p.Level,
			&p.Experience,
			&p.MaxExperience,
			&p.Attack,
			&p.Defense,
			&p.Mobility,
			&p.Agility,
			&p.SightRange,
			&p.IsRanged,
			&p.AttackRange,
			&p.Balance,
			&p.GroupID,
			&inventoryRaw,
		)
		if err != nil {
			return nil, err
		}

		// position JSONB -> struct { X, Y }
		if len(positionRaw) > 0 {
			var pos struct {
				X int `json:"x"`
				Y int `json:"y"`
			}
			if err := json.Unmarshal(positionRaw, &pos); err == nil {
				p.Position.X = pos.X
				p.Position.Y = pos.Y
			}
		}

		// inventory JSONB -> string
		if len(inventoryRaw) > 0 && json.Valid(inventoryRaw) {
			p.Inventory = string(inventoryRaw)
		} else {
			p.Inventory = "{}"
		}

		players = append(players, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return players, nil
}

func UpdateMatchPlayerHealth(instanceID string, userID int, newHP int) error {
	_, err := DB.Exec(`
		UPDATE match_players
		SET health = $1
		WHERE instance_id = $2 AND user_id = $3
	`, newHP, instanceID, userID)
	return err
}

func UpdateMatchPlayerInventory(instanceID string, userID int, inventory string) error {
	_, err := DB.Exec(`
		UPDATE match_players
		SET inventory = $1
		WHERE instance_id = $2 AND user_id = $3
	`, inventory, instanceID, userID)
	return err
}

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
            portal_position,
            COALESCE(quest_artifact_id, 0)
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
		&match.StartPositions,
		&match.PortalPosition,
		&match.QuestArtifactID,
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
			attack, defense, mobility, agility, sight_range, is_ranged, attack_range, balance, group_id
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19, $20, $21, $22
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
		p.Mobility,
		p.Agility,
		p.SightRange,
		p.IsRanged,
		p.AttackRange,
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

// функция получения данных игрока из match_players,
// которая формирует структуру PlayerResponse.
// GetMatchPlayerByID – возвращает данные игрока из match_players по instance_id и user_id.
func GetMatchPlayerByID(matchID string, userID int) (*models.PlayerResponse, error) {
	query := `
		SELECT 
			mp.user_id,
			mp.name, 
			mp.image,
			COALESCE(p.character_type, 'adventurer') AS character_type,
			mp.position, 
			mp.inventory,
			mp.level,
			mp.energy,
			mp.max_energy,
			mp.health,
			mp.max_health,
			mp.experience,
			mp.max_experience,
			mp.attack,
			mp.defense,
			mp.mobility,
			mp.agility,
			mp.sight_range,
			mp.is_ranged,
			mp.attack_range,
			mp.group_id,
			mp.balance
		FROM match_players mp
		LEFT JOIN players p ON p.user_id = mp.user_id
		WHERE mp.instance_id = $1 AND mp.user_id = $2
		LIMIT 1;
	`
	var pr models.PlayerResponse
	var positionJSON []byte

	err := DB.QueryRow(query, matchID, userID).Scan(
		&pr.UserID,
		&pr.Name,
		&pr.Image,
		&pr.CharacterType,
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
		&pr.Mobility,
		&pr.Agility,
		&pr.SightRange,
		&pr.IsRanged,
		&pr.AttackRange,
		&pr.GroupID,
		&pr.Balance,
	)
	if err != nil {
		log.Printf("GetMatchPlayerByID: ошибка получения игрока matchID=%s, user_id=%d: %v", matchID, userID, err)
		return nil, err
	}

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
			mp.user_id,
			mp.name,
			mp.image,
			COALESCE(p.character_type, 'adventurer') AS character_type,
			mp.position,
			mp.inventory,
			mp.level,
			mp.energy,
			mp.max_energy,
			mp.health,
			mp.max_health,
			mp.experience,
			mp.max_experience,
			mp.attack,
			mp.defense,
			mp.mobility,
			mp.agility,
			mp.sight_range,
			mp.is_ranged,
			mp.attack_range,
			mp.group_id,
			mp.balance
		FROM match_players mp
		LEFT JOIN players p ON p.user_id = mp.user_id
		WHERE mp.instance_id = $1
		AND mp.health > 0
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
			&pr.CharacterType,
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
			&pr.Mobility,
			&pr.Agility,
			&pr.SightRange,
			&pr.IsRanged,
			&pr.AttackRange,
			&pr.GroupID,
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

// SetMatchWinner фиксирует победителя матча перед финализацией.
// Для одиночного победителя используем winnerID > 0 и winnerGroupID = 0.
// Для командной победы используем winnerGroupID > 0 и winnerID = 0.
func SetMatchWinner(instanceID string, winnerID int, winnerGroupID int) error {
	query := `
		UPDATE matches
		SET winner_id = $1,
		    winner_group_id = $2
		WHERE instance_id = $3
	`
	_, err := DB.Exec(query, winnerID, winnerGroupID, instanceID)
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

// DeleteMatch удаляет матч и всё по нему в БД
func DeleteMatch(instanceID string) error {
	// 1) Удаляем всех игроков
	if _, err := DB.Exec(`
        DELETE FROM match_players
        WHERE instance_id = $1
    `, instanceID); err != nil {
		return fmt.Errorf("DeleteMatch: failed to delete match_players: %w", err)
	}

	// 2) Наконец, удаляем сам матч.
	// Итоговые таблицы match_stats/match_player_stats сохраняем для истории.
	if _, err := DB.Exec(`
        DELETE FROM matches
        WHERE instance_id = $1
    `, instanceID); err != nil {
		return fmt.Errorf("DeleteMatch: failed to delete match: %w", err)
	}

	return nil
}

func SaveMatchStats(stats *models.MatchInfo) error {
	winnerUserIDsJSON, err := json.Marshal(stats.WinnerUserIDs)
	if err != nil {
		winnerUserIDsJSON = []byte("[]")
	}

	query := `
        INSERT INTO match_stats (instance_id, winner_id, winner_group_id, winner_user_ids, created_at)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (instance_id) DO UPDATE
          SET winner_id       = EXCLUDED.winner_id,
              winner_group_id = EXCLUDED.winner_group_id,
              winner_user_ids = EXCLUDED.winner_user_ids,
              created_at      = EXCLUDED.created_at;
    `
	// используем NOW(), но чтобы тестировать, передаём время из Go
	now := time.Now().UTC()
	if _, err := DB.Exec(query,
		stats.InstanceID,
		stats.WinnerID,
		stats.WinnerGroupID,
		winnerUserIDsJSON,
		now,
	); err != nil {
		return fmt.Errorf("SaveMatchStats: exec insert: %w", err)
	}
	return nil
}

// SaveMatchPlayerStats сохраняет агрегированные результаты по каждому игроку в таблице match_player_stats.
func SaveMatchPlayerStats(instanceID string, results []game.PlayerResult) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
      INSERT INTO match_player_stats (
        instance_id,
        user_id,
				is_winner,
        exp_gained,
        rewards,
        player_kills,
        monster_kills,
        damage_total,
        damage_to_players,
        damage_to_monsters
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
      ON CONFLICT (instance_id, user_id) DO UPDATE SET
				is_winner         = EXCLUDED.is_winner,
        exp_gained         = EXCLUDED.exp_gained,
        rewards            = EXCLUDED.rewards,
        player_kills       = EXCLUDED.player_kills,
        monster_kills      = EXCLUDED.monster_kills,
        damage_total       = EXCLUDED.damage_total,
        damage_to_players  = EXCLUDED.damage_to_players,
        damage_to_monsters = EXCLUDED.damage_to_monsters;
    `)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, pr := range results {
		if _, err := stmt.Exec(
			instanceID,
			pr.UserID,
			pr.IsWinner,
			pr.ExpGained,
			pr.RewardsData,
			pr.PlayerKills,
			pr.MonsterKills,
			pr.DamageTotal,
			pr.DamageToPlayers,
			pr.DamageToMonsters,
		); err != nil {
			return fmt.Errorf("exec for user %d: %w", pr.UserID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

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

	// Читаем только match_stats: запись в matches может быть уже удалена после финализации.
	query := `
    SELECT 
      ms.instance_id,
      ms.winner_id,
      ms.winner_group_id,
      COALESCE(ms.winner_user_ids, '[]'::jsonb)
    FROM match_stats ms
    WHERE ms.instance_id = $1
    `
	row := DB.QueryRow(query, instanceID)
	var winnerUserIDsRaw []byte
	if err := row.Scan(
		&mi.InstanceID,
		&mi.WinnerID,
		&mi.WinnerGroupID,
		&winnerUserIDsRaw,
	); err != nil {
		return nil, fmt.Errorf("LoadMatchStats: scan error: %w", err)
	}

	if len(winnerUserIDsRaw) > 0 {
		if err := json.Unmarshal(winnerUserIDsRaw, &mi.WinnerUserIDs); err != nil {
			mi.WinnerUserIDs = []int{}
		}
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
