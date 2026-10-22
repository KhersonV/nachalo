package repository

import (
	"database/sql"
	"fmt"
)

type PlayerProgressSummary struct {
	MatchesPlayed  int                   `json:"matchesPlayed"`
	Wins           int                   `json:"wins"`
	Losses         int                   `json:"losses"`
	WinRate        int                   `json:"winRate"`
	TotalExpGained int                   `json:"totalExpGained"`
	PlayerKills    int                   `json:"playerKills"`
	MonsterKills   int                   `json:"monsterKills"`
	Deaths         int                   `json:"deaths"`
	DamageTotal    int                   `json:"damageTotal"`
	DamageTaken    int                   `json:"damageTaken"`
	Classes        []PlayerClassProgress `json:"classes"`
}

type PlayerClassProgress struct {
	CharacterType  string `json:"characterType"`
	MatchesPlayed  int    `json:"matchesPlayed"`
	Wins           int    `json:"wins"`
	Losses         int    `json:"losses"`
	WinRate        int    `json:"winRate"`
	PlayerKills    int    `json:"playerKills"`
	MonsterKills   int    `json:"monsterKills"`
	Deaths         int    `json:"deaths"`
	DamageTotal    int    `json:"damageTotal"`
	DamageTaken    int    `json:"damageTaken"`
	TotalExpGained int    `json:"totalExpGained"`
}

func GetPlayerProgressSummary(userID int) (*PlayerProgressSummary, error) {
	summary := &PlayerProgressSummary{}

	if err := DB.QueryRow(`
        SELECT
            COUNT(*) AS matches_played,
			COALESCE(SUM(CASE WHEN mps.is_winner THEN 1 ELSE 0 END),0),
            COALESCE(SUM(exp_gained), 0) AS total_exp,
            COALESCE(SUM(player_kills), 0) AS player_kills,
            COALESCE(SUM(monster_kills), 0) AS monster_kills,
			COALESCE(SUM(deaths),0),
			COALESCE(SUM(damage_total), 0) AS damage_total,
			COALESCE(SUM(damage_taken),0)
        FROM match_player_stats mps JOIN match_stats ms ON ms.instance_id=mps.instance_id
        WHERE mps.user_id = $1
    `, userID).Scan(
		&summary.MatchesPlayed,
		&summary.Wins,
		&summary.TotalExpGained,
		&summary.PlayerKills,
		&summary.MonsterKills,
		&summary.Deaths,
		&summary.DamageTotal,
		&summary.DamageTaken,
	); err != nil {
		return nil, fmt.Errorf("GetPlayerProgressSummary: aggregate stats: %w", err)
	}

	summary.Losses = summary.MatchesPlayed - summary.Wins
	if summary.MatchesPlayed > 0 {
		summary.WinRate = int(float64(summary.Wins) * 100.0 / float64(summary.MatchesPlayed))
	}
	summary.Classes = []PlayerClassProgress{}
	rows, err := DB.Query(`SELECT character_type,COUNT(*),COALESCE(SUM(CASE WHEN is_winner THEN 1 ELSE 0 END),0),COALESCE(SUM(player_kills),0),COALESCE(SUM(monster_kills),0),COALESCE(SUM(deaths),0),COALESCE(SUM(damage_total),0),COALESCE(SUM(damage_taken),0),COALESCE(SUM(exp_gained),0) FROM match_player_stats WHERE user_id=$1 AND character_type IN ('guardian','ranger','berserker','mystic') GROUP BY character_type`, userID)
	if err != nil {
		return nil, fmt.Errorf("GetPlayerProgressSummary: classes: %w", err)
	}
	defer rows.Close()
	byClass := map[string]PlayerClassProgress{}
	for rows.Next() {
		var entry PlayerClassProgress
		if err := rows.Scan(&entry.CharacterType, &entry.MatchesPlayed, &entry.Wins, &entry.PlayerKills, &entry.MonsterKills, &entry.Deaths, &entry.DamageTotal, &entry.DamageTaken, &entry.TotalExpGained); err != nil {
			return nil, err
		}
		entry.Losses = entry.MatchesPlayed - entry.Wins
		if entry.MatchesPlayed > 0 {
			entry.WinRate = entry.Wins * 100 / entry.MatchesPlayed
		}
		byClass[entry.CharacterType] = entry
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, class := range []string{"guardian", "ranger", "berserker", "mystic"} {
		entry, ok := byClass[class]
		if !ok {
			entry = PlayerClassProgress{CharacterType: class}
		}
		summary.Classes = append(summary.Classes, entry)
	}

	return summary, nil
}

func UpdatePlayerIdentity(userID int, name string, image string) error {
	query := `
        UPDATE player_profiles
        SET name = $1,
            image = $2,
            updated_at = CURRENT_TIMESTAMP
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
	if err := DB.QueryRow(`SELECT name, image FROM player_profiles WHERE user_id = $1`, userID).Scan(&name, &image); err != nil {
		return "", "", fmt.Errorf("GetPlayerIdentityDefaults: %w", err)
	}

	resolvedName := name.String
	if resolvedName == "" {
		resolvedName = "Player"
	}

	resolvedImage := image.String
	if resolvedImage == "" {
		resolvedImage = defaultProfileImage
	}

	return resolvedName, resolvedImage, nil
}
