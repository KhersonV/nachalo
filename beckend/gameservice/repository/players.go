// ====================================
// /gameservice/repository/players.go
// ====================================

package repository

import (
	"encoding/json"
	"fmt"
	"log"

	"gameservice/models"
)

// GetPlayerByUserID получает игрока из таблицы players по user_id.
// position хранится в match_players, поэтому здесь не читается.
func GetPlayerByUserID(userID int) (*models.PlayerResponse, error) {
	player := &models.PlayerResponse{}
	query := `
		SELECT 
			user_id, 
			name, 
			image, 
			character_type,
			energy, 
			max_energy, 
			health, 
			max_health, 
			level, 
			experience, 
			max_experience, 
			attack, 
			defense, 
			mobility, 
			agility, 
			sight_range, 
			is_ranged,
			attack_range,
			balance, 
			inventory
		FROM players
		WHERE user_id = $1
	`
	row := DB.QueryRow(query, userID)
	err := row.Scan(
		&player.UserID,
		&player.Name,
		&player.Image,
		&player.CharacterType,
		&player.Energy,
		&player.MaxEnergy,
		&player.Health,
		&player.MaxHealth,
		&player.Level,
		&player.Experience,
		&player.MaxExperience,
		&player.Attack,
		&player.Defense,
		&player.Mobility,
		&player.Agility,
		&player.SightRange,
		&player.IsRanged,
		&player.AttackRange,
		&player.Balance,
		&player.Inventory,
	)
	if err != nil {
		log.Printf("GetPlayerByUserID: ошибка получения игрока user_id=%d: %v", userID, err)
		return nil, err
	}

	return player, nil
}

// UpdatePlayer обновляет данные игрока в таблице players,
// включая баланс и инвентарь (JSON-строку).
func UpdatePlayer(player *models.PlayerResponse) error {
    query := `
    UPDATE players
    SET
        energy         = $1,
        health         = $2,
        level          = $3,
        experience     = $4,
        max_experience = $5,
        balance        = $6,
        inventory      = $7
    WHERE user_id = $8
    `
    if _, err := DB.Exec(query,
        player.Energy,
        player.Health,
        player.Level,
        player.Experience,
        player.MaxExperience,
        player.Balance,
        player.Inventory,
        player.UserID,
    ); err != nil {
        return fmt.Errorf("UpdatePlayer: %w", err)
    }
    return nil
}

// DeleteMatchPlayer удаляет игрока из таблицы match_players по идентификатору матча и user_id.
func DeleteMatchPlayer(instanceID string, userID int) error {
	_, err := DB.Exec(`
        DELETE FROM match_players
        WHERE instance_id = $1
          AND user_id = $2
    `, instanceID, userID)
	return err
}

func MarkPlayerDead(instanceID string, userID int) error {
	_, err := DB.Exec(`
        UPDATE match_players
           SET health = 0
         WHERE instance_id = $1
           AND user_id = $2
    `, instanceID, userID)
	return err
}

// пороги опыта для повышения уровня — дублируют логику из handlers/players.go
var levelThresholds = map[int]int{
	1:  500,
	2:  2000,
	3:  8000,
	4:  32000,
	5:  128000,
	6:  512000,
	7:  2048000,
	8:  8192000,
	9:  32768000,
	10: 131072000,
}

// AddPlayerExperience добавляет опыт перманентному игроку, проверяет и повышает уровень при необходимости.
func AddPlayerExperience(userID, exp int) error {
	// 1) Получаем текущего игрока
	player, err := GetPlayerByUserID(userID)
	if err != nil {
		return fmt.Errorf("AddPlayerExperience: fetch player: %w", err)
	}

	// 2) Увеличиваем опыт
	player.Experience += exp

	// 3) Проверяем, не перешёл ли через порог для повышения уровня
	if threshold, ok := levelThresholds[player.Level]; ok && player.Experience >= threshold {
		player.Level++
		player.Experience -= threshold
		player.MaxExperience = levelThresholds[player.Level]
		log.Printf("GetPlayerByUserID: MaxExperience = %d", player.MaxExperience)
	}

	// 4) Сохраняем изменения
	if err := UpdatePlayer(player); err != nil {
		return fmt.Errorf("AddPlayerExperience: update player: %w", err)
	}
	return nil
}

func AddPlayerRewards(userID int, rewardsData []byte) error {
	log.Printf("Raw rewardsData for user %d: %s", userID, string(rewardsData))

	type rewardEntry struct {
		Type   string `json:"type"`
		Amount int    `json:"amount"`
	}

	// 1) Распарсим JSON наград (поддерживаем и map, и массив объектов).
	rewards := make(map[string]int)
	var rewardsMap map[string]int
	if err := json.Unmarshal(rewardsData, &rewardsMap); err == nil {
		for k, v := range rewardsMap {
			rewards[k] += v
		}
	} else {
		var rewardsList []rewardEntry
		if err := json.Unmarshal(rewardsData, &rewardsList); err != nil {
			return fmt.Errorf("AddPlayerRewards: unmarshal rewards: %w", err)
		}
		for _, r := range rewardsList {
			if r.Type == "" || r.Amount == 0 {
				continue
			}
			rewards[r.Type] += r.Amount
		}
	}

	// 2) Получаем игрока
	player, err := GetPlayerByUserID(userID)
	if err != nil {
		return fmt.Errorf("AddPlayerRewards: fetch player: %w", err)
	}

	// 3) Применяем каждую награду
	for key, amount := range rewards {
		switch key {
		case "balance", "coin", "coins":
			player.Balance += amount

		default:
			// Обновляем inventory, храня его как JSON-строку
			var inv map[string]map[string]interface{}
			if err := json.Unmarshal([]byte(player.Inventory), &inv); err != nil {
				inv = make(map[string]map[string]interface{})
			}
			inv[key] = map[string]interface{}{
				"name":       key,
				"item_count": amount,
			}
			b, _ := json.Marshal(inv)
			player.Inventory = string(b)
		}
	}

	// 4) Сохраняем изменени
	if err := UpdatePlayer(player); err != nil {
		return fmt.Errorf("AddPlayerRewards: update player: %w", err)
	}
	return nil
}
