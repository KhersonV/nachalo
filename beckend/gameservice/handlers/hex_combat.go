package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"gameservice/game"
	"gameservice/internal/game/balance"
	"gameservice/internal/hexgrid"
	"gameservice/repository"

	"github.com/gorilla/mux"
)

var errHexCombatUnsupported = errors.New("this HEX class effect is not transactional yet")

type hexCombatActor struct {
	ID               int
	Position         hexgrid.Coord
	Stats            stats
	AP               int
	MaxAP            int
	Agility          int
	ArmorBreakStacks int
	ArmorBreakOwner  int
	Reactions        int
	TeamID           int
	CharacterType    string
}

type hexAttackState struct {
	Player, Monster hexCombatActor
	Mode            attackMode
	Distance, Cost  int
}

type HexAttackPreview struct {
	Route                 *repository.HexRouteResult `json:"route,omitempty"`
	AttackPosition        hexgrid.Coord              `json:"attackPosition"`
	PlannedTargetPosition hexgrid.Coord              `json:"plannedTargetPosition"`
	CanAttack             bool                       `json:"canAttack"`
	Reason                string                     `json:"reason,omitempty"`
	PreviewToken          string                     `json:"previewToken,omitempty"`
	PreviewRevision       int64                      `json:"previewRevision"`
	PreviewMapRevision    int64                      `json:"previewMapRevision"`

	TargetID int        `json:"targetId"`
	Distance int        `json:"distance"`
	Cost     int        `json:"cost"`
	Mode     attackMode `json:"mode"`
	AP       int        `json:"ap"`
	TargetHP int        `json:"targetHP"`
}

type HexAttackResult struct {
	Budget        *repository.HexBudgets `json:"budget,omitempty"`
	ActionEffects []map[string]any       `json:"actionEffects,omitempty"`
	HexAttackPreview
	Damage           int                    `json:"damage"`
	CounterDamage    int                    `json:"counterDamage"`
	NewTargetHP      int                    `json:"newTargetHP"`
	NewAttackerHP    int                    `json:"newAttackerHP"`
	MonsterDied      bool                   `json:"monsterDied"`
	PlayerDied       bool                   `json:"playerDied,omitempty"`
	AttackStyle      AttackStyle            `json:"attackStyle"`
	Effects          []CombatEffect         `json:"effects"`
	Steps            []CombatStep           `json:"steps"`
	Drops            []EquipmentDropPayload `json:"drops"`
	StateRevision    int64                  `json:"stateRevision"`
	MapRevision      int64                  `json:"mapRevision"`
	Replayed         bool                   `json:"-"`
	AttackerPosition hexgrid.Coord          `json:"attackerPosition"`
	TargetPosition   hexgrid.Coord          `json:"targetPosition"`
	AttackerHPBefore int                    `json:"attackerHPBefore"`
}

func (r *HexAttackResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

// readHexAttackTx is the geometry adapter. All damage/class formulas remain
// the shared combat formulas. Preview runs it read-only; the
// command calls it again after locking the matches row.
func readHexAttackTx(ctx context.Context, tx *sql.Tx, instanceID string, userID, targetID int, requireAP bool, targetKind ...string) (hexAttackState, error) {
	return readHexAttackFromTx(ctx, tx, instanceID, userID, targetID, requireAP, nil, targetKind...)
}

func readHexAttackFromTx(ctx context.Context, tx *sql.Tx, instanceID string, userID, targetID int, requireAP bool, origin *hexgrid.Coord, targetKind ...string) (hexAttackState, error) {
	var out hexAttackState
	if targetID <= 0 {
		return out, repository.ErrCommandRejected
	}
	var mode string
	var version, active int
	if err := tx.QueryRowContext(ctx, `SELECT geometry->>'kind',schema_version,COALESCE(active_user_id,0) FROM matches WHERE instance_id=$1 AND finished_at IS NULL`, instanceID).Scan(&mode, &version, &active); err != nil {
		return out, err
	}
	if mode != "hex" || version != 2 {
		return out, repository.ErrHexUnavailable
	}
	if active != userID {
		return out, repository.ErrCommandRejected
	}
	p := &out.Player
	p.ID = userID
	err := tx.QueryRowContext(ctx, `SELECT position_q,position_r,ap,max_ap,health,max_health,attack,defense,agility,is_ranged,attack_range,character_type,group_id,hex_armor_break_stacks,hex_armor_break_owner,reactions FROM match_players WHERE instance_id=$1 AND user_id=$2`, instanceID, userID).Scan(&p.Position.Q, &p.Position.R, &p.AP, &p.MaxAP, &p.Stats.Health, &p.Stats.MaxHealth, &p.Stats.Attack, &p.Stats.Defense, &p.Stats.Agility, &p.Stats.IsRanged, &p.Stats.AttackRange, &p.Stats.CharacterType, &p.TeamID, &p.ArmorBreakStacks, &p.ArmorBreakOwner, &p.Reactions)
	if errors.Is(err, sql.ErrNoRows) {
		return out, repository.ErrCommandRejected
	}
	if err != nil {
		return out, err
	}
	m := &out.Monster
	m.ID = targetID
	if len(targetKind) > 0 && targetKind[0] == "player" {
		if targetID == userID {
			return out, repository.ErrCommandRejected
		}
		err = tx.QueryRowContext(ctx, `SELECT position_q,position_r,ap,health,max_health,attack,defense,agility,is_ranged,attack_range,character_type,group_id,hex_armor_break_stacks,hex_armor_break_owner,reactions FROM match_players WHERE instance_id=$1 AND user_id=$2`, instanceID, targetID).Scan(&m.Position.Q, &m.Position.R, &m.AP, &m.Stats.Health, &m.Stats.MaxHealth, &m.Stats.Attack, &m.Stats.Defense, &m.Stats.Agility, &m.Stats.IsRanged, &m.Stats.AttackRange, &m.Stats.CharacterType, &m.TeamID, &m.ArmorBreakStacks, &m.ArmorBreakOwner, &m.Reactions)
		if err == nil && m.TeamID == p.TeamID {
			return out, repository.ErrCommandRejected
		}
	} else {
		err = tx.QueryRowContext(ctx, `SELECT pos_q,pos_r,health,max_health,attack,defense,hex_armor_break_stacks,hex_armor_break_owner FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2`, instanceID, targetID).Scan(&m.Position.Q, &m.Position.R, &m.Stats.Health, &m.Stats.MaxHealth, &m.Stats.Attack, &m.Stats.Defense, &m.ArmorBreakStacks, &m.ArmorBreakOwner)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return out, repository.ErrHexBlocked
	}
	if err != nil {
		return out, err
	}
	if p.Stats.Health <= 0 || m.Stats.Health <= 0 {
		return out, repository.ErrCommandRejected
	}
	var visible bool
	if origin != nil {
		p.Position = *origin
		cells, e := repository.HexVisibleCellsFromTx(ctx, tx, instanceID, userID, *origin)
		err = e
		visible = cells[m.Position]
	} else {
		visible, err = repository.HexPositionVisibleTx(ctx, tx, instanceID, userID, m.Position)
	}
	if err != nil {
		return out, err
	}
	if !visible {
		return out, repository.ErrHexBlocked
	}
	distance, err := hexgrid.Distance(p.Position, m.Position)
	if err != nil {
		return out, err
	}
	out.Distance = distance
	switch {
	case distance == 1:
		out.Mode = attackModeMelee
	case distance > 1 && p.Stats.IsRanged && distance <= p.Stats.AttackRange:
		out.Mode = attackModeRanged
	default:
		return out, repository.ErrHexBlocked
	}
	if p.Stats.CharacterType != "guardian" && p.Stats.CharacterType != "mystic" && p.Stats.CharacterType != "berserker" && p.Stats.CharacterType != "ranger" {
		return out, errHexCombatUnsupported
	}
	out.Cost = resolveAttackEnergyCost(out.Mode)
	if requireAP && p.AP < out.Cost {
		return out, repository.ErrHexEnergy
	}
	rows, err := tx.QueryContext(ctx, `SELECT q,r,tile_code FROM match_map_cells WHERE instance_id=$1`, instanceID)
	if err != nil {
		return out, err
	}
	walkable := map[hexgrid.Coord]bool{}
	for rows.Next() {
		var pos hexgrid.Coord
		var tile int
		if err := rows.Scan(&pos.Q, &pos.R, &tile); err != nil {
			rows.Close()
			return out, err
		}
		walkable[pos] = tile == 48 || tile == 82 || tile == 66 && (pos == p.Position || pos == m.Position)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	rows, err = tx.QueryContext(ctx, `SELECT q,r,direction FROM match_hex_edges WHERE instance_id=$1`, instanceID)
	if err != nil {
		return out, err
	}
	walls := map[hexgrid.EdgeKey]bool{}
	for rows.Next() {
		var pos hexgrid.Coord
		var direction int
		if err := rows.Scan(&pos.Q, &pos.R, &direction); err != nil {
			rows.Close()
			return out, err
		}
		next, err := hexgrid.Neighbor(pos, direction)
		if err != nil {
			rows.Close()
			return out, err
		}
		key, err := hexgrid.CanonicalEdge(pos, next)
		if err != nil {
			rows.Close()
			return out, err
		}
		walls[key] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	if origin != nil {
		// A plan cannot infer obstacle geometry outside the currently visible area.
		current, e := repository.HexVisibleCellsTx(ctx, tx, instanceID, userID)
		if e != nil {
			return out, e
		}
		for pos := range walkable {
			if !current[pos] {
				walkable[pos] = false
			}
		}
	}
	if !hexgrid.Visible(p.Position, m.Position, walkable, walls) {
		return out, repository.ErrHexBlocked
	}
	return out, nil
}

func PreviewHexAttack(ctx context.Context, instanceID string, userID, targetID int) (HexAttackPreview, error) {
	var out HexAttackPreview
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := repository.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	state, err := readHexAttackTx(ctx, tx, instanceID, userID, targetID, true)
	if err != nil {
		return out, err
	}
	if err := tx.Commit(); err != nil {
		return out, err
	}
	return HexAttackPreview{TargetID: targetID, Distance: state.Distance, Cost: state.Cost, Mode: state.Mode, AP: state.Player.AP, TargetHP: state.Monster.Stats.Health}, nil
}

// A push follows the closest axial direction to the attack ray. The match row
// lock held by the caller protects all occupancy reads and writes.
func pushHexMonsterTx(ctx context.Context, c *repository.MatchCommand, from, target hexgrid.Coord, monsterID int) (bool, hexgrid.Coord, error) {
	a, _ := hexgrid.ToWorld(from, 1)
	b, _ := hexgrid.ToWorld(target, 1)
	best, score := 0, math.Inf(-1)
	for i, direction := range hexgrid.Directions {
		v, _ := hexgrid.ToWorld(direction, 1)
		if dot := (b.X-a.X)*v.X + (b.Z-a.Z)*v.Z; dot > score {
			best, score = i, dot
		}
	}
	destination, err := hexgrid.Neighbor(target, best)
	if err != nil {
		return false, hexgrid.Coord{}, nil
	}
	var tile int
	var player, portal, construction bool
	var structure string
	var resource, barbel, cellMonster []byte
	err = c.Tx.QueryRowContext(ctx, `SELECT tile_code,is_player,is_portal,is_under_construction,structure_type,resource,barbel,monster FROM match_map_cells WHERE instance_id=$1 AND q=$2 AND r=$3`, c.InstanceID, destination.Q, destination.R).Scan(&tile, &player, &portal, &construction, &structure, &resource, &barbel, &cellMonster)
	if errors.Is(err, sql.ErrNoRows) {
		return false, destination, nil
	}
	if err != nil {
		return false, destination, err
	}
	if (tile != 48 && tile != 82) || player || portal || construction || structure != "" || len(resource) > 0 || len(barbel) > 0 || len(cellMonster) > 0 {
		return false, destination, nil
	}
	var occupied int
	if err := c.Tx.QueryRowContext(ctx, `SELECT count(*) FROM match_monsters WHERE instance_id=$1 AND pos_q=$2 AND pos_r=$3 AND health>0`, c.InstanceID, destination.Q, destination.R).Scan(&occupied); err != nil {
		return false, destination, err
	}
	if occupied != 0 {
		return false, destination, nil
	}
	key, _ := hexgrid.CanonicalEdge(target, destination)
	rows, err := c.Tx.QueryContext(ctx, `SELECT q,r,direction FROM match_hex_edges WHERE instance_id=$1`, c.InstanceID)
	if err != nil {
		return false, destination, err
	}
	blocked := false
	for rows.Next() {
		var pos hexgrid.Coord
		var direction int
		if err = rows.Scan(&pos.Q, &pos.R, &direction); err != nil {
			break
		}
		next, edgeErr := hexgrid.Neighbor(pos, direction)
		if edgeErr != nil {
			err = edgeErr
			break
		}
		edge, _ := hexgrid.CanonicalEdge(pos, next)
		blocked = blocked || edge == key
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil || blocked {
		return false, destination, err
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_monsters SET pos_q=$3,pos_r=$4 WHERE instance_id=$1 AND monster_instance_id=$2 AND pos_q=$5 AND pos_r=$6 AND health>0`, c.InstanceID, monsterID, destination.Q, destination.R, target.Q, target.R)
	if err != nil {
		return false, destination, err
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return false, destination, err
	}
	for _, update := range []struct {
		pos   hexgrid.Coord
		moved bool
	}{{target, false}, {destination, true}} {
		if update.moved {
			result, err = c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET monster=(SELECT jsonb_build_object('db_instance_id',monster_instance_id,'health',health) FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2) WHERE instance_id=$1 AND q=$3 AND r=$4`, c.InstanceID, monsterID, update.pos.Q, update.pos.R)
		} else {
			result, err = c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET monster=NULL WHERE instance_id=$1 AND q=$2 AND r=$3`, c.InstanceID, update.pos.Q, update.pos.R)
		}
		if err != nil {
			return false, destination, err
		}
		if count, _ := result.RowsAffected(); count != 1 {
			return false, destination, repository.ErrCommandRejected
		}
	}
	return true, destination, nil
}

func ApplyHexAttack(ctx context.Context, instanceID string, userID, targetID int, commandID string, publish func(HexAttackResult)) (HexAttackResult, error) {
	var out HexAttackResult
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c, err := repository.BeginMatchCommand(ctx, instanceID, userID, commandID, fmt.Sprintf("hex-pve-v1:%d:%d", userID, targetID))
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
	out, err = applyHexAttackTx(ctx, c, userID, targetID)
	if err != nil {
		return out, err
	}
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, c.MapChanged)
	if err != nil {
		return HexAttackResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}

// Uses the caller's locked MatchCommand; no nested transaction or second commit.
func applyHexAttackTx(ctx context.Context, c *repository.MatchCommand, userID, targetID int) (HexAttackResult, error) {
	var out HexAttackResult
	instanceID := c.InstanceID
	state, err := readHexAttackTx(ctx, c.Tx, instanceID, userID, targetID, true)
	if err != nil {
		return out, err
	}
	if err := c.PrepareHexMainTargetTx(ctx, userID, "monster", targetID); err != nil {
		return out, err
	}
	keepKind, keepID := "", 0
	if state.Player.Stats.CharacterType == "ranger" && state.Mode == attackModeRanged {
		keepKind, keepID = "monster", targetID
	}
	if err := c.ClearRangerBreakTx(ctx, userID, keepKind, keepID); err != nil {
		return out, err
	}
	if state.Monster.ArmorBreakOwner != userID {
		state.Monster.ArmorBreakStacks = 0
	}
	a, d := state.Player.Stats, state.Monster.Stats
	effectiveMonster := d
	if a.CharacterType == "ranger" && state.Mode == attackModeRanged && state.Monster.ArmorBreakOwner == userID {
		effectiveMonster.Defense -= state.Monster.ArmorBreakStacks * monsterArmorBreakPenaltyPerStack
	}
	if effectiveMonster.Defense < 0 {
		effectiveMonster.Defense = 0
	}
	if a.CharacterType == "mystic" {
		effectiveMonster.Defense = effectiveMonster.Defense * mysticMonsterDefensePercent / 100
	}
	hit := applyDamage(a, effectiveMonster)
	rangerChance := 0
	if a.CharacterType == "ranger" {
		rangerChance = balance.CalculateReflexProcChance(a.Agility)
	}
	var criticalShot bool
	hit.Damage, criticalShot, err = c.CraftCriticalTx(ctx, userID, hit.Damage, rangerChance, rollReflexProc)
	if err != nil {
		return out, err
	}
	hit.NewHealth = d.Health - hit.Damage
	if hit.NewHealth < 0 {
		hit.NewHealth = 0
	}
	mysticExtra, mysticReflex := 0, 0
	if a.CharacterType == "mystic" && hit.Triggered {
		mysticExtra, mysticReflex, err = c.MysticHitTx(ctx, userID, "monster", targetID, rollReflexProc(balance.CalculateReflexProcChance(a.Agility)))
		if err != nil {
			return out, err
		}
	}
	primaryHP := hit.NewHealth
	hit.NewHealth -= mysticExtra + mysticReflex
	if hit.NewHealth < 0 {
		hit.NewHealth = 0
	}
	counterDamage := 0
	if state.Mode == attackModeMelee && hit.NewHealth > 0 {
		counterDamage = applyDamage(d, a).Damage
		if a.CharacterType == "berserker" && counterDamage > 0 {
			counterDamage = counterDamage * berserkerMonsterCounterPercent / 100
			if counterDamage < 1 {
				counterDamage = 1
			}
		}
	}
	attackerRef := CombatTargetRef{ID: userID, Type: CombatActorPlayer}
	targetRef := CombatTargetRef{ID: targetID, Type: CombatActorMonster}
	steps := []CombatStep{{Kind: "hit", Source: &attackerRef, Target: targetRef, Damage: hit.Damage, TargetHPAfter: primaryHP}}
	effects := []CombatEffect{}
	if mysticExtra+mysticReflex > 0 {
		steps = append(steps, CombatStep{Kind: "mysticOverload", Source: &attackerRef, Target: targetRef, Damage: mysticExtra + mysticReflex, TargetHPAfter: hit.NewHealth})
		effects = append(effects, CombatEffect{Kind: "mysticOverload", Source: &attackerRef, Target: &targetRef, Amount: mysticExtra, BonusDamage: mysticReflex, Succeeded: true})
	}

	if criticalShot {
		effects = append(effects, CombatEffect{Kind: "crit", Source: &attackerRef, Target: &targetRef, Succeeded: true})
	}
	heal, err := c.CraftLifestealTx(ctx, userID, actualDamageDealt(d.Health, primaryHP), a.CharacterType == "berserker" && hit.Damage > 0 && rollReflexProc(balance.CalculateReflexProcChance(a.Agility)), rollReflexProc)
	if err != nil {
		return out, err
	}
	if heal > 0 {
		if missing := a.MaxHealth - a.Health; heal > missing {
			heal = missing
		}
		if heal > 0 {
			a.Health += heal
			effects = append(effects, CombatEffect{Kind: "lifesteal", Source: &attackerRef, Target: &attackerRef, Amount: heal, Succeeded: true})
		}
	}
	finalMonsterHP := hit.NewHealth
	counter := attackResult{NewHealth: a.Health}
	if state.Mode == attackModeMelee && finalMonsterHP > 0 {
		counter = attackResult{Damage: counterDamage, NewHealth: a.Health - counterDamage, Triggered: true}
		steps = append(steps, CombatStep{Kind: "counter", Source: &targetRef, Target: attackerRef, Damage: counter.Damage, TargetHPAfter: counter.NewHealth})
	}
	playerDied := counter.Triggered && counter.NewHealth <= 0
	guardianResponse := attackResult{}
	if counter.Triggered && counter.NewHealth > 0 && a.CharacterType == "guardian" {
		responseDamage := a.Defense - effectiveMonster.Defense/2
		if responseDamage < 1 {
			responseDamage = 1
		}
		guardianResponse = applyFlatDamage(finalMonsterHP, responseDamage)
		finalMonsterHP = guardianResponse.NewHealth
		steps = append(steps, CombatStep{Kind: "guardianResponse", Source: &attackerRef, Target: targetRef, Damage: guardianResponse.Damage, TargetHPAfter: finalMonsterHP})
	}
	followupDamage := 0
	if a.CharacterType == "berserker" && state.Mode == attackModeMelee && counter.Triggered && hit.Triggered && finalMonsterHP > 0 && counter.NewHealth > 0 && berserkerFollowUpLimitPerTurn == 0 {
		followupDamage = hit.Damage / 2
		if followupDamage > 0 {
			followup := applyFlatDamage(finalMonsterHP, followupDamage)
			finalMonsterHP = followup.NewHealth
			steps = append(steps, CombatStep{Kind: "followup", Source: &attackerRef, Target: targetRef, Damage: followup.Damage, TargetHPAfter: finalMonsterHP})
		}
	}
	ap := state.Player.AP - state.Cost
	mapChanged := false
	fallbackDamage := 0
	if a.CharacterType == "ranger" && state.Mode == attackModeRanged && hit.Triggered && finalMonsterHP > 0 {
		if state.Monster.ArmorBreakStacks >= armorBreakMaxStacks {
			granted := 2 // +1 MP in half points, capped by actual available stock.
			b, err := repository.ReadHexBudgetsTx(ctx, c.Tx, instanceID, userID)
			if err != nil {
				return out, err
			}
			if granted > repository.HexMPCap-b.MP {
				granted = repository.HexMPCap - b.MP
			}
			if err := c.GrantHexMPTx(ctx, userID, granted); err != nil {
				return out, err
			}
			pushed, destination, err := pushHexMonsterTx(ctx, c, state.Player.Position, state.Monster.Position, targetID)
			if err != nil {
				return out, err
			}
			if pushed {
				mapChanged = true
				position := CombatPoint{X: destination.Q, Y: destination.R}
				effects = append(effects, CombatEffect{Kind: "push", Source: &attackerRef, Target: &targetRef, Succeeded: true, PositionAfter: &position, MPGranted: granted})
			} else {
				bonusTarget := d
				bonusTarget.Health = finalMonsterHP
				bonusTarget.Defense = effectiveMonster.Defense
				bonus := applyDamage(a, bonusTarget)
				fallbackDamage = bonus.Damage / 2
				if bonus.Damage > 0 && fallbackDamage < 1 {
					fallbackDamage = 1
				}
				finalMonsterHP -= fallbackDamage
				if finalMonsterHP < 0 {
					finalMonsterHP = 0
				}
				steps = append(steps, CombatStep{Kind: "bonus", Source: &attackerRef, Target: targetRef, Damage: fallbackDamage, TargetHPAfter: finalMonsterHP})
				effects = append(effects, CombatEffect{Kind: "push", Source: &attackerRef, Target: &targetRef, Succeeded: false, BonusDamage: fallbackDamage, MPGranted: granted})
			}
			if err := c.SetRangerBreakTx(ctx, "monster", targetID, userID, 0); err != nil {
				return out, err
			}
			endEffect := CombatEffect{Kind: "armorBreakEnd", Target: &targetRef, Succeeded: true}
			if pushed {
				endEffect.PositionAfter = &CombatPoint{X: destination.Q, Y: destination.R}
			}
			effects = append(effects, endEffect)
		} else {
			stacks := state.Monster.ArmorBreakStacks + 1
			if stacks > armorBreakMaxStacks {
				stacks = armorBreakMaxStacks
			}
			if err := c.SetRangerBreakTx(ctx, "monster", targetID, userID, stacks); err != nil {
				return out, err
			}
			effects = append(effects, CombatEffect{Kind: "armorBreak", Source: &attackerRef, Target: &targetRef, Value: -monsterArmorBreakPenaltyPerStack, Stacks: stacks, Succeeded: true})
		}
	}
	if !playerDied && (ap != state.Player.AP || counter.NewHealth != state.Player.Stats.Health) {
		if err := c.SetHexCombatVitalsTx(ctx, userID, counter.NewHealth, ap); err != nil {
			return out, err
		}
	}
	if playerDied {
		if err := c.SetHexCombatVitalsTx(ctx, userID, state.Player.Stats.Health, ap); err != nil {
			return out, err
		}
	}
	monsterDied := finalMonsterHP <= 0
	if monsterDied {
		if err := c.GrantProfessionMonsterLootTx(ctx, userID, targetID); err != nil {
			return out, err
		}
		result, err := c.Tx.ExecContext(ctx, `DELETE FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2 AND health>0`, instanceID, targetID)
		if err != nil {
			return out, err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return out, repository.ErrCommandRejected
		}
		steps = append(steps, CombatStep{Kind: "death", Target: targetRef})
		mapChanged = true
		if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET monster=NULL WHERE instance_id=$1 AND q=$2 AND r=$3`, instanceID, state.Monster.Position.Q, state.Monster.Position.R); err != nil {
			return out, err
		}
	} else {
		if err := c.SetMonsterHealthTx(ctx, targetID, finalMonsterHP); err != nil {
			return out, err
		}
		if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET monster=jsonb_set(monster,'{health}',to_jsonb($3::int)) WHERE instance_id=$1 AND monster->>'db_instance_id'=$2::text`, instanceID, targetID, finalMonsterHP); err != nil {
			return out, err
		}
	}
	var drops []EquipmentDropPayload
	if monsterDied {
		drops, err = grantHexMonsterEquipmentDropTx(c.Tx, userID, instanceID)
		if err != nil {
			return out, err
		}
	}
	for _, event := range hexDamageEventsFromSteps(steps, effects, map[CombatTargetRef]int{attackerRef: state.Player.Stats.Health, targetRef: state.Monster.Stats.Health}) {
		if err := c.RecordDamageEventTx(ctx, event); err != nil {
			return out, err
		}
	}
	if monsterDied {
		killDamage := hit.Damage
		if followupDamage > 0 {
			killDamage = followupDamage
		} else if fallbackDamage > 0 {
			killDamage = fallbackDamage
		} else if guardianResponse.Damage > 0 {
			killDamage = guardianResponse.Damage
		}
		if _, err := c.RecordKillEventTx(ctx, game.KillEvent{SourceType: game.ResultSourcePlayer, KillerID: userID, VictimType: "monster", VictimID: targetID, Damage: killDamage}); err != nil {
			return out, err
		}
	}
	if playerDied {
		steps = append(steps, CombatStep{Kind: "death", Target: attackerRef})
		if err := finishHexPlayerDeathTx(ctx, c, userID, targetID, counter.Damage, state.Player.Position); err != nil {
			return out, err
		}
		mapChanged = true
	}
	out = HexAttackResult{HexAttackPreview: HexAttackPreview{TargetID: targetID, Distance: state.Distance, Cost: state.Cost, Mode: state.Mode, AP: ap, TargetHP: d.Health}, Damage: hit.Damage, CounterDamage: counter.Damage, NewTargetHP: finalMonsterHP, NewAttackerHP: counter.NewHealth, MonsterDied: monsterDied, AttackStyle: resolvePresentationAttackStyle(a, state.Mode), Effects: effects, Steps: steps, Drops: drops, AttackerPosition: state.Player.Position, TargetPosition: state.Monster.Position, AttackerHPBefore: a.Health}
	c.MapChanged = c.MapChanged || mapChanged
	return out, nil
}

// ApplyHexMonsterAttack is a server-only entrypoint. It has no HTTP route;
// callers supply a stable command ID for a monster action so retries replay.
func ApplyHexMonsterAttack(ctx context.Context, instanceID string, monsterID, playerID int, commandID string, publish func(HexAttackResult)) (HexAttackResult, error) {
	var out HexAttackResult
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c, err := repository.BeginMatchCommand(ctx, instanceID, 0, commandID, fmt.Sprintf("hex-monster-attack-v1:%d:%d", monsterID, playerID))
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
	state, err := readHexAttackTx(ctx, c.Tx, instanceID, playerID, monsterID, false)
	if err != nil {
		return out, err
	}
	if state.Distance != 1 {
		return out, repository.ErrHexBlocked
	}
	a, d := state.Monster.Stats, state.Player.Stats
	effectivePlayer := d
	// Monster attacks cannot use player-owned Ranger armor break.
	if effectivePlayer.Defense < 0 {
		effectivePlayer.Defense = 0
	}
	hit := applyDamage(a, effectivePlayer)
	blocked := resolveGuardianShieldBlock("player", d)
	if blocked {
		hit = attackResult{NewHealth: d.Health}
	}
	monsterRef := CombatTargetRef{ID: monsterID, Type: CombatActorMonster}
	playerRef := CombatTargetRef{ID: playerID, Type: CombatActorPlayer}
	steps := []CombatStep{{Kind: "hit", Source: &monsterRef, Target: playerRef, Damage: hit.Damage, TargetHPAfter: hit.NewHealth}}
	effects := []CombatEffect{}
	if blocked {
		effects = append(effects, CombatEffect{Kind: "block", Target: &playerRef, Succeeded: true})
	}
	counter := attackResult{NewHealth: a.Health}
	ap := state.Player.AP
	if hit.NewHealth > 0 && state.Player.Reactions > 0 {
		if err := c.SpendHexTx(ctx, playerID, 0, 0, 1); err != nil {
			return out, err
		}
		effectiveMonster := a
		// Reactions never borrow Ranger main-attack armor break.
		if effectiveMonster.Defense < 0 {
			effectiveMonster.Defense = 0
		}
		counter = applyDamage(d, effectiveMonster)
		steps = append(steps, CombatStep{Kind: "counter", Source: &playerRef, Target: monsterRef, Damage: counter.Damage, TargetHPAfter: counter.NewHealth})
	}
	mapChanged := false
	if hit.NewHealth > 0 {
		if err := c.SetHexCombatVitalsTx(ctx, playerID, hit.NewHealth, ap); err != nil {
			return out, err
		}
	}
	monsterDied := counter.Triggered && counter.NewHealth <= 0
	var drops []EquipmentDropPayload
	if monsterDied {
		if err := c.GrantProfessionMonsterLootTx(ctx, playerID, monsterID); err != nil {
			return out, err
		}
		result, err := c.Tx.ExecContext(ctx, `DELETE FROM match_monsters WHERE instance_id=$1 AND monster_instance_id=$2 AND health>0`, instanceID, monsterID)
		if err != nil {
			return out, err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return out, repository.ErrCommandRejected
		}
		mapChanged = true
		steps = append(steps, CombatStep{Kind: "death", Target: monsterRef})
		if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET monster=NULL WHERE instance_id=$1 AND q=$2 AND r=$3`, instanceID, state.Monster.Position.Q, state.Monster.Position.R); err != nil {
			return out, err
		}
		drops, err = grantHexMonsterEquipmentDropTx(c.Tx, playerID, instanceID)
		if err != nil {
			return out, err
		}
	} else if counter.Triggered {
		if err := c.SetMonsterHealthTx(ctx, monsterID, counter.NewHealth); err != nil {
			return out, err
		}
		if _, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET monster=jsonb_set(monster,'{health}',to_jsonb($3::int)) WHERE instance_id=$1 AND monster->>'db_instance_id'=$2::text`, instanceID, monsterID, counter.NewHealth); err != nil {
			return out, err
		}
	}
	for _, event := range hexDamageEventsFromSteps(steps, effects, map[CombatTargetRef]int{monsterRef: state.Monster.Stats.Health, playerRef: state.Player.Stats.Health}) {
		if err := c.RecordDamageEventTx(ctx, event); err != nil {
			return out, err
		}
	}
	if monsterDied {
		if _, err := c.RecordKillEventTx(ctx, game.KillEvent{SourceType: game.ResultSourcePlayer, KillerID: playerID, VictimType: "monster", VictimID: monsterID, Damage: counter.Damage}); err != nil {
			return out, err
		}
	}
	if hit.NewHealth <= 0 {
		steps = append(steps, CombatStep{Kind: "death", Target: playerRef})
		if err := finishHexPlayerDeathTx(ctx, c, playerID, monsterID, hit.Damage, state.Player.Position); err != nil {
			return out, err
		}
		mapChanged = true
	}
	out = HexAttackResult{HexAttackPreview: HexAttackPreview{TargetID: playerID, Distance: state.Distance, Mode: attackModeMelee, AP: ap, TargetHP: d.Health}, Damage: hit.Damage, CounterDamage: counter.Damage, NewTargetHP: hit.NewHealth, NewAttackerHP: counter.NewHealth, MonsterDied: monsterDied, AttackStyle: resolvePresentationAttackStyle(a, attackModeMelee), Effects: effects, Steps: steps, Drops: drops, AttackerPosition: state.Monster.Position, TargetPosition: state.Player.Position, AttackerHPBefore: a.Health}
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, mapChanged)
	if err != nil {
		return HexAttackResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}

func finishHexPlayerDeathTx(ctx context.Context, c *repository.MatchCommand, playerID, monsterID, damage int, position hexgrid.Coord) error {
	if _, err := c.RecordKillEventTx(ctx, game.KillEvent{SourceType: game.ResultSourceMonster, KillerID: monsterID, VictimType: "player", VictimID: playerID, Damage: damage}); err != nil {
		return err
	}
	if err := c.MarkCombatPlayerDeadTx(ctx, playerID); err != nil {
		return err
	}
	if err := c.LoseHexBackpackTx(ctx, playerID, position); err != nil {
		return err
	}
	turn, err := c.RemoveCombatPlayerFromTurnOrderTx(ctx, playerID)
	if err != nil {
		return err
	}
	if err := c.ResetHexTurnDeadlineTx(ctx, turn.ActiveUserID); err != nil {
		return err
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=false WHERE instance_id=$1 AND q=$2 AND r=$3 AND is_player=true`, c.InstanceID, position.Q, position.R)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return repository.ErrCommandRejected
	}
	if turn.ActiveUserID != 0 {
		return nil
	}
	var complete bool
	if err := c.Tx.QueryRowContext(ctx, `SELECT combat_results_complete FROM matches WHERE instance_id=$1`, c.InstanceID).Scan(&complete); err != nil {
		return err
	}
	if complete {
		return repository.ErrCommandRejected
	}
	_, err = c.Tx.ExecContext(ctx, `UPDATE matches SET finished_at=now(),hex_turn_deadline=NULL WHERE instance_id=$1`, c.InstanceID)
	return err
}

func hexCombatHTTPError(w http.ResponseWriter, err error) {
	status := hexCommandStatus(err)
	if errors.Is(err, errHexCombatUnsupported) {
		status = http.StatusUnprocessableEntity
	}
	if errors.Is(err, repository.ErrHexBlocked) || errors.Is(err, repository.ErrHexEnergy) {
		status = http.StatusBadRequest
	}
	http.Error(w, err.Error(), status)
}

func PreviewHexAttackHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := hexUser(w, r)
	if !ok {
		return
	}
	var req struct {
		TargetID   int    `json:"targetId"`
		TargetKind string `json:"targetKind"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	kind := req.TargetKind
	if kind == "" {
		kind = "monster"
	}
	preview, err := PreviewHexEngagement(r.Context(), mux.Vars(r)["instance_id"], userID, req.TargetID, kind)
	if err != nil {
		hexCombatHTTPError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(preview)
}

func AttackHexMonsterHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := hexUser(w, r)
	if !ok {
		return
	}
	var req struct {
		TargetID     int    `json:"targetId"`
		TargetKind   string `json:"targetKind"`
		CommandID    string `json:"commandId"`
		PreviewToken string `json:"previewToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CommandID == "" || len(req.CommandID) > 128 {
		http.Error(w, "commandId and targetId required", 400)
		return
	}
	id := mux.Vars(r)["instance_id"]
	kind := req.TargetKind
	if kind == "" {
		kind = "monster"
	}
	result, err := ApplyHexEngagement(r.Context(), id, userID, req.TargetID, kind, req.CommandID, req.PreviewToken, func(committed HexEngagementResult) {
		event := "MOVE_PLAYER"
		if committed.Attacked {
			event = "COMBAT_EXCHANGE"
		}
		publishHexPatch(id, req.CommandID, committed.StateRevision, committed.MapRevision, event, map[string]any{"instanceId": id, "userId": userID, "targetId": req.TargetID})
		if committed.Movement != nil && len(committed.Movement.Path) > 0 {
			publishHexVisual(id, req.CommandID+":movement", committed.StateRevision, "move", map[string]any{"userId": userID, "position": committed.Movement.Position, "path": committed.Movement.Path, "auraMPCost": committed.Movement.AuraMPCost})
		}
		if committed.Attacked {
			publishHexVisual(id, req.CommandID, committed.StateRevision, "attack", map[string]any{"attackerPosition": committed.AttackerPosition, "targetPosition": committed.TargetPosition, "attackerHPBefore": committed.AttackerHPBefore, "targetHP": committed.TargetHP, "steps": committed.Steps, "effects": committed.Effects})
		}
	})
	if err != nil {
		hexCombatHTTPError(w, err)
		return
	}
	if result.NewAttackerHP <= 0 && !result.Replayed {
		var active, turn int
		if err := repository.DB.QueryRowContext(r.Context(), `SELECT active_user_id,turn_number FROM matches WHERE instance_id=$1 AND finished_at IS NULL`, id).Scan(&active, &turn); err == nil {
			scheduleHexTurn(id, active, turn)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
