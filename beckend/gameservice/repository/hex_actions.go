package repository

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"gameservice/game"
	"gameservice/internal/hexgrid"
)

const HexMaxAP = 6 // HALF points, also the wire format.
const HexMPCap = 16

//go:embed sql/hex_action_system_stage9.sql
var hexActionSchema string

func EnsureHexActionSchema() error { _, err := DB.Exec(hexActionSchema); return err }

type HexBudgets struct {
	InGuardianAura     bool   `json:"inGuardianAura"`
	AP                 int    `json:"ap"`
	MaxAP              int    `json:"maxAP"`
	MP                 int    `json:"mp"`
	MaxMP              int    `json:"maxMP"`
	Reactions          int    `json:"reactions"`
	MaxReactions       int    `json:"maxReactions"`
	PersonalTurn       int    `json:"personalTurn"`
	WaterAvailableTurn int    `json:"waterAvailableTurn"`
	MPDrain            bool   `json:"mpDrain"`
	OverloadTarget     string `json:"overloadTarget"`
	OverloadCount      int    `json:"overloadCount"`
	MysticReflexUsed   bool   `json:"mysticReflexUsed"`
	AuraSlowUsed       bool   `json:"auraSlowUsed"`
}

func ReadHexBudgetsTx(ctx context.Context, tx storageQuerier, instance string, user int) (HexBudgets, error) {
	var b HexBudgets
	err := tx.QueryRowContext(ctx, `SELECT ap,max_ap,mp,max_mp,reactions,max_reactions,personal_turn_counter,water_available_turn,hex_mp_drain,hex_overload_target,hex_overload_count,hex_mystic_reflex_used,hex_aura_slow_used FROM match_players WHERE instance_id=$1 AND user_id=$2`, instance, user).Scan(&b.AP, &b.MaxAP, &b.MP, &b.MaxMP, &b.Reactions, &b.MaxReactions, &b.PersonalTurn, &b.WaterAvailableTurn, &b.MPDrain, &b.OverloadTarget, &b.OverloadCount, &b.MysticReflexUsed, &b.AuraSlowUsed)
	if err == nil {
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM match_players p JOIN match_players g ON g.instance_id=p.instance_id AND g.user_id<>p.user_id AND g.group_id<>p.group_id AND g.character_type='guardian' AND g.health>0 WHERE p.instance_id=$1 AND p.user_id=$2 AND p.health>0 AND GREATEST(abs(g.position_q-p.position_q),abs(g.position_r-p.position_r),abs((g.position_q-p.position_q)+(g.position_r-p.position_r)))<=2)`, instance, user).Scan(&b.InGuardianAura)
	}
	return b, err
}
func (c *MatchCommand) SpendHexTx(ctx context.Context, user, ap, mp, reactions int) error {
	if ap < 0 || mp < 0 || reactions < 0 {
		return ErrCommandRejected
	}
	result, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET ap=ap-$3,mp=mp-$4,reactions=reactions-$5 WHERE instance_id=$1 AND user_id=$2 AND health>0 AND ap>=$3 AND mp>=$4 AND reactions>=$5`, c.InstanceID, user, ap, mp, reactions)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrHexEnergy
	}
	return nil
}
func (c *MatchCommand) GrantHexMPTx(ctx context.Context, user, units int) error {
	_, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET mp=LEAST($4,mp+$3) WHERE instance_id=$1 AND user_id=$2 AND health>0`, c.InstanceID, user, units, HexMPCap)
	return err
}
func (c *MatchCommand) ClearRangerBreakTx(ctx context.Context, owner int, keepKind string, keepID int) error {
	for _, target := range []struct{ table, id, kind string }{{"match_players", "user_id", "player"}, {"match_monsters", "monster_instance_id", "monster"}} {
		query := fmt.Sprintf(`UPDATE %s SET hex_armor_break_owner=0,hex_armor_break_stacks=0,hex_armor_break_turns=0 WHERE instance_id=$1 AND hex_armor_break_owner=$2 AND NOT ($3::text=$4::text AND %s=$5)`, target.table, target.id)
		if _, err := c.Tx.ExecContext(ctx, query, c.InstanceID, owner, keepKind, target.kind, keepID); err != nil {
			return err
		}
	}
	return nil
}
func (c *MatchCommand) SetRangerBreakTx(ctx context.Context, kind string, id, owner, stacks int) error {
	table, column := "match_monsters", "monster_instance_id"
	if kind == "player" {
		table, column = "match_players", "user_id"
	}
	if stacks == 0 {
		owner = 0
	}
	if stacks < 0 || stacks > 2 {
		return ErrCommandRejected
	}
	_, err := c.Tx.ExecContext(ctx, fmt.Sprintf(`UPDATE %s SET hex_armor_break_owner=$3,hex_armor_break_stacks=$4,hex_armor_break_turns=0 WHERE instance_id=$1 AND %s=$2`, table, column), c.InstanceID, id, owner, stacks)
	return err
}

// Only triggered MAIN hits advance a series; blocks and failed commands do not.
func (c *MatchCommand) MysticHitTx(ctx context.Context, user int, targetKind string, targetID int, reflexProc bool) (int, int, error) {
	key := fmt.Sprintf("%s:%d", targetKind, targetID)
	var last string
	var count int
	var used bool
	if err := c.Tx.QueryRowContext(ctx, `SELECT hex_overload_target,hex_overload_count,hex_mystic_reflex_used FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user).Scan(&last, &count, &used); err != nil {
		return 0, 0, err
	}
	if last != key {
		count = 0
	}
	count++
	if count > 4 {
		count = 4
	}
	extra := 0
	if count == 2 {
		extra = 2
	}
	if count == 3 {
		extra = 5
	}
	reflex := 0
	if !used && reflexProc {
		reflex = 3
		used = true
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET hex_overload_target=$3,hex_overload_count=$4,hex_mystic_reflex_used=$5 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user, key, count, used); err != nil {
		return 0, 0, err
	}
	return extra, reflex, nil
}
func (c *MatchCommand) MysticDrainTx(ctx context.Context, user, target int) (bool, error) {
	if immune, err := c.CraftImmuneTx(ctx, target); err != nil || immune {
		return false, err
	}
	var round, used int
	var pending bool
	if err := c.Tx.QueryRowContext(ctx, `SELECT m.turn_number,p.hex_mystic_drain_round,t.hex_mp_drain FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=$2 JOIN match_players t ON t.instance_id=m.instance_id AND t.user_id=$3 WHERE m.instance_id=$1`, c.InstanceID, user, target).Scan(&round, &used, &pending); err != nil {
		return false, err
	}
	if pending || used == round {
		return false, nil
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET hex_mp_drain=true WHERE instance_id=$1 AND user_id=$2 AND health>0`, c.InstanceID, target); err != nil {
		return false, err
	}
	_, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET hex_mystic_drain_round=$3 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user, round)
	return err == nil, err
}

type hexAuraMember struct {
	User, Team, Health int
	Class              string
	Position           hexgrid.Coord
}

func (c *MatchCommand) AuraMembersTx(ctx context.Context) ([]hexAuraMember, error) {
	rows, err := c.Tx.QueryContext(ctx, `SELECT user_id,group_id,health,character_type,position_q,position_r FROM match_players WHERE instance_id=$1 AND health>0 ORDER BY user_id`, c.InstanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []hexAuraMember{}
	for rows.Next() {
		var p hexAuraMember
		if err := rows.Scan(&p.User, &p.Team, &p.Health, &p.Class, &p.Position.Q, &p.Position.R); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func auraMembership(all []hexAuraMember) map[int]int {
	out := map[int]int{}
	for _, p := range all {
		for _, g := range all {
			if p.User == g.User || g.Class != "guardian" || p.Team == g.Team {
				continue
			}
			d, err := hexgrid.Distance(p.Position, g.Position)
			if err == nil && d <= 2 {
				out[p.User] = g.User
				break
			}
		}
	}
	return out
}
func (c *MatchCommand) AuraDamageTx(ctx context.Context, user, source, damage int, kind string) error {
	var hp int
	var pos hexgrid.Coord
	if err := c.Tx.QueryRowContext(ctx, `SELECT health,position_q,position_r FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user).Scan(&hp, &pos.Q, &pos.R); err != nil {
		return err
	}
	if hp <= 0 {
		return nil
	}
	if damage > hp {
		damage = hp
	}
	if err := c.RecordDamageEventTx(ctx, game.DamageEvent{SourceType: game.ResultSourcePlayer, DealerID: source, TargetType: "player", TargetID: user, Amount: damage}); err != nil {
		return err
	}
	// Public visual effects are returned with the command; positions are filtered by the publisher.
	c.ActionEffects = append(c.ActionEffects, map[string]any{"kind": kind, "userId": user, "sourceId": source, "position": pos, "amount": damage})
	if hp > damage {
		_, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET health=health-$3 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user, damage)
		return err
	}
	if _, err := c.RecordKillEventTx(ctx, game.KillEvent{SourceType: game.ResultSourcePlayer, KillerID: source, VictimType: "player", VictimID: user, Damage: damage}); err != nil {
		return err
	}
	c.MapChanged = true
	if err := c.MarkCombatPlayerDeadTx(ctx, user); err != nil {
		return err
	}
	if err := c.LoseHexBackpackTx(ctx, user, pos); err != nil {
		return err
	}
	turn, err := c.RemoveCombatPlayerFromTurnOrderTx(ctx, user)
	if err != nil {
		return err
	}
	if _, err = c.Tx.ExecContext(ctx, `UPDATE match_map_cells SET is_player=false WHERE instance_id=$1 AND q=$2 AND r=$3`, c.InstanceID, pos.Q, pos.R); err != nil {
		return err
	}
	if err = c.ResetHexTurnDeadlineTx(ctx, turn.ActiveUserID); err != nil {
		return err
	}
	if turn.ActiveUserID == 0 {
		_, err = c.Tx.ExecContext(ctx, `UPDATE matches SET finished_at=now(),hex_turn_deadline=NULL WHERE instance_id=$1`, c.InstanceID)
	}
	return err
}

// A single transition event, including a Ranger push or the Guardian moving away.
func (c *MatchCommand) ResolveAuraTransitionTx(ctx context.Context, before []hexAuraMember) error {
	after, err := c.AuraMembersTx(ctx)
	if err != nil {
		return err
	}
	moved := false
	for _, a := range after {
		for _, b := range before {
			if a.User == b.User && a.Position != b.Position {
				moved = true
			}
		}
	}
	if !moved {
		return nil
	}
	old, now := auraMembership(before), auraMembership(after)
	for _, p := range after {
		if source := old[p.User]; source != 0 && now[p.User] == 0 {
			if err := c.AuraDamageTx(ctx, p.User, source, 3, "guardianAuraExit"); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *MatchCommand) StartHexPersonalTurnTx(ctx context.Context, user int) error {
	if err := c.ClearRangerBreakTx(ctx, user, "", 0); err != nil {
		return err
	}
	if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET ap=max_ap,mp=GREATEST(0,max_mp-CASE WHEN hex_mp_drain THEN 2 ELSE 0 END),reactions=max_reactions,personal_turn_counter=personal_turn_counter+1,hex_mp_drain=false,hex_overload_target='',hex_overload_count=0,hex_mystic_reflex_used=false,hex_aura_slow_used=false,hex_first_step_used=false WHERE instance_id=$1 AND user_id=$2 AND health>0`, c.InstanceID, user); err != nil {
		return err
	}
	if err := c.TickCraftStatusesTx(ctx, user); err != nil {
		return err
	}
	all, err := c.AuraMembersTx(ctx)
	if err != nil {
		return err
	}
	if source := auraMembership(all)[user]; source != 0 {
		return c.AuraDamageTx(ctx, user, source, 2, "guardianAuraStart")
	}
	return nil
}
func (c *MatchCommand) ResolveHexTurnStartTx(ctx context.Context) error {
	var active int
	var finished bool
	if err := c.Tx.QueryRowContext(ctx, `SELECT COALESCE(active_user_id,0),finished_at IS NOT NULL FROM matches WHERE instance_id=$1`, c.InstanceID).Scan(&active, &finished); err != nil {
		return err
	}
	if finished || active == 0 || (!c.NextPersonalTurn && active == c.ActiveUser) {
		return nil
	}
	for active > 0 && !finished {
		previous := active
		if err := c.StartHexPersonalTurnTx(ctx, active); err != nil {
			return err
		}
		if err := c.Tx.QueryRowContext(ctx, `SELECT COALESCE(active_user_id,0),finished_at IS NOT NULL FROM matches WHERE instance_id=$1`, c.InstanceID).Scan(&active, &finished); err != nil {
			return err
		}
		if active == previous {
			break
		}
	}
	return nil
}

type HexExchangeResult struct {
	Budget        *HexBudgets      `json:"budget,omitempty"`
	ActionEffects []map[string]any `json:"actionEffects,omitempty"`
	HexBudgets
	StateRevision int64 `json:"stateRevision"`
	MapRevision   int64 `json:"mapRevision"`
}

func (r *HexExchangeResult) SetCommandRevisions(s, m int64) { r.StateRevision, r.MapRevision = s, m }
func ExchangeHexAP(ctx context.Context, instance string, user int, id string) (HexExchangeResult, error) {
	var out HexExchangeResult
	c, err := BeginMatchCommand(ctx, instance, user, id, fmt.Sprintf("hex-exchange-ap-v1:%d", user))
	if err != nil {
		return out, err
	}
	defer c.Rollback()
	if c.Replay != nil {
		err = json.Unmarshal(c.Replay, &out)
		return out, err
	}
	if c.ActiveUser != user {
		return out, ErrCommandRejected
	}
	b, err := ReadHexBudgetsTx(ctx, c.Tx, instance, user)
	if err != nil {
		return out, err
	}
	if b.MP+4 > HexMPCap {
		return out, ErrCommandRejected
	}
	if err = c.SpendHexTx(ctx, user, 2, 0, 0); err != nil {
		return out, err
	}
	if err = c.GrantHexMPTx(ctx, user, 4); err != nil {
		return out, err
	}
	out.HexBudgets, err = ReadHexBudgetsTx(ctx, c.Tx, instance, user)
	if err != nil {
		return out, err
	}
	_, _, err = c.Commit(ctx, &out, false)
	return out, err
}

// Preserve legacy signature for non-HEX callers; the final argument is AP in HEX.
func (c *MatchCommand) SetHexCombatVitalsTx(ctx context.Context, user, hp, ap int) error {
	_, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET health=$3,ap=$4 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user, hp, ap)
	return err
}

func (c *MatchCommand) PrepareHexMainTargetTx(ctx context.Context, user int, kind string, id int) error {
	key := fmt.Sprintf("%s:%d", kind, id)
	_, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET hex_overload_target=$3,hex_overload_count=0 WHERE instance_id=$1 AND user_id=$2 AND character_type='mystic' AND hex_overload_target<>$3`, c.InstanceID, user, key)
	return err
}
