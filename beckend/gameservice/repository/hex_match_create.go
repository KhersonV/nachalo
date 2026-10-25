package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
	"gameservice/models"
)

// HexMatchPlan uses the team and turn order already validated by RequestMatch.
type HexMatchPlan struct {
	InstanceID string
	Mode       string
	Teams      []hexmap.Team
	TurnOrder  []int
	Seed       int64
}

// CreateHexMatch materializes one generated map and every participant in one transaction.
func CreateHexMatch(ctx context.Context, plan HexMatchPlan) (hexmap.Snapshot, error) {
	var empty hexmap.Snapshot
	if plan.InstanceID == "" || plan.Mode == "" || len(plan.TurnOrder) == 0 {
		return empty, ErrCommandRejected
	}
	seen := map[int]bool{}
	for _, team := range plan.Teams {
		for _, id := range team.PlayerIDs {
			if id <= 0 || seen[id] {
				return empty, ErrCommandRejected
			}
			seen[id] = true
		}
	}
	if len(seen) != len(plan.TurnOrder) {
		return empty, ErrCommandRejected
	}
	orderSeen := map[int]bool{}
	for _, id := range plan.TurnOrder {
		if !seen[id] || orderSeen[id] {
			return empty, ErrCommandRejected
		}
		orderSeen[id] = true
	}
	monsters, err := GetMonstersData()
	if err != nil {
		return empty, err
	}
	if len(monsters) == 0 {
		return empty, fmt.Errorf("monster catalog is empty")
	}
	resources, err := GetResourcesData()
	if err != nil {
		return empty, err
	}
	snapshot, err := hexmap.GenerateDungeon(hexmap.Config{Teams: plan.Teams, MonsterProb: .05, BarrelProb: .1, ResourceProb: .1}, hexmap.Catalogs{Monsters: monsters, Resources: resources}, plan.Seed)
	if err != nil {
		return empty, err
	}
	if plan.Mode != "hex-test" {
		if err := placeProfessionNodes(ctx, &snapshot, plan.Seed); err != nil {
			return empty, err
		}
	}
	snapshot.InstanceID = plan.InstanceID
	starts := map[int]hexmap.Start{}
	for _, start := range snapshot.Starts {
		starts[start.UserID] = start
	}
	// Resolve the same selected hero and effective equipment stats used by Square.
	type participant struct {
		profile   *models.PlayerResponse
		character *PlayerCharacter
		effective *CharacterEffectiveStats
	}
	participants := map[int]participant{}
	for _, id := range plan.TurnOrder {
		profile, err := GetPlayerByUserID(id)
		if err != nil {
			return empty, fmt.Errorf("player %d: %w", id, err)
		}
		character, err := GetSelectedCharacterForUser(id)
		if err != nil {
			return empty, fmt.Errorf("selected hero for %d: %w", id, err)
		}
		effective, err := GetCharacterEffectiveStats(character.ID)
		if err != nil {
			return empty, fmt.Errorf("effective hero for %d: %w", id, err)
		}
		participants[id] = participant{profile, character, effective}
	}
	orderedStarts := make([]hexgrid.Coord, 0, len(plan.TurnOrder))
	for _, id := range plan.TurnOrder {
		orderedStarts = append(orderedStarts, starts[id].Position)
	}
	startsJSON, err := json.Marshal(orderedStarts)
	if err != nil {
		return empty, err
	}
	portalJSON, err := json.Marshal(snapshot.Portal)
	if err != nil {
		return empty, err
	}
	mapJSON, err := json.Marshal(struct {
		Seed   int64          `json:"seed"`
		Cells  []hexmap.Cell  `json:"cells"`
		Starts []hexmap.Start `json:"starts"`
	}{snapshot.Seed, snapshot.Cells, snapshot.Starts})
	if err != nil {
		return empty, err
	}
	geometryJSON, err := json.Marshal(models.HexV1)
	if err != nil {
		return empty, err
	}
	orderJSON, err := json.Marshal(plan.TurnOrder)
	if err != nil {
		return empty, err
	}
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	// Square selects one catalog artifact per match. Keep that choice inside
	// the creation transaction so a failed match never claims an artifact.
	var questArtifactID int
	if err := tx.QueryRowContext(ctx, `SELECT id FROM artifacts ORDER BY RANDOM() LIMIT 1`).Scan(&questArtifactID); err != nil {
		return empty, fmt.Errorf("select HEX quest artifact: %w", err)
	}
	snapshot.QuestArtifactID = questArtifactID
	loadouts := make(map[int]hexMatchLoadout, len(plan.TurnOrder))
	if plan.Mode == "hex-test" {
		// Test matches must never debit the real account Storage.
		for _, id := range plan.TurnOrder {
			loadouts[id] = hexMatchLoadout{Inventory: map[string]map[string]any{}, Balance: participants[id].profile.Balance}
		}
	} else {
		characterIDs := map[int]int{}
		for id, p := range participants {
			characterIDs[id] = p.character.ID
		}
		loadouts, err = takeHexLoadoutsTx(ctx, tx, plan.TurnOrder, characterIDs)
		if err != nil {
			return empty, fmt.Errorf("take HEX loadouts: %w", err)
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO matches(instance_id,mode,teams_count,total_players,active_user_id,turn_order,turn_number,map_width,map_height,map,schema_version,geometry,start_positions_v2,portal_position_v2,map_revision,state_revision,quest_artifact_id,hex_reward_events_ready,hex_turn_deadline)
		VALUES($1,$2,$3,$4,$5,$6,1,$7,$7,$8,2,$9,$10,$11,1,1,$12,TRUE,now()+$13 * interval '1 second')`, plan.InstanceID, plan.Mode, len(plan.Teams), len(plan.TurnOrder), plan.TurnOrder[0], orderJSON, snapshot.Radius*2+1, mapJSON, geometryJSON, startsJSON, portalJSON, questArtifactID, int(HexTurnDuration().Seconds()))
	if err != nil {
		return empty, err
	}
	for _, cell := range snapshot.Cells {
		tile := 32
		if cell.Terrain == "floor" {
			tile = 48
		}
		if cell.Resource != nil {
			tile = 82
		}
		if cell.Barrel != nil {
			tile = 66
		}
		var resourceJSON, barrelJSON any
		if cell.Resource != nil {
			resourceJSON, err = json.Marshal(cell.Resource)
			if err != nil {
				return empty, err
			}
		}
		if cell.Barrel != nil {
			barrelJSON, err = json.Marshal(cell.Barrel)
			if err != nil {
				return empty, err
			}
		}
		occupied := false
		for _, start := range snapshot.Starts {
			if start.Position == cell.Position {
				occupied = true
				break
			}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO match_map_cells(instance_id,cell_id,q,r,tile_code,is_player,resource,barbel,is_portal) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, plan.InstanceID, cell.CellID, cell.Position.Q, cell.Position.R, tile, occupied, resourceJSON, barrelJSON, cell.IsPortal)
		if err != nil {
			return empty, err
		}
	}
	for _, edge := range snapshot.Edges {
		_, err = tx.ExecContext(ctx, `INSERT INTO match_hex_edges(instance_id,q,r,direction,wall_kind) VALUES($1,$2,$3,$4,$5)`, plan.InstanceID, edge.Position.Q, edge.Position.R, edge.Direction, edge.Kind)
		if err != nil {
			return empty, err
		}
	}
	for _, id := range plan.TurnOrder {
		p := participants[id]
		start := starts[id]
		stats := p.effective.EffectiveStats
		image := p.character.Image
		if image == "" {
			image = ResolveHeroClassMeta(p.character.HeroClassID).Image
		}
		matchInventory, err := json.Marshal(loadouts[id].Inventory)
		if err != nil {
			return empty, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO match_players(instance_id,user_id,name,image,character_id,character_type,position,position_q,position_r,inventory,level,energy,max_energy,health,max_health,experience,max_experience,attack,defense,mobility,agility,sight_range,is_ranged,attack_range,balance,group_id,hex_disconnected_at)
			VALUES($1,$2,$3,$4,$5,$6,NULL,$7,$8,$9,$10,$11,$11,$12,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,now())`, plan.InstanceID, id, p.profile.Name, image, p.character.ID, p.character.HeroClassID, start.Position.Q, start.Position.R, matchInventory, p.character.Level, 0, stats.MaxHealth, p.character.Exp, p.character.MaxExp, stats.Attack, stats.Defense, stats.Mobility, stats.Agility, stats.SightRange, stats.IsRanged, stats.AttackRange, loadouts[id].Balance, start.TeamID)
		if err != nil {
			return empty, err
		}
		// Freeze account Guild access once, inside the participant creation transaction.
		// Building the Guild later never updates this flag in an existing match.
		if _, err = tx.ExecContext(ctx, `UPDATE match_players SET profession_unlocked=($3<>'hex-test' AND EXISTS(SELECT 1 FROM player_base_buildings WHERE user_id=$2 AND guild_level>0)) WHERE instance_id=$1 AND user_id=$2`, plan.InstanceID, id, plan.Mode); err != nil {
			return empty, err
		}

		maxMP := stats.Mobility * 2
		if maxMP < 0 {
			maxMP = 0
		}
		if maxMP > HexMPCap {
			maxMP = HexMPCap
		}
		reactions := 1
		if p.character.HeroClassID == "guardian" {
			reactions = 2
		}
		if _, err = tx.ExecContext(ctx, `UPDATE match_players SET ap=6,max_ap=6,mp=$3,max_mp=$3,reactions=$4,max_reactions=$4 WHERE instance_id=$1 AND user_id=$2`, plan.InstanceID, id, maxMP, reactions); err != nil {
			return empty, err
		}
		for key, entry := range loadouts[id].Inventory {
			parts := strings.SplitN(key, "_", 2)
			if len(parts) != 2 || (parts[0] != "resource" && parts[0] != "scroll") {
				continue // Blueprints stay in JSON until HEX construction is implemented.
			}
			itemID, err := strconv.Atoi(parts[1])
			if err != nil {
				return empty, err
			}
			name, _ := entry["name"].(string)
			if name == "" {
				name, _ = entry["item_name"].(string)
			}
			description, _ := entry["description"].(string)
			imageURL, _ := entry["image"].(string)
			_, err = tx.ExecContext(ctx, `INSERT INTO inventory_items(instance_id,user_id,item_type,item_id,item_name,item_description,item_count,image_url) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, plan.InstanceID, id, parts[0], itemID, name, description, storageCount(entry), imageURL)
			if err != nil {
				return empty, err
			}
		}
	}
	for _, actor := range snapshot.Actors {
		if actor.Kind != "monster" {
			continue
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO match_monsters(instance_id,monster_ref_id,pos_x,pos_y,pos_q,pos_r,health,max_health,attack,defense,speed,maneuverability,vision,image)
			VALUES($1,$2,NULL,NULL,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, plan.InstanceID, actor.MonsterRefID, actor.Position.Q, actor.Position.R, actor.Health, actor.MaxHealth, actor.Attack, actor.Defense, actor.Speed, actor.Maneuverability, actor.Vision, actor.Image)
		if err != nil {
			return empty, err
		}
	}
	initial := &MatchCommand{Tx: tx, InstanceID: plan.InstanceID, ActiveUser: 0, NextPersonalTurn: true}
	if err := initial.SyncHexArtifactsTx(ctx); err != nil {
		return empty, err
	}
	if err := initial.ResolveHexTurnStartTx(ctx); err != nil {
		return empty, err
	}
	for _, id := range plan.TurnOrder {
		if err := recordHexExploredTx(ctx, tx, plan.InstanceID, id, starts[id].Position); err != nil {
			return empty, err
		}
	}
	if err := tx.Commit(); err != nil {
		return empty, err
	}
	loaded, err := LoadHexTestMatch(ctx, plan.InstanceID, plan.TurnOrder[0])
	if err != nil {
		// Commit already moved the loadout. The match exists; reporting creation
		// failure here could make matchmaking retry and strand those items.
		return snapshot, nil
	}
	return loaded, nil
}
