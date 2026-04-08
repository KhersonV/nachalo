package repository

import (
    "database/sql"
    "fmt"
)

type PlayerProgressSummary struct {
    MatchesPlayed   int `json:"matchesPlayed"`
    Wins            int `json:"wins"`
    WinRate         int `json:"winRate"`
    TotalExpGained  int `json:"totalExpGained"`
    PlayerKills     int `json:"playerKills"`
    MonsterKills    int `json:"monsterKills"`
    DamageTotal     int `json:"damageTotal"`
}

func GetPlayerProgressSummary(userID int) (*PlayerProgressSummary, error) {
    summary := &PlayerProgressSummary{}

    if err := DB.QueryRow(`
        SELECT
            COUNT(*) AS matches_played,
            COALESCE(SUM(exp_gained), 0) AS total_exp,
            COALESCE(SUM(player_kills), 0) AS player_kills,
            COALESCE(SUM(monster_kills), 0) AS monster_kills,
            COALESCE(SUM(damage_total), 0) AS damage_total
        FROM match_player_stats
        WHERE user_id = $1
    `, userID).Scan(
        &summary.MatchesPlayed,
        &summary.TotalExpGained,
        &summary.PlayerKills,
        &summary.MonsterKills,
        &summary.DamageTotal,
    ); err != nil {
        return nil, fmt.Errorf("GetPlayerProgressSummary: aggregate stats: %w", err)
    }

    if err := DB.QueryRow(`
        SELECT COUNT(*)
        FROM match_player_stats
        WHERE user_id = $1
          AND is_winner = TRUE
    `, userID).Scan(&summary.Wins); err != nil {
        return nil, fmt.Errorf("GetPlayerProgressSummary: wins: %w", err)
    }

    if summary.MatchesPlayed > 0 {
        summary.WinRate = int(float64(summary.Wins) * 100.0 / float64(summary.MatchesPlayed))
    }

    return summary, nil
}

func UpdatePlayerIdentity(userID int, name string, image string) error {
    query := `
        UPDATE players
        SET name = $1,
            image = $2
        WHERE user_id = $3
    `
    if _, err := DB.Exec(query, name, image, userID); err != nil {
        return fmt.Errorf("UpdatePlayerIdentity: %w", err)
    }
    return nil
}

func GetPlayerIdentityDefaults(userID int) (string, string, error) {
    var name sql.NullString
    var image sql.NullString
    if err := DB.QueryRow(`SELECT name, image FROM players WHERE user_id = $1`, userID).Scan(&name, &image); err != nil {
        return "", "", fmt.Errorf("GetPlayerIdentityDefaults: %w", err)
    }

    resolvedName := name.String
    if resolvedName == "" {
        resolvedName = "Player"
    }

    resolvedImage := image.String
    if resolvedImage == "" {
        resolvedImage = "/ranger/ranger.webp"
    }

    return resolvedName, resolvedImage, nil
}
