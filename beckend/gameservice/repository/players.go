

// ====================================
// /gameservice/repository/players.go
// ====================================

package repository

import (
	"encoding/json"
	"log"

	"gameservice/models"
)

// GetPlayerByUserID получает игрока из таблицы players по user_id 
// и формирует структуру PlayerResponse, разбирая поле position (JSONB).
func GetPlayerByUserID(userID int) (*models.PlayerResponse, error) {
	player := &models.PlayerResponse{}
	query := `
		SELECT 
			user_id, 
			name, 
			image, 
			position, 
			energy, 
			max_energy, 
			health, 
			max_health, 
			level, 
			experience, 
			max_experience, 
			attack, 
			defense, 
			speed, 
			maneuverability, 
			vision, 
			vision_range, 
			balance, 
			inventory
		FROM players
		WHERE user_id = $1
	`
	var positionJSON []byte
	row := DB.QueryRow(query, userID)
	err := row.Scan(
		&player.UserID,
		&player.Name,
		&player.Image,
		&positionJSON,
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
	)
	if err != nil {
		log.Printf("GetPlayerByUserID: ошибка получения игрока user_id=%d: %v", userID, err)
		return nil, err
	}

	// Разбираем JSON из поля position и записываем его в структуру Position.
	if err := json.Unmarshal(positionJSON, &player.Position); err != nil {
		log.Printf("GetPlayerByUserID: ошибка разбора поля position: %v", err)
		return nil, err
	}

	return player, nil
}

// UpdatePlayer обновляет данные игрока в таблице players.
// Обновляются поле position (формируется из player.Position), энергия, здоровье, уровень и опыт.
func UpdatePlayer(player *models.PlayerResponse) error {
	// Формируем JSON из структуры Position.
	positionJSON, err := json.Marshal(player.Position)
	if err != nil {
		return err
	}

	query := `
		UPDATE players
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

// DeleteMatchPlayer удаляет игрока из таблицы match_players по идентификатору матча и user_id.
func DeleteMatchPlayer(instanceID string, userID int) error {
    _, err := DB.Exec(`
        DELETE FROM match_players
        WHERE match_instance_id = $1
          AND user_id = $2
    `, instanceID, userID)
    return err
}

