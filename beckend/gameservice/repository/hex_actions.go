package repository

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"gameservice/game"
	"gameservice/internal/game/balance"
	"gameservice/internal/hexgrid"
)

const HexMaxAP = 6 // HALF points, also the wire format.
const HexMPCap = 16

//go:embed sql/hex_action_system_stage9.sql
var hexActionSchema string

//go:embed sql/combat_ruleset_v34.sql
var combatRulesetSchema string

func EnsureHexActionSchema() error {
	_, err := DB.Exec(hexActionSchema + "\n" + combatRulesetSchema)
	return err
}

type HexBudgets struct {
	MagicShield int `json:"magicShield,omitempty"`
	CombatProfile      balance.CombatProfile `json:"combatProfile"`
	InGuardianAura     bool                  `json:"inGuardianAura"`
	AP                 int                   `json:"ap"`
	MaxAP              int                   `json:"maxAP"`
	MP                 int                   `json:"mp"`
	MaxMP              int                   `json:"maxMP"`
	Reactions          int                   `json:"reactions"`
	MaxReactions       int                   `json:"maxReactions"`
	PersonalTurn       int                   `json:"personalTurn"`
	WaterAvailableTurn int                   `json:"waterAvailableTurn"`
	MPDrain            bool                  `json:"mpDrain"`
	OverloadTarget     string                `json:"overloadTarget"`
	OverloadCount      int                   `json:"overloadCount"`
	MysticReflexUsed   bool                  `json:"mysticReflexUsed"`
	AuraSlowUsed       bool                  `json:"auraSlowUsed"`
}

func ReadHexBudgetsTx(ctx context.Context, tx storageQuerier, instance string, user int) (HexBudgets, error) {
	var b HexBudgets
	err := tx.QueryRowContext(ctx, `SELECT ap,max_ap,mp,max_mp,reactions,max_reactions,personal_turn_counter,water_available_turn,hex_mp_drain,hex_overload_target,hex_overload_count,hex_mystic_reflex_used,hex_aura_slow_used FROM match_players WHERE instance_id=$1 AND user_id=$2`, instance, user).Scan(&b.AP, &b.MaxAP, &b.MP, &b.MaxMP, &b.Reactions, &b.MaxReactions, &b.PersonalTurn, &b.WaterAvailableTurn, &b.MPDrain, &b.OverloadTarget, &b.OverloadCount, &b.MysticReflexUsed, &b.AuraSlowUsed)
	if err == nil {
		b.CombatProfile, err = ReadHexCombatProfileTx(ctx, tx, instance, user)
	}
	if err==nil && b.CombatProfile.Ruleset==balance.ExperimentalV34 {
		var class string
		var attack,reflex int
		err=tx.QueryRowContext(ctx,`SELECT character_type,attack,agility,combat_magic_shield FROM match_players WHERE instance_id=$1 AND user_id=$2`,instance,user).Scan(&class,&attack,&reflex,&b.MagicShield)
		if err==nil {
			_,config,e:=ReadMatchCombatRulesTx(ctx,tx,instance)
			if e!=nil {err=e
			} else if config!=nil {
				chance:=balance.CalculateReflexProcChance(reflex)
				if class=="guardian" {chance=balance.ResearchRound(float64(chance)*config.Factors["block"]);if chance>55 {chance=55}}
				b.CombatProfile.ReflexChance=&chance
				b.CombatProfile.ClassHealPercent=50*config.Factors["heal"]
				b.CombatProfile.FollowupMultiplier=.5*config.Factors["followup"]
				b.CombatProfile.CriticalMultiplier=config.Factors["crit"]
				b.CombatProfile.OverloadSecond=balance.ResearchRound(2*config.Factors["overload"])
				b.CombatProfile.OverloadThird=balance.ResearchRound(5*config.Factors["overload"])
				b.CombatProfile.MysticReflexDamage=balance.ResearchRound(3*config.Factors["magic"])
				if class=="guardian" {b.CombatProfile.AbilityAttackScale=float64(attack)/float64(config.BaseAttack[0]);b.CombatProfile.AuraExit=balance.ResearchRound(3*config.Factors["aura"])
				} else if class=="mystic" {b.CombatProfile.AbilityAttackScale=float64(attack)/float64(config.BaseAttack[3])}
			}
		}
	}
	if err == nil {
		var all []hexAuraMember
		all, err = readHexAuraMembersTx(ctx, tx, instance)
		if err == nil {
			b.InGuardianAura = auraMembership(all)[user] != 0
		}
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

// Callers admit connected main attacks: new v34 includes physical blocks,
// excludes dodges; saved pre-penetration configurations retain their old gates.
func (c *MatchCommand) MysticHitTx(ctx context.Context, user int, targetKind string, targetID int, reflexProc bool) (int, int, error) {
	key := fmt.Sprintf("%s:%d", targetKind, targetID)
	var last string
	var count int
	var used bool
	var reflexStat int
	if err := c.Tx.QueryRowContext(ctx, `SELECT hex_overload_target,hex_overload_count,hex_mystic_reflex_used,agility FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user).Scan(&last, &count, &used,&reflexStat); err != nil {
		return 0, 0, err
	}
	if last != key {
		count = 0
	}
	count++
	if count > 4 {
		count = 4
	}
	profile, err := ReadHexCombatProfileTx(ctx, c.Tx, c.InstanceID, user)
	if err != nil {
		return 0, 0, err
	}
	extra := 0
	if count == 2 {
		extra = profile.OverloadSecond
	}
	if count == 3 {
		extra = profile.OverloadThird
	}
	if c.CombatConfig!=nil {
		extra=balance.ResearchRound(float64(extra)*c.CombatConfig.Factors["overload"])
		reflexProc=!used && c.RollResearchChance(float64(balance.CalculateReflexProcChance(reflexStat)))
	}
	reflex := 0
	if !used && reflexProc {
		reflex = profile.MysticReflexDamage
		if c.CombatConfig!=nil {reflex=balance.ResearchRound(float64(reflex)*c.CombatConfig.Factors["magic"])}
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
	Attack int
	Profile            balance.CombatProfile
	User, Team, Health int
	Class              string
	Position           hexgrid.Coord
}

func (c *MatchCommand) AuraMembersTx(ctx context.Context) ([]hexAuraMember, error) {
	return readHexAuraMembersTx(ctx, c.Tx, c.InstanceID)
}
func readHexAuraMembersTx(ctx context.Context, tx storageQuerier, instance string) ([]hexAuraMember, error) {
	var raw []byte
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('User',user_id,'Team',group_id,'Health',health,'Class',character_type,'Attack',attack,'Position',jsonb_build_object('q',position_q,'r',position_r),'Profile',hex_combat_balance) ORDER BY user_id),'[]'::jsonb) FROM match_players WHERE instance_id=$1 AND health>0`, instance).Scan(&raw)
	if err != nil {
		return nil, err
	}
	out := []hexAuraMember{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Profile.AuraRadius <= 0 {
			out[i].Profile = balance.DefaultCombatProfile()
		}
	}
	return out, nil
}

func auraMembership(all []hexAuraMember) map[int]int {
	out := map[int]int{}
	for _, p := range all {
		for _, g := range all {
			if p.User == g.User || g.Class != "guardian" || p.Team == g.Team {
				continue
			}
			d, err := hexgrid.Distance(p.Position, g.Position)
			if err == nil && d <= g.Profile.AuraRadius {
				out[p.User] = g.User
				break
			}
		}
	}
	return out
}
func auraProfile(all []hexAuraMember, user int) balance.CombatProfile {
	for _, p := range all {
		if p.User == user {
			return p.Profile
		}
	}
	return balance.DefaultCombatProfile()
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
	if c.CombatConfig!=nil {
		var class string
		if err:=c.Tx.QueryRowContext(ctx,`SELECT character_type FROM match_players WHERE instance_id=$1 AND user_id=$2`,c.InstanceID,user).Scan(&class);err!=nil {return err}
		event:=kind
		attack:=0
		switch kind {
		case "guardianAuraStart":event="aura_tick"
		case "guardianAuraStartExtra":event="aura_tick_extra"
		case "guardianAuraExit":event="aura_exit";damage=balance.ResearchRound(float64(damage)*c.CombatConfig.Factors["aura"])
		}
		if event=="aura_tick" || event=="aura_tick_extra" || event=="aura_exit" {
			if err:=c.Tx.QueryRowContext(ctx,`SELECT attack FROM match_players WHERE instance_id=$1 AND user_id=$2`,c.InstanceID,source).Scan(&attack);err!=nil {return err}
		}
		resolved,err:=c.ResolveResearchDamageTx(ctx,user,balance.V34Incoming{Kind:event,Damage:damage,Attack:attack,Defender:class})
		if err!=nil {return err};damage=resolved.Damage
		for _,effect:=range resolved.Effects {c.ActionEffects=append(c.ActionEffects,map[string]any{"kind":effect.Kind,"userId":user,"sourceId":source,"position":pos,"amount":effect.Amount})}
		if damage<=0 {return nil}
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

type AuraMovementCause string

const (
	AuraMovementVoluntary AuraMovementCause = "voluntary"
	AuraMovementForced    AuraMovementCause = "forced"
)

// The movement executor records the cause explicitly, independently of turn ownership.
// Entries belong to the next transition only; route steps resolve separately.
func (c *MatchCommand) RecordAuraMovement(user int, cause AuraMovementCause) {
	if c.auraMovements == nil {
		c.auraMovements = make(map[int]AuraMovementCause)
	}
	c.auraMovements[user] = cause
	c.AuraResolved = false
}

// A single transition event, including voluntary steps and forced displacement.
func (c *MatchCommand) ResolveAuraTransitionTx(ctx context.Context, before []hexAuraMember) error {
	after, err := c.AuraMembersTx(ctx)
	if err != nil {
		return err
	}
	movements := c.auraMovements
	c.auraMovements = nil
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
			// Keep the union-of-enemy-auras exit rule. Exclude only owners whose
			// voluntary movement caused this exit: with that owner held at its old
			// position, the enemy would still be inside. Other owners remain eligible,
			// including when this player is itself a voluntarily moving Guardian.
			exitBefore := append([]hexAuraMember(nil), before...)
			for i, g := range exitBefore {
				if g.Class != "guardian" || movements[g.User] != AuraMovementVoluntary {
					continue
				}
				distance, e := hexgrid.Distance(p.Position, g.Position)
				if e == nil && distance <= g.Profile.AuraRadius {
					exitBefore[i].Class = "" // Suppress this aura as a damage source only.
				}
			}
			source = auraMembership(exitBefore)[p.User]
			if source == 0 {
				continue
			}
			if err := c.AuraDamageTx(ctx, p.User, source, auraProfile(before, source).AuraExit, "guardianAuraExit"); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *MatchCommand) StartHexPersonalTurnTx(ctx context.Context, user int) error {
	if err:=c.StartResearchPersonalTurnTx(ctx,user);err!=nil {return err}
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
		if err:=c.AuraDamageTx(ctx,user,source,auraProfile(all,source).AuraTick,"guardianAuraStart");err!=nil {return err}
		// v22 adds only a POSITIVE correction to the original 2-HP tick.
		// Preserve the two separate rounding/buffer events when tuning above 1.
		if c.CombatConfig!=nil {extra:=balance.ResearchRound(2*c.CombatConfig.Factors["aura"])-2;if extra>0 {return c.AuraDamageTx(ctx,user,source,extra,"guardianAuraStartExtra")}}
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

// Legacy and saved pre-penetration v34 only: a block breaks the series
// without refunding its once-per-turn Reflex. New v34 blocks do not call this.
func (c *MatchCommand) ResetHexOverloadTx(ctx context.Context, user int) error {
	_, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET hex_overload_target='',hex_overload_count=0 WHERE instance_id=$1 AND user_id=$2 AND character_type='mystic'`, c.InstanceID, user)
	return err
}
