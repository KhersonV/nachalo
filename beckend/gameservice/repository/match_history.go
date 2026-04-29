package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"gameservice/models"
)

type CompletedMatchRecord struct {
	Match       models.MatchInfo
	PlayerStats models.PlayerMatchStat
}

func scanCompletedMatchRecord(
	scan func(dest ...interface{}) error,
) (*CompletedMatchRecord, error) {
	var record CompletedMatchRecord
	var winnerUserIDsRaw []byte
	var participantsRaw []byte

	err := scan(
		&record.Match.InstanceID,
		&record.Match.Mode,
		&record.Match.WinnerID,
		&record.Match.WinnerGroupID,
		&winnerUserIDsRaw,
		&participantsRaw,
		&record.Match.FinishedAt,
		&record.PlayerStats.InstanceID,
		&record.PlayerStats.UserID,
		&record.PlayerStats.PlayerName,
		&record.PlayerStats.GroupID,
		&record.PlayerStats.CharacterType,
		&record.PlayerStats.IsWinner,
		&record.PlayerStats.Survived,
		&record.PlayerStats.Deaths,
		&record.PlayerStats.ExpGained,
		&record.PlayerStats.Rewards,
		&record.PlayerStats.PlayerKills,
		&record.PlayerStats.MonsterKills,
		&record.PlayerStats.DamageTotal,
		&record.PlayerStats.DamageToPlayers,
		&record.PlayerStats.DamageToMonsters,
		&record.PlayerStats.DamageTaken,
		&record.PlayerStats.Placement,
		&record.PlayerStats.InventorySnapshot,
	)
	if err != nil {
		return nil, err
	}

	if len(winnerUserIDsRaw) > 0 {
		if err := json.Unmarshal(winnerUserIDsRaw, &record.Match.WinnerUserIDs); err != nil {
			record.Match.WinnerUserIDs = []int{}
		}
	}
	if len(participantsRaw) > 0 {
		if err := json.Unmarshal(participantsRaw, &record.Match.Participants); err != nil {
			record.Match.Participants = []models.MatchParticipantSnapshot{}
		}
	}

	return &record, nil
}

func ListCompletedMatchesForUser(userID int) ([]CompletedMatchRecord, error) {
	rows, err := DB.Query(`
		SELECT
			ms.instance_id,
			COALESCE(ms.mode, ''),
			ms.winner_id,
			ms.winner_group_id,
			COALESCE(ms.winner_user_ids, '[]'::jsonb),
			COALESCE(ms.participants, '[]'::jsonb),
			ms.created_at,
			mps.instance_id,
			mps.user_id,
			COALESCE(mps.player_name, ''),
			COALESCE(mps.group_id, 0),
			COALESCE(mps.character_type, 'guardian'),
			COALESCE(mps.is_winner, FALSE),
			COALESCE(mps.survived, FALSE),
			COALESCE(mps.deaths, 0),
			mps.exp_gained,
			mps.rewards,
			mps.player_kills,
			mps.monster_kills,
			mps.damage_total,
			mps.damage_to_players,
			mps.damage_to_monsters,
			COALESCE(mps.damage_taken, 0),
			COALESCE(mps.placement, 0),
			COALESCE(mps.inventory_snapshot, '{}'::jsonb)
		FROM match_player_stats mps
		INNER JOIN match_stats ms ON ms.instance_id = mps.instance_id
		WHERE mps.user_id = $1
		ORDER BY ms.created_at DESC, ms.instance_id DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("ListCompletedMatchesForUser query: %w", err)
	}
	defer rows.Close()

	records := make([]CompletedMatchRecord, 0)
	for rows.Next() {
		record, err := scanCompletedMatchRecord(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("ListCompletedMatchesForUser scan: %w", err)
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListCompletedMatchesForUser rows: %w", err)
	}

	return records, nil
}

func GetCompletedMatchForUser(userID int, instanceID string) (*CompletedMatchRecord, error) {
	row := DB.QueryRow(`
		SELECT
			ms.instance_id,
			COALESCE(ms.mode, ''),
			ms.winner_id,
			ms.winner_group_id,
			COALESCE(ms.winner_user_ids, '[]'::jsonb),
			COALESCE(ms.participants, '[]'::jsonb),
			ms.created_at,
			mps.instance_id,
			mps.user_id,
			COALESCE(mps.player_name, ''),
			COALESCE(mps.group_id, 0),
			COALESCE(mps.character_type, 'guardian'),
			COALESCE(mps.is_winner, FALSE),
			COALESCE(mps.survived, FALSE),
			COALESCE(mps.deaths, 0),
			mps.exp_gained,
			mps.rewards,
			mps.player_kills,
			mps.monster_kills,
			mps.damage_total,
			mps.damage_to_players,
			mps.damage_to_monsters,
			COALESCE(mps.damage_taken, 0),
			COALESCE(mps.placement, 0),
			COALESCE(mps.inventory_snapshot, '{}'::jsonb)
		FROM match_player_stats mps
		INNER JOIN match_stats ms ON ms.instance_id = mps.instance_id
		WHERE mps.user_id = $1
		  AND mps.instance_id = $2
	`, userID, instanceID)

	record, err := scanCompletedMatchRecord(row.Scan)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func StoredMatchExists(instanceID string) (bool, error) {
	var exists bool
	if err := DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1
			FROM match_stats
			WHERE instance_id = $1
		)
	`, instanceID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func CompletedMatchParticipantExists(instanceID string, userID int) (bool, error) {
	var exists bool
	if err := DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1
			FROM match_player_stats
			WHERE instance_id = $1 AND user_id = $2
		)
	`, instanceID, userID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func ActiveMatchExists(instanceID string) (bool, error) {
	var exists bool
	if err := DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1
			FROM matches
			WHERE instance_id = $1
		)
	`, instanceID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func ActiveMatchParticipantExists(instanceID string, userID int) (bool, error) {
	var exists bool
	if err := DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1
			FROM match_players
			WHERE instance_id = $1 AND user_id = $2
		)
	`, instanceID, userID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func IsNotFoundError(err error) bool {
	return err == sql.ErrNoRows
}
