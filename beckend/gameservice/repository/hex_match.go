package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

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
	snapshot, err := hexmap.GenerateDungeon(hexmap.Config{
		Teams:       []hexmap.Team{{ID: 1, PlayerIDs: []int{userID}}},
		MonsterProb: 0.05, BarrelProb: 0.1, ResourceProb: 0.1,
	}, hexmap.Catalogs{Monsters: monsters, Resources: resources}, seed)
	if err != nil {
		return empty, err
	}
	var randomID [16]byte
	if _, err := rand.Read(randomID[:]); err != nil {
		return empty, err
	}
	id := "hex-test-" + hex.EncodeToString(randomID[:])
	snapshot.InstanceID = id
	start := snapshot.Starts[0].Position
	startsJSON, _ := json.Marshal([]hexgrid.Coord{start})
	portalJSON, _ := json.Marshal(snapshot.Portal)
	mapJSON, _ := json.Marshal(struct {
		Seed  int64         `json:"seed"`
		Cells []hexmap.Cell `json:"cells"`
	}{snapshot.Seed, snapshot.Cells})
	geometryJSON, _ := json.Marshal(models.HexV1)
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO matches(instance_id,mode,teams_count,total_players,active_user_id,turn_order,turn_number,map_width,map_height,map,schema_version,geometry,start_positions_v2,portal_position_v2,map_revision,state_revision)
		VALUES($1,'hex-test',1,1,$2,$3,1,$4,$4,$5,2,$6,$7,$8,1,1)`, id, userID, fmt.Sprintf("[%d]", userID), snapshot.Radius*2+1, mapJSON, geometryJSON, startsJSON, portalJSON)
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
		_, err = tx.ExecContext(ctx, `INSERT INTO match_map_cells(instance_id,cell_id,q,r,tile_code,is_player,resource,barbel,is_portal) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, cell.CellID, cell.Position.Q, cell.Position.R, tile, cell.Position == start, resourceJSON, barrelJSON, cell.IsPortal)
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
	for _, actor := range snapshot.Actors[1:] {
		if actor.Kind != "monster" {
			continue
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO match_monsters(instance_id,monster_ref_id,pos_x,pos_y,pos_q,pos_r,health,max_health,attack,defense,speed,maneuverability,vision,image)
			VALUES($1,$2,NULL,NULL,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, id, actor.MonsterRefID, actor.Position.Q, actor.Position.R, actor.Health, actor.MaxHealth, actor.Attack, actor.Defense, actor.Speed, actor.Maneuverability, actor.Vision, actor.Image)
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
	var startsRaw, portalRaw []byte
	var mode string
	var version, width int
	err = tx.QueryRowContext(ctx, `SELECT mode,schema_version,geometry,map,state_revision,map_revision,COALESCE(active_user_id,0),turn_number,map_width,start_positions_v2,portal_position_v2 FROM matches WHERE instance_id=$1 AND finished_at IS NULL`, instanceID).
		Scan(&mode, &version, &geometryRaw, &rawCells, &snapshot.StateRevision, &snapshot.MapRevision, &snapshot.ActiveUser, &snapshot.TurnNumber, &width, &startsRaw, &portalRaw)
	if err != nil {
		return snapshot, err
	}
	geometry, err := models.ParseGeometry(version, geometryRaw)
	if err != nil || mode != "hex-test" || geometry != models.HexV1 {
		return snapshot, ErrHexUnavailable
	}
	snapshot.SchemaVersion, snapshot.Geometry, snapshot.InstanceID = 2, models.HexV1, instanceID
	snapshot.Radius = (width - 1) / 2
	if err := json.Unmarshal(portalRaw, &snapshot.Portal); err != nil {
		return snapshot, err
	}
	var startPositions []hexgrid.Coord
	if err := json.Unmarshal(startsRaw, &startPositions); err != nil {
		return snapshot, err
	}
	if len(rawCells) > 0 && rawCells[0] == '[' {
		if err := json.Unmarshal(rawCells, &snapshot.Cells); err != nil {
			return snapshot, err
		}
	} else {
		var saved struct {
			Seed  int64         `json:"seed"`
			Cells []hexmap.Cell `json:"cells"`
		}
		if err := json.Unmarshal(rawCells, &saved); err != nil {
			return snapshot, err
		}
		snapshot.Seed, snapshot.Cells = saved.Seed, saved.Cells
	}
	cellIndex := make(map[hexgrid.Coord]int, len(snapshot.Cells))
	for i, cell := range snapshot.Cells {
		cellIndex[cell.Position] = i
	}
	cellRows, err := tx.QueryContext(ctx, `SELECT q,r,resource,barbel,is_portal FROM match_map_cells WHERE instance_id=$1`, instanceID)
	if err != nil {
		return snapshot, err
	}
	for cellRows.Next() {
		var pos hexgrid.Coord
		var rawResource, rawBarrel []byte
		var isPortal bool
		if err := cellRows.Scan(&pos.Q, &pos.R, &rawResource, &rawBarrel, &isPortal); err != nil {
			cellRows.Close()
			return snapshot, err
		}
		if i, ok := cellIndex[pos]; ok {
			snapshot.Cells[i].Resource = nil
			snapshot.Cells[i].Barrel = nil
			snapshot.Cells[i].IsPortal = isPortal
			if len(rawResource) > 0 {
				var resource game.ResourceData
				if err := json.Unmarshal(rawResource, &resource); err != nil {
					cellRows.Close()
					return snapshot, err
				}
				snapshot.Cells[i].Resource = &resource
			}
			if len(rawBarrel) > 0 {
				var barrel game.ResourceData
				if err := json.Unmarshal(rawBarrel, &barrel); err != nil {
					cellRows.Close()
					return snapshot, err
				}
				snapshot.Cells[i].Barrel = &barrel
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
	err = tx.QueryRowContext(ctx, `SELECT name,position_q,position_r,energy,max_energy,health,max_health,mobility,agility,character_type,is_ranged,attack_range,inventory FROM match_players WHERE instance_id=$1 AND user_id=$2`, instanceID, userID).
		Scan(&player.Name, &player.Position.Q, &player.Position.R, &player.Energy, &player.MaxEnergy, &player.Health, &player.MaxHealth, &player.Mobility, &player.Agility, &player.CharacterType, &player.IsRanged, &player.AttackRange, &inventoryRaw)
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
	if len(startPositions) > 0 {
		snapshot.Starts = []hexmap.Start{{UserID: userID, TeamID: 1, Position: startPositions[0]}}
	}
	snapshot.Actors = []hexmap.Actor{{ID: fmt.Sprintf("player-%d", userID), Kind: "player", Position: player.Position, Health: player.Health, MaxHealth: player.MaxHealth}}
	rows, err := tx.QueryContext(ctx, `SELECT mm.monster_instance_id,mm.monster_ref_id,mm.pos_q,mm.pos_r,mm.health,mm.max_health,mm.attack,mm.defense,mm.speed,mm.maneuverability,mm.vision,mm.image,COALESCE(m.name,''),COALESCE(m.type,'') FROM match_monsters mm LEFT JOIN monsters m ON m.id=mm.monster_ref_id WHERE mm.instance_id=$1 AND mm.health>0 ORDER BY mm.monster_instance_id`, instanceID)
	if err != nil {
		return snapshot, err
	}
	for rows.Next() {
		var actor hexmap.Actor
		var id int
		if err := rows.Scan(&id, &actor.MonsterRefID, &actor.Position.Q, &actor.Position.R, &actor.Health, &actor.MaxHealth, &actor.Attack, &actor.Defense, &actor.Speed, &actor.Maneuverability, &actor.Vision, &actor.Image, &actor.Name, &actor.Type); err != nil {
			rows.Close()
			return snapshot, err
		}
		actor.ID, actor.Kind = fmt.Sprintf("monster-%d", id), "monster"
		snapshot.Actors = append(snapshot.Actors, actor)
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
