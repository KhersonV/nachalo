package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gameservice/game"
	"gameservice/internal/hexgrid"
)

var ErrHexPortalPosition = errors.New("stand on the HEX portal to extract")
var ErrHexExtractionMethod = errors.New("extraction method does not match quest artifact")

const hexPaidExtractionCost = 50
const hexQuestGoldBonus = 100 // Square winner bonus in applyWinnerBonusesAndPlacements.
const hexQuestXPBonus = 120

// jsonb_build_object accepts polymorphic values. Parameters used only there
// need explicit PostgreSQL types; otherwise lib/pq fails to prepare this query
// with "could not determine data type of parameter $4".
const hexQuestParticipantResultUpsertSQL = `INSERT INTO hex_player_results(instance_id,user_id,result)
	VALUES($1,$2,$3::jsonb)
	ON CONFLICT(instance_id,user_id) DO UPDATE
	SET result=hex_player_results.result || jsonb_build_object('reason',$4::text,'winnerUserId',$5::integer,'winnerTeamId',$6::integer)`

type HexPlayerResult struct {
	Method             string                    `json:"method"`
	Reason             string                    `json:"reason,omitempty"`
	WinnerUserID       int                       `json:"winnerUserId,omitempty"`
	WinnerTeamID       int                       `json:"winnerTeamId,omitempty"`
	GoldEarned         int                       `json:"goldEarned"`
	GoldSpent          int                       `json:"goldSpent"`
	XP                 int                       `json:"xp"`
	Transferred        map[string]map[string]any `json:"transferred"`
	Lost               map[string]map[string]any `json:"lost"`
	Equipment          []string                  `json:"equipment"`
	LostEquipment      []string                  `json:"lostEquipment"`
	EquipmentItems     []HexResultItem           `json:"equipmentItems,omitempty"`
	LostEquipmentItems []HexResultItem           `json:"lostEquipmentItems,omitempty"`
	WornEquipment      []HexResultItem           `json:"wornEquipment,omitempty"`
	Stats              *HexResultStats           `json:"stats,omitempty"`
	ActiveUser         int                       `json:"activeUser"`
	TurnNumber         int                       `json:"turnNumber"`
	StateRevision      int64                     `json:"stateRevision"`
	MapRevision        int64                     `json:"mapRevision"`
	Replayed           bool                      `json:"-"`
}

type HexResultItem struct {
	Name   string `json:"name"`
	Image  string `json:"image,omitempty"`
	Status string `json:"status,omitempty"`
}

type HexResultStats struct {
	PlayerKills      int `json:"playerKills"`
	MonsterKills     int `json:"monsterKills"`
	DamageTotal      int `json:"damageTotal"`
	DamageToPlayers  int `json:"damageToPlayers"`
	DamageToMonsters int `json:"damageToMonsters"`
	DamageTaken      int `json:"damageTaken"`
}

func (r *HexPlayerResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

func ApplyHexExtraction(ctx context.Context, instanceID string, userID int, commandID, method string, confirmedPaid bool, publish func(HexPlayerResult)) (HexPlayerResult, error) {
	var out HexPlayerResult
	if method != "artifact" && method != "paid" {
		return out, ErrHexExtractionMethod
	}
	if method == "paid" && !confirmedPaid {
		return out, ErrHexExtractionMethod
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c, err := BeginMatchCommand(ctx, instanceID, userID, commandID, fmt.Sprintf("hex-extract-v1:%d:%s:%t", userID, method, confirmedPaid))
	if err != nil {
		return out, err
	}
	defer c.Rollback()
	if c.Replay != nil {
		if err := json.Unmarshal(c.Replay, &out); err != nil {
			return out, err
		}
		out.Replayed = true
		return out, nil
	}
	if c.ActiveUser != userID {
		return out, ErrCommandRejected
	}
	var mode, geometry string
	var version, questID, turn int
	var rewardsReady bool
	if err := c.Tx.QueryRowContext(ctx, `SELECT mode,geometry->>'kind',schema_version,COALESCE(quest_artifact_id,0),turn_number,hex_reward_events_ready FROM matches WHERE instance_id=$1`, instanceID).Scan(&mode, &geometry, &version, &questID, &turn, &rewardsReady); err != nil {
		return out, err
	}
	if mode == "hex-test" || geometry != "hex" || version != 2 || !rewardsReady {
		return out, ErrHexUnavailable
	}
	var pos hexgrid.Coord
	var health, characterID int
	var backpackRaw []byte
	if err := c.Tx.QueryRowContext(ctx, `SELECT position_q,position_r,health,character_id,inventory FROM match_players WHERE instance_id=$1 AND user_id=$2 FOR UPDATE`, instanceID, userID).Scan(&pos.Q, &pos.R, &health, &characterID, &backpackRaw); err != nil {
		return out, err
	}
	if health <= 0 {
		return out, ErrCommandRejected
	}
	var onPortal bool
	if err := c.Tx.QueryRowContext(ctx, `SELECT is_portal FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3`, instanceID, pos.Q, pos.R).Scan(&onPortal); err != nil {
		return out, err
	}
	if !onPortal {
		return out, ErrHexPortalPosition
	}
	var hasQuest bool
	if questID > 0 {
		if err := c.Tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM inventory_items WHERE instance_id=$1 AND user_id=$2 AND item_type='artifact' AND item_id=$3 AND item_count>0)`, instanceID, userID, questID).Scan(&hasQuest); err != nil {
			return out, err
		}
	}
	if (hasQuest && method != "artifact") || (!hasQuest && method != "paid") {
		return out, ErrHexExtractionMethod
	}
	backpack, err := parseStorage(backpackRaw)
	if err != nil {
		return out, err
	}
	if err := enrichHexResultItemsTx(ctx, c.Tx, backpack); err != nil {
		return out, err
	}
	if hasQuest && storageCount(backpack[fmt.Sprintf("artifact_%d", questID)]) == 0 {
		return out, ErrCommandRejected
	}
	var storageRaw []byte
	var balance int
	if err := c.Tx.QueryRowContext(ctx, `SELECT inventory,balance FROM player_profiles WHERE user_id=$1 FOR UPDATE`, userID).Scan(&storageRaw, &balance); err != nil {
		return out, err
	}
	if method == "paid" && balance < hexPaidExtractionCost {
		return out, ErrStorageInsufficientGold
	}
	storage, err := parseStorage(storageRaw)
	if err != nil {
		return out, err
	}
	transferred, err := mergeHexExtractionStorage(storage, backpack, questID)
	if err != nil {
		return out, err
	}
	equipment, err := transferHexEquipmentTx(ctx, c.Tx, instanceID, userID)
	if err != nil {
		return out, err
	}
	wornEquipment, err := loadHexWornEquipmentTx(ctx, c.Tx, characterID)
	if err != nil {
		return out, err
	}
	events, err := c.LoadCombatResultEventsTx(ctx)
	if err != nil {
		return out, err
	}
	xp, rewards, playerKills, monsterKills, damageTotal, damagePlayers, damageMonsters, damageTaken, err := game.CalculateResultsFromEvents(events, userID, true)
	if err != nil {
		return out, err
	}
	gold := 0
	for _, reward := range rewards {
		if reward.Type != "balance" {
			return out, ErrCommandRejected
		}
		gold += reward.Amount
	}
	if hasQuest {
		gold += hexQuestGoldBonus
		xp += hexQuestXPBonus
	}
	spent := 0
	if method == "paid" {
		spent = hexPaidExtractionCost
	}
	encodedStorage, err := json.Marshal(storage)
	if err != nil {
		return out, err
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE player_profiles SET inventory=$2::jsonb,balance=balance+$3-$4,updated_at=now() WHERE user_id=$1`, userID, encodedStorage, gold, spent); err != nil {
		return out, err
	}
	if err := grantHexExperienceTx(ctx, c.Tx, characterID, xp); err != nil {
		return out, err
	}
	if _, err := c.Tx.ExecContext(ctx, `DELETE FROM inventory_items WHERE instance_id=$1 AND user_id=$2`, instanceID, userID); err != nil {
		return out, err
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET inventory='{}'::jsonb,health=0 WHERE instance_id=$1 AND user_id=$2 AND health>0`, instanceID, userID)
	if err != nil {
		return out, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return out, ErrCommandRejected
	}
	result, err = c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=false WHERE instance_id=$1 AND q=$2 AND r=$3 AND is_player=true`, instanceID, pos.Q, pos.R)
	if err != nil {
		return out, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return out, ErrCommandRejected
	}
	activeUser := 0
	if method != "artifact" {
		turnState, err := c.RemoveCombatPlayerFromTurnOrderTx(ctx, userID)
		if err != nil {
			return out, err
		}
		activeUser = turnState.ActiveUserID
		if err := c.ResetHexTurnDeadlineTx(ctx, activeUser); err != nil {
			return out, err
		}
		if activeUser == 0 {
			if _, err := c.Tx.ExecContext(ctx, `UPDATE matches SET finished_at=now(),hex_turn_deadline=NULL WHERE instance_id=$1`, instanceID); err != nil {
				return out, err
			}
		}
	}
	equipmentNames := make([]string, 0, len(equipment))
	for _, item := range equipment {
		equipmentNames = append(equipmentNames, item.Name)
	}
	out = HexPlayerResult{Method: method, GoldEarned: gold, GoldSpent: spent, XP: xp, Transferred: transferred, Lost: map[string]map[string]any{}, Equipment: equipmentNames, LostEquipment: []string{}, EquipmentItems: equipment, WornEquipment: wornEquipment, Stats: &HexResultStats{PlayerKills: playerKills, MonsterKills: monsterKills, DamageTotal: damageTotal, DamageToPlayers: damagePlayers, DamageToMonsters: damageMonsters, DamageTaken: damageTaken}, ActiveUser: activeUser, TurnNumber: turn}
	if method == "artifact" {
		out.Reason = "quest_artifact_extracted"
		out.WinnerUserID = userID
		if err := c.Tx.QueryRowContext(ctx, `SELECT group_id FROM match_players WHERE instance_id=$1 AND user_id=$2`, instanceID, userID).Scan(&out.WinnerTeamID); err != nil {
			return out, err
		}
	}
	if err := c.Tx.QueryRowContext(ctx, `SELECT state_revision+1,map_revision+1 FROM matches WHERE instance_id=$1`, instanceID).Scan(&out.StateRevision, &out.MapRevision); err != nil {
		return out, err
	}
	encodedResult, err := json.Marshal(out)
	if err != nil {
		return out, err
	}
	if _, err := c.Tx.ExecContext(ctx, `INSERT INTO hex_player_results(instance_id,user_id,result) VALUES($1,$2,$3::jsonb)`, instanceID, userID, encodedResult); err != nil {
		return out, err
	}
	if method == "artifact" {
		if err := finishHexQuestMatchTx(ctx, c.Tx, instanceID, out, events); err != nil {
			return out, err
		}
	}
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, true)
	if err != nil {
		return HexPlayerResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}

// The matches row is locked by BeginMatchCommand. Results for every remaining
// participant and the terminal match state commit with the winner's payout.
func finishHexQuestMatchTx(ctx context.Context, tx *sql.Tx, instanceID string, winner HexPlayerResult, events game.ResultEvents) error {
	rows, err := tx.QueryContext(ctx, `SELECT user_id,group_id FROM match_players WHERE instance_id=$1 ORDER BY user_id`, instanceID)
	if err != nil {
		return err
	}
	type participant struct{ userID, teamID int }
	players := []participant{}
	for rows.Next() {
		var p participant
		if err := rows.Scan(&p.userID, &p.teamID); err != nil {
			rows.Close()
			return err
		}
		players = append(players, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, p := range players {
		if p.userID == winner.WinnerUserID {
			continue
		}
		method, reason := "opponent_extracted", "quest_artifact_extracted_by_opponent"
		if p.teamID == winner.WinnerTeamID {
			method, reason = "team_victory", "teammate_extracted_quest_artifact"
		}
		result := HexPlayerResult{Method: method, Reason: reason, WinnerUserID: winner.WinnerUserID, WinnerTeamID: winner.WinnerTeamID, Transferred: map[string]map[string]any{}, Lost: map[string]map[string]any{}, Equipment: []string{}, LostEquipment: []string{}, TurnNumber: winner.TurnNumber, StateRevision: winner.StateRevision, MapRevision: winner.MapRevision}
		_, _, pk, mk, total, dp, dm, taken, err := game.CalculateResultsFromEvents(events, p.userID, false)
		if err != nil {
			return err
		}
		result.Stats = &HexResultStats{PlayerKills: pk, MonsterKills: mk, DamageTotal: total, DamageToPlayers: dp, DamageToMonsters: dm, DamageTaken: taken}
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, hexQuestParticipantResultUpsertSQL, instanceID, p.userID, raw, reason, winner.WinnerUserID, winner.WinnerTeamID); err != nil {
			return err
		}
	}
	winningUser := winner.WinnerUserID
	if winner.WinnerTeamID > 0 {
		winningUser = 0
	}
	_, err = tx.ExecContext(ctx, `UPDATE matches SET finished_at=now(),active_user_id=NULL,turn_order='[]'::jsonb,hex_turn_deadline=NULL,winner_id=$2,winner_group_id=$3 WHERE instance_id=$1 AND finished_at IS NULL`, instanceID, winningUser, winner.WinnerTeamID)
	return err
}

func mergeHexExtractionStorage(storage, backpack map[string]map[string]any, questID int) (map[string]map[string]any, error) {
	transferred := map[string]map[string]any{}
	for key, item := range backpack {
		if key == fmt.Sprintf("artifact_%d", questID) && questID > 0 {
			continue
		}
		if !(strings.HasPrefix(key, "resource_") || strings.HasPrefix(key, "scroll_") || strings.HasPrefix(key, "artifact_") || strings.HasPrefix(key, "blueprint_")) {
			return nil, ErrCommandRejected
		}
		count := storageCount(item)
		if count <= 0 {
			return nil, ErrCommandRejected
		}
		copyItem := make(map[string]any, len(item))
		for field, value := range item {
			copyItem[field] = value
		}
		copyItem["item_count"] = count
		transferred[key] = copyItem
		current := storage[key]
		if current == nil {
			storage[key] = copyItem
			continue
		}
		if storageCount(current) > 1_000_000_000-count {
			return nil, ErrCommandRejected
		}
		current["item_count"] = storageCount(current) + count
	}
	return transferred, nil
}

// Match inventory already contains item metadata. Refresh resource/artifact
// labels from their catalogs for durable results and Storage presentation.
func enrichHexResultItemsTx(ctx context.Context, tx *sql.Tx, items map[string]map[string]any) error {
	for key, item := range items {
		parts := strings.SplitN(key, "_", 2)
		if len(parts) != 2 || (parts[0] != "resource" && parts[0] != "artifact") {
			continue
		}
		id, err := strconv.Atoi(parts[1])
		if err != nil || id <= 0 {
			return ErrCommandRejected
		}
		var name, image, description string
		query := `SELECT type,COALESCE(image,''),COALESCE(description,'') FROM resources WHERE id=$1`
		if parts[0] == "artifact" {
			query = `SELECT name,COALESCE(image,''),COALESCE(description,'') FROM artifacts WHERE id=$1`
		}
		err = tx.QueryRowContext(ctx, query, id).Scan(&name, &image, &description)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		item["name"], item["item_name"], item["image"], item["description"] = name, name, image, description
	}
	return nil
}

func transferHexEquipmentTx(ctx context.Context, tx *sql.Tx, instanceID string, userID int) ([]HexResultItem, error) {
	rows, err := tx.QueryContext(ctx, `SELECT it.name,it.image_url,ii.status FROM item_instances ii JOIN item_templates it ON it.id=ii.template_id WHERE ii.match_instance_id=$1 AND ii.owner_user_id=$2 FOR UPDATE OF ii`, instanceID, userID)
	if err != nil {
		return nil, err
	}
	items := []HexResultItem{}
	for rows.Next() {
		var item HexResultItem
		if err := rows.Scan(&item.Name, &item.Image, &item.Status); err != nil {
			rows.Close()
			return nil, err
		}
		if item.Status != "inventory" && item.Status != "equipped" {
			rows.Close()
			return nil, ErrCommandRejected
		}
		if item.Status == "inventory" {
			items = append(items, item)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	_, err = tx.ExecContext(ctx, `UPDATE item_instances SET match_instance_id=NULL,updated_at=now() WHERE match_instance_id=$1 AND owner_user_id=$2 AND status IN ('inventory','equipped')`, instanceID, userID)
	return items, err
}

func grantHexExperienceTx(ctx context.Context, tx *sql.Tx, characterID, exp int) error {
	if exp <= 0 {
		return nil
	}
	var level, current int
	if err := tx.QueryRowContext(ctx, `SELECT level,exp FROM player_characters WHERE id=$1 FOR UPDATE`, characterID).Scan(&level, &current); err != nil {
		return err
	}
	current += exp
	for {
		threshold, ok := levelThresholds[level]
		if !ok || current < threshold {
			break
		}
		level++
		current -= threshold
	}
	_, err := tx.ExecContext(ctx, `UPDATE player_characters SET level=$2,exp=$3,max_exp=$4,updated_at=now() WHERE id=$1`, characterID, level, current, maxExperienceForLevel(level))
	return err
}

func loadHexWornEquipmentTx(ctx context.Context, tx *sql.Tx, characterID int) ([]HexResultItem, error) {
	rows, err := tx.QueryContext(ctx, `SELECT it.name,it.image_url FROM character_equipment ce JOIN item_instances ii ON ii.id=ce.item_instance_id JOIN item_templates it ON it.id=ii.template_id WHERE ce.character_id=$1 AND ii.status='equipped' ORDER BY ce.slot`, characterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []HexResultItem{}
	for rows.Next() {
		var item HexResultItem
		if err := rows.Scan(&item.Name, &item.Image); err != nil {
			return nil, err
		}
		item.Status = "equipped"
		items = append(items, item)
	}
	return items, rows.Err()
}
