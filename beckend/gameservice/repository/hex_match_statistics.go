package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"gameservice/game"
)

// Called from MatchCommand.Commit while its matches row is locked. A finished
// HEX match gets one durable history row and one payout per unpaid participant.
func (c *MatchCommand) persistHexCompletedMatchTx(ctx context.Context) error {
	var mode string
	var winnerID, winnerGroupID int
	err := c.Tx.QueryRowContext(ctx, `SELECT COALESCE(to_jsonb(matches)->>'mode',''),COALESCE((to_jsonb(matches)->>'winner_id')::integer,0),COALESCE((to_jsonb(matches)->>'winner_group_id')::integer,0) FROM matches WHERE instance_id=$1 AND to_jsonb(matches)->'geometry'->>'kind'='hex' AND finished_at IS NOT NULL`, c.InstanceID).Scan(&mode, &winnerID, &winnerGroupID)
	if err == sql.ErrNoRows || mode == "hex-test" {
		return nil
	}
	if err != nil {
		return err
	}
	var recorded bool
	if err := c.Tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM match_stats WHERE instance_id=$1)`, c.InstanceID).Scan(&recorded); err != nil {
		return err
	}
	if recorded {
		return nil
	}

	events, err := c.LoadCombatResultEventsTx(ctx)
	if err != nil {
		return err
	}
	methods := map[int]string{}
	transferredItems := map[int][]byte{}
	paidXP, paidGold := map[int]int{}, map[int]int{}
	resultRows, err := c.Tx.QueryContext(ctx, `SELECT user_id,result->>'method',result->'transferred',COALESCE((result->>'xp')::integer,0),COALESCE((result->>'goldEarned')::integer,0) FROM hex_player_results WHERE instance_id=$1`, c.InstanceID)
	if err != nil {
		return err
	}
	for resultRows.Next() {
		var id int
		var method sql.NullString
		var transferred []byte
		var xp, gold int
		if err := resultRows.Scan(&id, &method, &transferred, &xp, &gold); err != nil {
			resultRows.Close()
			return err
		}
		methods[id] = method.String
		paidXP[id], paidGold[id] = xp, gold
		if method.String == "paid" || method.String == "artifact" {
			transferredItems[id] = transferred
		}
	}
	if err := resultRows.Err(); err != nil {
		resultRows.Close()
		return err
	}
	resultRows.Close()

	results := &game.MatchResults{WinnerID: winnerID, WinnerGroupID: winnerGroupID}
	groups := map[int]int{}
	rows, err := c.Tx.QueryContext(ctx, `SELECT user_id,COALESCE(name,''),character_id,COALESCE(NULLIF(character_type,''),'guardian'),COALESCE(group_id,0),health,COALESCE(inventory,'{}'::jsonb) FROM match_players WHERE instance_id=$1 ORDER BY user_id`, c.InstanceID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, group, health int
		var name, class string
		var characterID sql.NullInt64
		var inventory []byte
		if err := rows.Scan(&id, &name, &characterID, &class, &group, &health, &inventory); err != nil {
			rows.Close()
			return err
		}
		method := methods[id]
		survived := health > 0 || method == "paid" || method == "artifact" || method == "team_victory" || method == "opponent_extracted"
		if method == "defeat" {
			survived = false
		}
		xp, rewards, pk, mk, total, players, monsters, taken, err := game.CalculateResultsFromEvents(events, id, survived)
		if err != nil {
			rows.Close()
			return err
		}
		rewardJSON, err := json.Marshal(rewards)
		if err != nil {
			rows.Close()
			return err
		}
		player := game.PlayerResult{UserID: id, PlayerName: name, GroupID: group, CharacterType: class, Survived: survived, Deaths: boolToInt(!survived), ExpGained: xp, RewardsData: rewardJSON, PlayerKills: pk, MonsterKills: mk, DamageTotal: total, DamageToPlayers: players, DamageToMonsters: monsters, DamageTaken: taken, InventorySnapshot: append([]byte(nil), inventory...)}
		if transferred := transferredItems[id]; len(transferred) > 0 && string(transferred) != "null" {
			player.InventorySnapshot = append([]byte(nil), transferred...)
		}
		if characterID.Valid {
			player.CharacterID = int(characterID.Int64)
		}
		results.PlayerResults = append(results.PlayerResults, player)
		groups[id] = group
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(results.PlayerResults) == 0 {
		return ErrCommandRejected
	}
	applyWinnerBonusesAndPlacements(results, groups, events.DefeatedUsers)
	if err := c.saveMatchStatsTx(ctx, mode, results); err != nil {
		return err
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_stats SET created_at=(SELECT finished_at FROM matches WHERE instance_id=$1) WHERE instance_id=$1`, c.InstanceID); err != nil {
		return err
	}
	for _, player := range results.PlayerResults {
		if err := c.savePlayerStatsTx(ctx, player); err != nil {
			return err
		}
		method := methods[player.UserID]
		xpDelta := hexPayoutDelta(player.ExpGained, paidXP[player.UserID])
		if player.CharacterID > 0 && xpDelta > 0 {
			if err := grantHexExperienceTx(ctx, c.Tx, player.CharacterID, xpDelta); err != nil {
				return err
			}
		}
		rewards, err := parseFinalRewards(player.RewardsData)
		if err != nil {
			return err
		}
		gold := 0
		for kind, amount := range rewards {
			if kind == "balance" || kind == "coin" || kind == "coins" {
				gold += amount
			}
		}
		goldDelta := hexPayoutDelta(gold, paidGold[player.UserID])
		if goldDelta > 0 {
			if _, err := c.Tx.ExecContext(ctx, `UPDATE player_profiles SET balance=balance+$2,updated_at=now() WHERE user_id=$1`, player.UserID, goldDelta); err != nil {
				return err
			}
		}
		if method == "" {
			method = "defeat"
			if player.IsWinner {
				method = "victory"
			}
		}
		stats := HexResultStats{PlayerKills: player.PlayerKills, MonsterKills: player.MonsterKills, DamageTotal: player.DamageTotal, DamageToPlayers: player.DamageToPlayers, DamageToMonsters: player.DamageToMonsters, DamageTaken: player.DamageTaken}
		statsJSON, err := json.Marshal(stats)
		if err != nil {
			return err
		}
		_, err = c.Tx.ExecContext(ctx, `INSERT INTO hex_player_results(instance_id,user_id,result) VALUES($1,$2,jsonb_build_object('method',$3::text,'xp',$4::integer,'goldEarned',$5::integer,'goldSpent',0,'stats',$6::jsonb,'isWinner',$7::boolean,'transferred','{}'::jsonb,'lost','{}'::jsonb,'equipment','[]'::jsonb,'lostEquipment','[]'::jsonb)) ON CONFLICT(instance_id,user_id) DO UPDATE SET result=hex_player_results.result || jsonb_build_object('xp',$4::integer,'goldEarned',$5::integer,'stats',$6::jsonb,'isWinner',$7::boolean)`, c.InstanceID, player.UserID, method, player.ExpGained, gold, statsJSON, player.IsWinner)
		if err != nil {
			return err
		}
	}
	return nil
}

func hexPayoutDelta(total, alreadyPaid int) int {
	if total <= alreadyPaid {
		return 0
	}
	return total - alreadyPaid
}
