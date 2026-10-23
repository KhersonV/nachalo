package repository

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"gameservice/game"
	"gameservice/models"
)

// Save completed HEX match history in the same transaction as its result.
func (c *MatchCommand) saveMatchStatsTx(ctx context.Context, mode string, results *game.MatchResults) error {
	participants := make([]models.MatchParticipantSnapshot, 0, len(results.PlayerResults))
	for _, r := range results.PlayerResults {
		participants = append(participants, models.MatchParticipantSnapshot{UserID: r.UserID, Name: r.PlayerName, GroupID: r.GroupID, CharacterType: r.CharacterType, Placement: r.Placement, IsWinner: r.IsWinner, Survived: r.Survived})
	}
	sort.SliceStable(participants, func(i, j int) bool {
		if participants[i].Placement != participants[j].Placement {
			return participants[i].Placement < participants[j].Placement
		}
		return participants[i].UserID < participants[j].UserID
	})
	winners, err := json.Marshal(results.WinnerUserIDs)
	if err != nil {
		return err
	}
	people, err := json.Marshal(participants)
	if err != nil {
		return err
	}
	_, err = c.Tx.ExecContext(ctx, `INSERT INTO match_stats(instance_id,mode,winner_id,winner_group_id,winner_user_ids,participants,created_at) VALUES($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7) ON CONFLICT(instance_id) DO UPDATE SET mode=EXCLUDED.mode,winner_id=EXCLUDED.winner_id,winner_group_id=EXCLUDED.winner_group_id,winner_user_ids=EXCLUDED.winner_user_ids,participants=EXCLUDED.participants,created_at=EXCLUDED.created_at`, c.InstanceID, mode, results.WinnerID, results.WinnerGroupID, winners, people, time.Now().UTC())
	return err
}

func (c *MatchCommand) savePlayerStatsTx(ctx context.Context, p game.PlayerResult) error {
	_, err := c.Tx.ExecContext(ctx, `INSERT INTO match_player_stats(instance_id,user_id,player_name,group_id,character_type,is_winner,survived,deaths,exp_gained,rewards,player_kills,monster_kills,damage_total,damage_to_players,damage_to_monsters,damage_taken,placement,inventory_snapshot) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$12,$13,$14,$15,$16,$17,$18::jsonb) ON CONFLICT(instance_id,user_id) DO UPDATE SET player_name=EXCLUDED.player_name,group_id=EXCLUDED.group_id,character_type=EXCLUDED.character_type,is_winner=EXCLUDED.is_winner,survived=EXCLUDED.survived,deaths=EXCLUDED.deaths,exp_gained=EXCLUDED.exp_gained,rewards=EXCLUDED.rewards,player_kills=EXCLUDED.player_kills,monster_kills=EXCLUDED.monster_kills,damage_total=EXCLUDED.damage_total,damage_to_players=EXCLUDED.damage_to_players,damage_to_monsters=EXCLUDED.damage_to_monsters,damage_taken=EXCLUDED.damage_taken,placement=EXCLUDED.placement,inventory_snapshot=EXCLUDED.inventory_snapshot`, c.InstanceID, p.UserID, p.PlayerName, p.GroupID, p.CharacterType, p.IsWinner, p.Survived, p.Deaths, p.ExpGained, p.RewardsData, p.PlayerKills, p.MonsterKills, p.DamageTotal, p.DamageToPlayers, p.DamageToMonsters, p.DamageTaken, p.Placement, p.InventorySnapshot)
	return err
}
