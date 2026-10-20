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
	var randomID [16]byte
	if _, err := rand.Read(randomID[:]); err != nil {
		return hexmap.Snapshot{}, err
	}
	return CreateHexMatch(ctx, HexMatchPlan{
		InstanceID: "hex-test-" + hex.EncodeToString(randomID[:]),
		Mode:       "hex-test", Teams: []hexmap.Team{{ID: 1, PlayerIDs: []int{userID}}},
		TurnOrder: []int{userID}, Seed: seed,
	})
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
	var startsRaw, portalRaw, orderRaw []byte
	var mode string
	var version, width int
	err = tx.QueryRowContext(ctx, `SELECT mode,schema_version,geometry,map,state_revision,map_revision,COALESCE(active_user_id,0),turn_number,map_width,start_positions_v2,portal_position_v2,turn_order FROM matches WHERE instance_id=$1 AND finished_at IS NULL`, instanceID).
		Scan(&mode, &version, &geometryRaw, &rawCells, &snapshot.StateRevision, &snapshot.MapRevision, &snapshot.ActiveUser, &snapshot.TurnNumber, &width, &startsRaw, &portalRaw, &orderRaw)
	if err != nil {
		return snapshot, err
	}
	geometry, err := models.ParseGeometry(version, geometryRaw)
	if err != nil || geometry != models.HexV1 {
		return snapshot, ErrHexUnavailable
	}
	snapshot.SchemaVersion, snapshot.Geometry, snapshot.InstanceID, snapshot.Mode = 2, models.HexV1, instanceID, mode
	if err := json.Unmarshal(orderRaw, &snapshot.TurnOrder); err != nil {
		return snapshot, err
	}
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
			Seed   int64          `json:"seed"`
			Cells  []hexmap.Cell  `json:"cells"`
			Starts []hexmap.Start `json:"starts"`
		}
		if err := json.Unmarshal(rawCells, &saved); err != nil {
			return snapshot, err
		}
		snapshot.Seed, snapshot.Cells, snapshot.Starts = saved.Seed, saved.Cells, saved.Starts
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
	err = tx.QueryRowContext(ctx, `SELECT name,position_q,position_r,energy,max_energy,health,max_health,mobility,agility,character_type,is_ranged,attack_range,inventory,group_id FROM match_players WHERE instance_id=$1 AND user_id=$2`, instanceID, userID).
		Scan(&player.Name, &player.Position.Q, &player.Position.R, &player.Energy, &player.MaxEnergy, &player.Health, &player.MaxHealth, &player.Mobility, &player.Agility, &player.CharacterType, &player.IsRanged, &player.AttackRange, &inventoryRaw, &player.TeamID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return snapshot, ErrHexUnavailable
		}
		return snapshot, err
	}
	if len(inventoryRaw) > 0 {
		if err := json.Unmarshal(inventoryRaw, &player.Inventory); err != nil {
			return snapshot, err
		}
	}
	player.UserID = userID
	snapshot.Player = &player
	if len(snapshot.Starts) == 0 && len(startPositions) > 0 {
		snapshot.Starts = []hexmap.Start{{UserID: userID, TeamID: 1, Position: startPositions[0]}}
	}
	snapshot.Actors = []hexmap.Actor{}
	playerRows, err := tx.QueryContext(ctx, `SELECT user_id,name,group_id,position_q,position_r,health,max_health FROM match_players WHERE instance_id=$1 AND health>0 ORDER BY user_id`, instanceID)
	if err != nil {
		return snapshot, err
	}
	for playerRows.Next() {
		var actor hexmap.Actor
		if err := playerRows.Scan(&actor.UserID, &actor.Name, &actor.TeamID, &actor.Position.Q, &actor.Position.R, &actor.Health, &actor.MaxHealth); err != nil {
			playerRows.Close()
			return snapshot, err
		}
		actor.ID, actor.Kind = fmt.Sprintf("player-%d", actor.UserID), "player"
		snapshot.Actors = append(snapshot.Actors, actor)
	}
	if err := playerRows.Err(); err != nil {
		playerRows.Close()
		return snapshot, err
	}
	playerRows.Close()
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
