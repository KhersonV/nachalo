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
	var questSpawned bool
	err = tx.QueryRowContext(ctx, `SELECT mode,schema_version,geometry,map,state_revision,map_revision,COALESCE(active_user_id,0),turn_number,map_width,start_positions_v2,portal_position_v2,turn_order,finished_at IS NOT NULL,COALESCE(quest_artifact_id,0),hex_quest_spawned,hex_turn_deadline FROM matches WHERE instance_id=$1`, instanceID).
		Scan(&mode, &version, &geometryRaw, &rawCells, &snapshot.StateRevision, &snapshot.MapRevision, &snapshot.ActiveUser, &snapshot.TurnNumber, &width, &startsRaw, &portalRaw, &orderRaw, &snapshot.Finished, &snapshot.QuestArtifactID, &questSpawned, &snapshot.TurnDeadline)
	if err != nil {
		return snapshot, err
	}
	geometry, err := models.ParseGeometry(version, geometryRaw)
	if err != nil || geometry != models.HexV1 {
		return snapshot, ErrHexUnavailable
	}
	snapshot.SchemaVersion, snapshot.Geometry, snapshot.InstanceID, snapshot.Mode = 2, models.HexV1, instanceID, mode
	snapshot.TurnSeconds = int(HexTurnDuration().Seconds())
	if snapshot.QuestArtifactID > 0 {
		snapshot.QuestArtifact, err = loadHexQuestArtifactTx(ctx, tx, instanceID, snapshot.QuestArtifactID, questSpawned)
		if err != nil {
			return snapshot, err
		}
	}
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
	cellRows, err := tx.QueryContext(ctx, `SELECT q,r,resource,barbel,is_portal,structure_type,structure_owner_user_id,structure_health,structure_defense,structure_attack,is_under_construction,construction_turns_left FROM match_map_cells WHERE instance_id=$1`, instanceID)
	if err != nil {
		return snapshot, err
	}
	for cellRows.Next() {
		var pos hexgrid.Coord
		var rawResource, rawBarrel []byte
		var isPortal bool
		var structureType string
		var owner, health, defense, attack, turns int
		var building bool
		if err := cellRows.Scan(&pos.Q, &pos.R, &rawResource, &rawBarrel, &isPortal, &structureType, &owner, &health, &defense, &attack, &building, &turns); err != nil {
			cellRows.Close()
			return snapshot, err
		}
		if i, ok := cellIndex[pos]; ok {
			snapshot.Cells[i].Resource = nil
			snapshot.Cells[i].Barrel = nil
			snapshot.Cells[i].IsPortal = isPortal
			snapshot.Cells[i].StructureType = structureType
			snapshot.Cells[i].StructureOwnerUserID = owner
			snapshot.Cells[i].StructureHealth = health
			snapshot.Cells[i].StructureDefense = defense
			snapshot.Cells[i].StructureAttack = attack
			snapshot.Cells[i].IsUnderConstruction = building
			snapshot.Cells[i].ConstructionTurnsLeft = turns
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
	err = tx.QueryRowContext(ctx, `SELECT p.name,p.position_q,p.position_r,p.energy,p.max_energy,p.health,p.max_health,p.mobility,p.agility,p.character_type,p.is_ranged,p.attack_range,p.inventory,p.group_id,profile.balance FROM match_players p JOIN player_profiles profile ON profile.user_id=p.user_id WHERE p.instance_id=$1 AND p.user_id=$2`, instanceID, userID).
		Scan(&player.Name, &player.Position.Q, &player.Position.R, &player.Energy, &player.MaxEnergy, &player.Health, &player.MaxHealth, &player.Mobility, &player.Agility, &player.CharacterType, &player.IsRanged, &player.AttackRange, &inventoryRaw, &player.TeamID, &player.Balance)
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
	if snapshot.ActiveUser > 0 {
		if err := tx.QueryRowContext(ctx, `SELECT name FROM match_players WHERE instance_id=$1 AND user_id=$2`, instanceID, snapshot.ActiveUser).Scan(&snapshot.ActivePlayerName); err != nil {
			return snapshot, err
		}
	}
	var resultRaw []byte
	if err := tx.QueryRowContext(ctx, `SELECT result FROM hex_player_results WHERE instance_id=$1 AND user_id=$2`, instanceID, userID).Scan(&resultRaw); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return snapshot, err
	}
	snapshot.Result = json.RawMessage(resultRaw)
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
	if err := filterHexSnapshotTx(ctx, tx, &snapshot); err != nil {
		return snapshot, err
	}
	return snapshot, tx.Commit()
}

func filterHexSnapshotTx(ctx context.Context, tx *sql.Tx, snapshot *hexmap.Snapshot) error {
	if snapshot.Player == nil {
		return ErrHexUnavailable
	}
	// The generator seed is enough to reconstruct the undiscovered dungeon.
	snapshot.Seed = 0
	rows, err := tx.QueryContext(ctx, `SELECT p.position_q,p.position_r,p.hex_explored,COALESCE(s.has_scout_tower_bonus,false) FROM match_players p LEFT JOIN match_player_structures s ON s.instance_id=p.instance_id AND s.user_id=p.user_id WHERE p.instance_id=$1 AND p.group_id=$2 AND p.health>0 ORDER BY p.user_id`, snapshot.InstanceID, snapshot.Player.TeamID)
	if err != nil {
		return err
	}
	positions := []hexgrid.Coord{}
	radii := map[hexgrid.Coord]int{}
	explored := map[hexgrid.Coord]bool{}
	for rows.Next() {
		var pos hexgrid.Coord
		var raw []byte
		var bonus bool
		if err := rows.Scan(&pos.Q, &pos.R, &raw, &bonus); err != nil {
			rows.Close()
			return err
		}
		positions = append(positions, pos)
		radii[pos] = hexSightRadius(bonus)
		var saved []hexgrid.Coord
		if err := json.Unmarshal(raw, &saved); err != nil {
			rows.Close()
			return err
		}
		for _, p := range saved {
			explored[p] = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(positions) == 0 {
		positions = append(positions, snapshot.Player.Position)
		radii[snapshot.Player.Position] = hexVisionRadius()
	}
	visible := hexVisibleWithRadii(positions, radii, snapshot.Cells, snapshot.Edges)
	for pos := range visible {
		explored[pos] = true
	}
	for i := range snapshot.Cells {
		cell := &snapshot.Cells[i]
		switch {
		case visible[cell.Position]:
			cell.Visibility = "visible"
		case explored[cell.Position]:
			cell.Visibility = "explored"
			cell.Resource = nil
			cell.Barrel = nil
			cell.StructureType = ""
			cell.StructureOwnerUserID = 0
			cell.StructureHealth = 0
			cell.StructureDefense = 0
			cell.StructureAttack = 0
			cell.IsUnderConstruction = false
			cell.ConstructionTurnsLeft = 0
		default:
			cell.Visibility = "unexplored"
			cell.Terrain = "rock"
			cell.Resource = nil
			cell.Barrel = nil
			cell.IsPortal = false
			cell.StructureType = ""
			cell.StructureOwnerUserID = 0
			cell.StructureHealth = 0
			cell.StructureDefense = 0
			cell.StructureAttack = 0
			cell.IsUnderConstruction = false
			cell.ConstructionTurnsLeft = 0
			cell.Room = 0
		}
	}
	actors := snapshot.Actors[:0]
	for _, actor := range snapshot.Actors {
		if visible[actor.Position] || actor.Kind == "player" && actor.TeamID == snapshot.Player.TeamID {
			actors = append(actors, actor)
		}
	}
	snapshot.Actors = actors
	edges := snapshot.Edges[:0]
	for _, edge := range snapshot.Edges {
		next, err := hexgrid.Neighbor(edge.Position, edge.Direction)
		if err == nil && explored[edge.Position] && explored[next] {
			edges = append(edges, edge)
		}
	}
	snapshot.Edges = edges
	starts := snapshot.Starts[:0]
	for _, start := range snapshot.Starts {
		if start.TeamID == snapshot.Player.TeamID {
			starts = append(starts, start)
		}
	}
	snapshot.Starts = starts
	if snapshot.Portal != nil && !visible[*snapshot.Portal] {
		snapshot.Portal = nil
	}
	if snapshot.QuestArtifact != nil && snapshot.QuestArtifact.Position != nil && !visible[*snapshot.QuestArtifact.Position] && snapshot.QuestArtifact.Status != "carried" {
		snapshot.QuestArtifact.Position = nil
	}
	return nil
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
