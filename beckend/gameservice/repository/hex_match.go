package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"gameservice/game"
	"gameservice/internal/hexgrid"
	"gameservice/internal/hexmap"
	"gameservice/models"
)

var ErrHexUnavailable = errors.New("HEX test match unavailable")
var ErrHexBlocked = errors.New("HEX cell or route is blocked")
var ErrHexEnergy = errors.New("not enough energy")

// CreateHexTestMatch uses the existing account, selected hero and equipment
// calculations. The entire match/map/player/monster materialization is one Tx.
func CreateHexTestMatch(ctx context.Context, userID int, seed int64) (hexmap.Snapshot, error) {
	var empty hexmap.Snapshot
	profile, err := GetPlayerByUserID(userID)
	if err != nil {
		return empty, err
	}
	if profile.SelectedCharacterID == nil {
		return empty, fmt.Errorf("select a hero before creating a HEX test match")
	}
	effective, err := GetCharacterEffectiveStats(*profile.SelectedCharacterID)
	if err != nil {
		return empty, err
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
	resources = hexTestResources(resources)
	if len(resources) == 0 {
		return empty, fmt.Errorf("resource catalog is empty")
	}
	sort.Slice(monsters, func(i, j int) bool { return monsters[i].ID < monsters[j].ID })
	snapshot, err := hexmap.Generate(seed)
	if err != nil {
		return empty, err
	}
	var randomID [16]byte
	if _, err := rand.Read(randomID[:]); err != nil {
		return empty, err
	}
	id := "hex-test-" + hex.EncodeToString(randomID[:])
	snapshot.InstanceID = id
	start := snapshot.Actors[0].Position
	occupied := map[hexgrid.Coord]bool{}
	for _, actor := range snapshot.Actors {
		occupied[actor.Position] = true
	}
	resourceIndex := 0
	for i := range snapshot.Cells {
		cell := &snapshot.Cells[i]
		if cell.Terrain != "floor" || occupied[cell.Position] || resourceIndex >= len(resources) {
			continue
		}
		if cell.Room == 0 {
			continue
		}
		resource := resources[resourceIndex]
		cell.Resource = &resource
		resourceIndex++
	}
	if resourceIndex == 0 {
		return empty, fmt.Errorf("HEX map has no free resource cells")
	}
	startsJSON, _ := json.Marshal([]hexgrid.Coord{start})
	// Stage 1 requires portal_position_v2 for HEX. It remains metadata only;
	// portal activation is outside this experimental movement slice.
	portalJSON, _ := json.Marshal(hexgrid.Coord{})
	mapJSON, _ := json.Marshal(snapshot.Cells)
	geometryJSON, _ := json.Marshal(models.HexV1)
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO matches(instance_id,mode,teams_count,total_players,active_user_id,turn_order,turn_number,map_width,map_height,map,schema_version,geometry,start_positions_v2,portal_position_v2,map_revision,state_revision)
		VALUES($1,'hex-test',1,1,$2,$3,1,17,17,$4,2,$5,$6,$7,1,1)`, id, userID, fmt.Sprintf("[%d]", userID), mapJSON, geometryJSON, startsJSON, portalJSON)
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
		var resourceJSON any
		if cell.Resource != nil {
			resourceJSON, err = json.Marshal(cell.Resource)
			if err != nil {
				return empty, err
			}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO match_map_cells(instance_id,cell_id,q,r,tile_code,is_player,resource) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, cell.CellID, cell.Position.Q, cell.Position.R, tile, cell.Position == start, resourceJSON)
		if err != nil {
			return empty, err
		}
	}
	for _, edge := range snapshot.Edges {
		_, err = tx.ExecContext(ctx, `INSERT INTO match_hex_edges(instance_id,q,r,direction,wall_kind) VALUES($1,$2,$3,$4,$5)`, id, edge.Position.Q, edge.Position.R, edge.Direction, edge.Kind)
		if err != nil {
			return empty, err
		}
	}
	stats := effective.EffectiveStats
	_, err = tx.ExecContext(ctx, `INSERT INTO match_players(instance_id,user_id,name,image,character_id,character_type,position,position_q,position_r,inventory,level,energy,max_energy,health,max_health,experience,max_experience,attack,defense,mobility,agility,sight_range,is_ranged,attack_range,balance,group_id)
		VALUES($1,$2,$3,$4,$5,$6,NULL,$7,$8,$9,$10,$11,$11,$12,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,0)`,
		id, userID, profile.Name, profile.Image, *profile.SelectedCharacterID, profile.CharacterType, start.Q, start.R, profile.Inventory, profile.Level, stats.MaxEnergy, stats.MaxHealth, profile.Experience, profile.MaxExperience, stats.Attack, stats.Defense, stats.Mobility, stats.Agility, stats.SightRange, stats.IsRanged, stats.AttackRange, profile.Balance)
	if err != nil {
		return empty, err
	}
	for i, actor := range snapshot.Actors[1:] {
		monster := monsters[i%len(monsters)]
		_, err = tx.ExecContext(ctx, `INSERT INTO match_monsters(instance_id,monster_ref_id,pos_x,pos_y,pos_q,pos_r,health,max_health,attack,defense,speed,maneuverability,vision,image)
			VALUES($1,$2,NULL,NULL,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, id, monster.ID, actor.Position.Q, actor.Position.R, monster.Health, monster.MaxHealth, monster.Attack, monster.Defense, monster.Speed, monster.Maneuverability, monster.Vision, monster.Image)
		if err != nil {
			return empty, err
		}
	}
	if err := tx.Commit(); err != nil {
		return empty, err
	}
	return LoadHexTestMatch(ctx, id, userID)
}

func LoadHexTestMatch(ctx context.Context, instanceID string, userID int) (hexmap.Snapshot, error) {
	var snapshot hexmap.Snapshot
	tx, err := DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return snapshot, err
	}
	defer tx.Rollback()
	var rawCells []byte
	var geometryRaw []byte
	var mode string
	var version int
	err = tx.QueryRowContext(ctx, `SELECT mode,schema_version,geometry,map,state_revision,map_revision,COALESCE(active_user_id,0),turn_number FROM matches WHERE instance_id=$1 AND finished_at IS NULL`, instanceID).
		Scan(&mode, &version, &geometryRaw, &rawCells, &snapshot.StateRevision, &snapshot.MapRevision, &snapshot.ActiveUser, &snapshot.TurnNumber)
	if err != nil {
		return snapshot, err
	}
	geometry, err := models.ParseGeometry(version, geometryRaw)
	if err != nil || mode != "hex-test" || geometry != models.HexV1 {
		return snapshot, ErrHexUnavailable
	}
	snapshot.SchemaVersion, snapshot.Geometry, snapshot.InstanceID = 2, models.HexV1, instanceID
	if err := json.Unmarshal(rawCells, &snapshot.Cells); err != nil {
		return snapshot, err
	}
	cellIndex := make(map[hexgrid.Coord]int, len(snapshot.Cells))
	for i, cell := range snapshot.Cells {
		cellIndex[cell.Position] = i
	}
	cellRows, err := tx.QueryContext(ctx, `SELECT q,r,resource FROM match_map_cells WHERE instance_id=$1`, instanceID)
	if err != nil {
		return snapshot, err
	}
	for cellRows.Next() {
		var pos hexgrid.Coord
		var rawResource []byte
		if err := cellRows.Scan(&pos.Q, &pos.R, &rawResource); err != nil {
			cellRows.Close()
			return snapshot, err
		}
		if i, ok := cellIndex[pos]; ok {
			snapshot.Cells[i].Resource = nil
			if len(rawResource) > 0 {
				var resource game.ResourceData
				if err := json.Unmarshal(rawResource, &resource); err != nil {
					cellRows.Close()
					return snapshot, err
				}
				snapshot.Cells[i].Resource = &resource
			}
		}
	}
	if err := cellRows.Err(); err != nil {
		cellRows.Close()
		return snapshot, err
	}
	cellRows.Close()
	var player hexmap.Player
	var inventoryRaw []byte
	var maxHealth int
	err = tx.QueryRowContext(ctx, `SELECT position_q,position_r,energy,max_energy,health,max_health,mobility,inventory FROM match_players WHERE instance_id=$1 AND user_id=$2`, instanceID, userID).
		Scan(&player.Position.Q, &player.Position.R, &player.Energy, &player.MaxEnergy, &player.Health, &maxHealth, &player.Mobility, &inventoryRaw)
	if err != nil {
		return snapshot, err
	}
	if len(inventoryRaw) > 0 {
		if err := json.Unmarshal(inventoryRaw, &player.Inventory); err != nil {
			return snapshot, err
		}
	}
	player.UserID = userID
	snapshot.Player = &player
	snapshot.Actors = []hexmap.Actor{{ID: fmt.Sprintf("player-%d", userID), Kind: "player", Position: player.Position, Health: player.Health, MaxHealth: maxHealth}}
	rows, err := tx.QueryContext(ctx, `SELECT monster_instance_id,pos_q,pos_r,health,max_health FROM match_monsters WHERE instance_id=$1 AND health>0 ORDER BY monster_instance_id`, instanceID)
	if err != nil {
		return snapshot, err
	}
	for rows.Next() {
		var id int
		var pos hexgrid.Coord
		var health, maxHealth int
		if err := rows.Scan(&id, &pos.Q, &pos.R, &health, &maxHealth); err != nil {
			rows.Close()
			return snapshot, err
		}
		snapshot.Actors = append(snapshot.Actors, hexmap.Actor{ID: fmt.Sprintf("monster-%d", id), Kind: "monster", Position: pos, Health: health, MaxHealth: maxHealth})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snapshot, err
	}
	rows.Close()
	snapshot.Edges, err = loadHexEdges(ctx, tx, instanceID)
	if err != nil {
		return snapshot, err
	}
	return snapshot, tx.Commit()
}

func hexTestResources(all []game.ResourceData) []game.ResourceData {
	allowed := map[string]bool{"food": true, "water": true, "wood": true, "stone": true, "iron": true}
	filtered := make([]game.ResourceData, 0, 3)
	for _, resource := range all {
		if allowed[resource.Type] {
			filtered = append(filtered, resource)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].ID < filtered[j].ID })
	if len(filtered) > 3 {
		filtered = filtered[:3]
	}
	return filtered
}

type hexQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadHexEdges(ctx context.Context, q hexQuerier, id string) ([]hexmap.Edge, error) {
	rows, err := q.QueryContext(ctx, `SELECT q,r,direction,wall_kind FROM match_hex_edges WHERE instance_id=$1 ORDER BY r,q,direction`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	edges := []hexmap.Edge{}
	for rows.Next() {
		var edge hexmap.Edge
		if err := rows.Scan(&edge.Position.Q, &edge.Position.R, &edge.Direction, &edge.Kind); err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	return edges, rows.Err()
}
