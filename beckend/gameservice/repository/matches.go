
// ====================================
// /gameservice/repository/matches.go
// ====================================


package repository

import (
    "log"
    "time"

    "gameservice/models"
)

// Сохранение матча в БД
func InsertMatch(instanceID, mode string, teamsCount, totalPlayers, mapWidth, mapHeight int, mapJSON []byte, createdAt time.Time) (string, error) {
    query := `
        INSERT INTO matches (instance_id, mode, teams_count, total_players, map_width, map_height, map, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING instance_id;
    `
    var returnedID string
    err := DB.QueryRow(query, instanceID, mode, teamsCount, totalPlayers, mapWidth, mapHeight, mapJSON, createdAt).Scan(&returnedID)
    if err != nil {
        return "", err
    }
    return returnedID, nil
}

// Получить матч по instance_id
func GetMatchByID(instanceID string) (*models.MatchInfo, error) {
    query := `
        SELECT instance_id, mode, teams_count, total_players, map_width, map_height, map, created_at
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
        &match.CreatedAt,
    )
    if err != nil {
        return nil, err
    }
    return match, nil
}

// Создать копию игрока в match_players
func CreateMatchPlayerCopy(matchID string, p *models.Player, startX, startY, groupID int) error {
    query := `
        INSERT INTO match_players (
            match_instance_id, player_id, name, image, color_class, pos_x, pos_y, 
            energy, max_energy, health, max_health, level, experience, max_experience,
            attack, defense, speed, maneuverability, vision, vision_range, balance, inventory, 
            group_id, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, 
            $8, $9, $10, $11, $12, $13, $14,
            $15, $16, $17, $18, $19, $20, $21, $22, 
            $23, $24
        );
    `
    res, err := DB.Exec(query,
        matchID, p.UserID, p.Name, p.Image, p.ColorClass, startX, startY,
        p.Energy, p.MaxEnergy, p.Health, p.MaxHealth, p.Level, p.Experience, p.MaxExperience,
        p.Attack, p.Defense, p.Speed, p.Maneuverability, p.Vision, p.VisionRange, p.Balance,
        p.Inventory, groupID, p.UpdatedAt,
    )
    if err != nil {
        log.Printf("CreateMatchPlayerCopy: ошибка вставки player_user_id=%d, matchID=%s: %v", p.UserID, matchID, err)
        return err
    }
    affected, _ := res.RowsAffected()
    log.Printf("CreateMatchPlayerCopy: успешно вставлено %d строк для player_user_id=%d, matchID=%s", affected, p.UserID, matchID)
    return nil
}


// Получить игрока из match_players по matchID + playerID
func GetMatchPlayerByID(matchID string, playerID int) (*models.Player, error) {
    player := &models.Player{}
    query := `
        SELECT id, player_id, name, image, color_class, pos_x, pos_y, energy, max_energy, 
               health, max_health, level, experience, max_experience, attack, defense, 
               speed, maneuverability, vision, vision_range, balance, inventory, updated_at
        FROM match_players
        WHERE match_instance_id = $1 AND player_id = $2
    `
    err := DB.QueryRow(query, matchID, playerID).Scan(
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
        log.Printf("GetMatchPlayerByID: ошибка получения игрока matchID=%s, playerID=%d: %v", matchID, playerID, err)
        return nil, err
    }
    return player, nil
}

// Обновить игрока в match_players
func UpdateMatchPlayer(matchID string, player *models.Player) error {
    query := `
        UPDATE match_players
        SET pos_x = $1, pos_y = $2, energy = $3, health = $4, level = $5, experience = $6,
            updated_at = $7
        WHERE match_instance_id = $8 AND player_id = $9
    `
    _, err := DB.Exec(query,
        player.PosX,
        player.PosY,
        player.Energy,
        player.Health,
        player.Level,
        player.Experience,
        player.UpdatedAt,
        matchID,
        player.UserID,
    )
    return err
}

// Получить список игроков (Player) из match_players по matchID
func GetPlayersInMatch(matchID string) ([]models.Player, error) {
    query := `
        SELECT 
            player_id as id, 
            name, 
            image, 
            color_class, 
            pos_x, 
            pos_y, 
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
            inventory, 
            updated_at
        FROM match_players
        WHERE match_instance_id = $1
    `
    rows, err := DB.Query(query, matchID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var players []models.Player
    for rows.Next() {
        var p models.Player
        if err := rows.Scan(
            &p.ID,
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
            return nil, err
        }
        players = append(players, p)
    }
    return players, nil
}

// Прочие вспомогательные функции...
