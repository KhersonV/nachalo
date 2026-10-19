package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"gameservice/game"
	"gameservice/models"
)

// MatchFinalizationResult is committed to the command ledger. Retaining the
// match row makes this result replayable after completion or process restart.
type MatchFinalizationResult struct {
	Results       game.MatchResults `json:"results"`
	StateRevision int64             `json:"stateRevision"`
	MapRevision   int64             `json:"mapRevision"`
}

func (r *MatchFinalizationResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

// FinalizeMatchTx joins an existing MatchCommand, including one which has just
// applied a lethal action. It never locks the match again or commits. The
// completeness flag prevents awarding from partial process-local combat data.
func (c *MatchCommand) FinalizeMatchTx(ctx context.Context) (*game.MatchResults, error) {
	if c == nil || c.Tx == nil || c.Finished {
		return nil, ErrCommandRejected
	}
	var mode string
	var winnerID, winnerGroupID int
	var complete bool
	var finished sql.NullTime
	err := c.Tx.QueryRowContext(ctx, `SELECT mode,COALESCE(winner_id,0),COALESCE(winner_group_id,0),combat_results_complete,finished_at FROM matches WHERE instance_id=$1`, c.InstanceID).Scan(&mode, &winnerID, &winnerGroupID, &complete, &finished)
	if err != nil {
		return nil, err
	}
	if finished.Valid || !complete {
		return nil, ErrCommandRejected
	}
	events, err := c.LoadCombatResultEventsTx(ctx)
	if err != nil {
		return nil, err
	}
	results, err := c.calculateMatchResultsTx(ctx, events, winnerID, winnerGroupID)
	if err != nil {
		return nil, err
	}
	if err = c.saveMatchStatsTx(ctx, mode, results); err != nil {
		return nil, err
	}
	for _, result := range results.PlayerResults {
		if err = c.savePlayerStatsTx(ctx, result); err != nil {
			return nil, err
		}
		if err = c.applyFinalPlayerRewardsTx(ctx, result); err != nil {
			return nil, err
		}
	}
	_, err = c.Tx.ExecContext(ctx, `UPDATE matches SET finished_at=now() WHERE instance_id=$1 AND finished_at IS NULL`, c.InstanceID)
	if err != nil {
		return nil, err
	}
	return results, nil
}

// ApplyMatchFinalization is the standalone entrypoint. A death command can
// instead call FinalizeMatchTx on its existing MatchCommand and commit once.
// The callback runs only after the ledger, rewards and finished marker commit.
func ApplyMatchFinalization(ctx context.Context, instanceID string, userID int, commandID string, publish func(MatchFinalizationResult)) (MatchFinalizationResult, error) {
	var response MatchFinalizationResult
	c, err := BeginMatchCommand(ctx, instanceID, userID, commandID, fmt.Sprintf("finalize-v1:%d", userID))
	if err != nil {
		return response, err
	}
	defer c.Rollback()
	if c.Replay != nil {
		if err = json.Unmarshal(c.Replay, &response); err != nil {
			return response, err
		}
		return response, nil
	}
	results, err := c.FinalizeMatchTx(ctx)
	if err != nil {
		return response, err
	}
	response.Results = *results
	if _, _, err = c.Commit(ctx, &response, false); err != nil {
		return MatchFinalizationResult{}, err
	}
	game.DeleteMatchStateInMemory(instanceID)
	if publish != nil {
		publish(response)
	}
	return response, nil
}

func (c *MatchCommand) calculateMatchResultsTx(ctx context.Context, events game.ResultEvents, winnerID, winnerGroupID int) (*game.MatchResults, error) {
	results := &game.MatchResults{WinnerID: winnerID, WinnerGroupID: winnerGroupID}
	rows, err := c.Tx.QueryContext(ctx, `SELECT user_id,COALESCE(name,''),character_id,COALESCE(NULLIF(character_type,''),'guardian'),COALESCE(group_id,0),health,COALESCE(inventory,'{}'::jsonb) FROM match_players WHERE instance_id=$1 ORDER BY user_id`, c.InstanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groupByUser := make(map[int]int)
	for rows.Next() {
		var userID, groupID, health int
		var name, characterType string
		var characterID sql.NullInt64
		var inventory []byte
		if err = rows.Scan(&userID, &name, &characterID, &characterType, &groupID, &health, &inventory); err != nil {
			return nil, err
		}
		survived := health > 0
		exp, rewards, killsP, killsM, dmgTotal, dmgPlayers, dmgMonsters, dmgTaken, calcErr := game.CalculateResultsFromEvents(events, userID, survived)
		if calcErr != nil {
			return nil, calcErr
		}
		rewardJSON, marshalErr := json.Marshal(rewards)
		if marshalErr != nil {
			return nil, marshalErr
		}
		pr := game.PlayerResult{UserID: userID, PlayerName: name, GroupID: groupID, CharacterType: characterType, Survived: survived, Deaths: boolToInt(!survived), ExpGained: exp, RewardsData: rewardJSON, PlayerKills: killsP, MonsterKills: killsM, DamageTotal: dmgTotal, DamageToPlayers: dmgPlayers, DamageToMonsters: dmgMonsters, DamageTaken: dmgTaken, InventorySnapshot: append([]byte(nil), inventory...)}
		if characterID.Valid {
			pr.CharacterID = int(characterID.Int64)
		}
		results.PlayerResults = append(results.PlayerResults, pr)
		groupByUser[userID] = groupID
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(results.PlayerResults) == 0 {
		return nil, ErrCommandRejected
	}
	applyWinnerBonusesAndPlacements(results, groupByUser, events.DefeatedUsers)
	return results, nil
}

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

func (c *MatchCommand) applyFinalPlayerRewardsTx(ctx context.Context, p game.PlayerResult) error {
	// Preserve SyncPersistentInventoryFromMatchResources: resources replace the
	// account inventory; quest artifacts are omitted before rewards are added.
	finalInv := make(map[string]map[string]interface{})
	if len(p.InventorySnapshot) > 0 && string(p.InventorySnapshot) != "{}" {
		if err := json.Unmarshal(p.InventorySnapshot, &finalInv); err != nil {
			finalInv = make(map[string]map[string]interface{})
		}
	}
	for key := range finalInv {
		if strings.HasPrefix(key, "artifact_") || key == "artifact" {
			delete(finalInv, key)
		}
	}
	var balance int
	var currentInv []byte
	if err := c.Tx.QueryRowContext(ctx, `SELECT balance,inventory FROM player_profiles WHERE user_id=$1 FOR UPDATE`, p.UserID).Scan(&balance, &currentInv); err != nil {
		return err
	}
	_ = currentInv // legacy sync replaces it with the match resource snapshot.
	if p.CharacterID > 0 {
		var level, exp int
		if err := c.Tx.QueryRowContext(ctx, `SELECT level,exp FROM player_characters WHERE id=$1 FOR UPDATE`, p.CharacterID).Scan(&level, &exp); err != nil {
			return err
		}
		exp += p.ExpGained
		for {
			threshold, ok := levelThresholds[level]
			if !ok || exp < threshold {
				break
			}
			level++
			exp -= threshold
		}
		if _, err := c.Tx.ExecContext(ctx, `UPDATE player_characters SET level=$2,exp=$3,max_exp=$4,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, p.CharacterID, level, exp, maxExperienceForLevel(level)); err != nil {
			return err
		}
	}
	rewards, err := parseFinalRewards(p.RewardsData)
	if err != nil {
		return err
	}
	for key, amount := range rewards {
		if strings.HasPrefix(key, "artifact_") || key == "artifact" {
			continue
		}
		switch key {
		case "balance", "coin", "coins":
			balance += amount
		default:
			finalInv[key] = map[string]interface{}{"name": key, "item_count": amount}
		}
	}
	encoded, err := json.Marshal(finalInv)
	if err != nil {
		return err
	}
	_, err = c.Tx.ExecContext(ctx, `UPDATE player_profiles SET balance=$2,inventory=$3::jsonb,updated_at=CURRENT_TIMESTAMP WHERE user_id=$1`, p.UserID, balance, encoded)
	return err
}

func parseFinalRewards(raw []byte) (map[string]int, error) {
	rewards := make(map[string]int)
	var mapped map[string]int
	if err := json.Unmarshal(raw, &mapped); err == nil {
		for k, v := range mapped {
			rewards[k] += v
		}
		return rewards, nil
	}
	var list []game.Reward
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	for _, r := range list {
		if r.Type != "" && r.Amount != 0 {
			rewards[r.Type] += r.Amount
		}
	}
	return rewards, nil
}
