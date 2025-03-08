package repository

import (
    "log"
    "time"

    "gameservice/models"
)


// Получение игрока по user_id (как в вашем коде)
func GetPlayerByUserID(userID int) (*models.Player, error) {
    player := &models.Player{}
    query := `
        SELECT id, user_id, name, image, color_class, pos_x, pos_y, energy, max_energy, 
               health, max_health, level, experience, max_experience, attack, defense, 
               speed, maneuverability, vision, vision_range, balance, inventory, updated_at
        FROM players
        WHERE user_id = $1
    `
    row := DB.QueryRow(query, userID)
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
        log.Printf("GetPlayerByUserID: ошибка получения игрока user_id=%d: %v", userID, err)
        return nil, err
    }
    return player, nil
}

// Сохранение (обновление) игрока
func UpdatePlayer(player *models.Player) error {
    query := `
        UPDATE players
        SET pos_x = $1, pos_y = $2, energy = $3, health = $4, level = $5, experience = $6,
            updated_at = $7
        WHERE id = $8
    `
    _, err := DB.Exec(query,
        player.PosX,
        player.PosY,
        player.Energy,
        player.Health,
        player.Level,
        player.Experience,
        time.Now(),
        player.ID,
    )
    return err
}
