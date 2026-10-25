package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"gameservice/internal/hexgrid"
)

var ErrCraftConsumable = errors.New("consumable unavailable: check target, cooldown or active effect")
var ErrCraftPoisonMonster = errors.New("Poison can only be used against an adjacent enemy player.")

type CraftPeriodic struct {
	Percent   int `json:"percent"`
	Remaining int `json:"remaining"`
	Source    int `json:"source"`
}
type CraftBuff struct {
	Kind    string `json:"kind"`
	Until   int    `json:"until"`
	Charges int    `json:"charges,omitempty"`
}
type CraftStatus struct {
	Cooldowns   map[string]int  `json:"cooldowns,omitempty"`
	Heal        []CraftPeriodic `json:"heal,omitempty"`
	Poison      []CraftPeriodic `json:"poison,omitempty"`
	Critical    *CraftBuff      `json:"critical,omitempty"`
	Vampirism   *CraftBuff      `json:"vampirism,omitempty"`
	Speed       *CraftBuff      `json:"speed,omitempty"`
	SpeedTicks  int             `json:"speedTicks,omitempty"`
	ImmuneUntil int             `json:"immuneUntil,omitempty"`
}

func readCraftStatusTx(ctx context.Context, q storageQuerier, instance string, user int) (CraftStatus, int, error) {
	var state CraftStatus
	var raw []byte
	var turn int
	err := q.QueryRowContext(ctx, `SELECT craft_status,personal_turn_counter FROM match_players WHERE instance_id=$1 AND user_id=$2`, instance, user).Scan(&raw, &turn)
	if err == nil {
		err = json.Unmarshal(raw, &state)
	}
	if state.Cooldowns == nil {
		state.Cooldowns = map[string]int{}
	}
	return state, turn, err
}
func writeCraftStatusTx(ctx context.Context, tx *sql.Tx, instance string, user int, s CraftStatus) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE match_players SET craft_status=$3 WHERE instance_id=$1 AND user_id=$2`, instance, user, string(raw))
	return err
}
func (c *MatchCommand) CraftImmuneTx(ctx context.Context, user int) (bool, error) {
	s, t, e := readCraftStatusTx(ctx, c.Tx, c.InstanceID, user)
	return s.ImmuneUntil > t, e
}
func percentHP(maxHP, percent int) int {
	n := maxHP * percent / 100
	if n < 1 && percent > 0 && maxHP > 0 {
		return 1
	}
	return n
}

func (c *MatchCommand) applyCraftConsumableTx(ctx context.Context, user, target int, r CraftRecipe) error {
	s, turn, err := readCraftStatusTx(ctx, c.Tx, c.InstanceID, user)
	if err != nil {
		return err
	}
	if s.Cooldowns[r.Effect] > turn {
		return ErrCraftConsumable
	}
	if target == 0 {
		target = user
	}
	if r.Effect == "poison" && target == user {
		return ErrCraftPoisonMonster
	}
	// Offensive use and ally healing are limited to a visible adjacent player.
	var hp, maxHP, team, ownTeam int
	var pos, own hexgrid.Coord
	if err = c.Tx.QueryRowContext(ctx, `SELECT health,max_health,group_id,position_q,position_r FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, target).Scan(&hp, &maxHP, &team, &pos.Q, &pos.R); err != nil {
		return ErrCraftConsumable
	}
	if err = c.Tx.QueryRowContext(ctx, `SELECT group_id,position_q,position_r FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user).Scan(&ownTeam, &own.Q, &own.R); err != nil {
		return err
	}
	if hp <= 0 {
		return ErrCraftConsumable
	}
	if target != user {
		distance, e := hexgrid.Distance(pos, own)
		if e != nil || distance != 1 {
			return ErrCraftConsumable
		}
		visible, e := HexPositionVisibleTx(ctx, c.Tx, c.InstanceID, user, pos)
		if e != nil {
			return e
		}
		if !visible {
			return ErrCraftConsumable
		}
		edgePos, edgeDirection := own, 0
		for direction := 0; direction < 6; direction++ {
			next, _ := hexgrid.Neighbor(own, direction)
			if next == pos {
				edgeDirection = direction
				break
			}
		}
		if edgeDirection >= 3 {
			edgePos = pos
			edgeDirection -= 3
		}
		var blocked bool
		if e = c.Tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM match_hex_edges WHERE instance_id=$1 AND q=$2 AND r=$3 AND direction=$4)`, c.InstanceID, edgePos.Q, edgePos.R, edgeDirection).Scan(&blocked); e != nil {
			return e
		}
		if blocked {
			return ErrCraftConsumable
		}
		if r.Effect == "poison" {
			if team == ownTeam {
				return ErrCraftConsumable
			}
		} else if r.Code != "healing_scroll" || team != ownTeam {
			return ErrCraftConsumable
		}
	}
	targetState, targetTurn, err := readCraftStatusTx(ctx, c.Tx, c.InstanceID, target)
	if err != nil {
		return err
	}
	scroll := r.Category == "scroll"
	switch r.Effect {
	case "heal":
		if scroll {
			targetState.Heal = append(targetState.Heal, CraftPeriodic{10, 3, user})
		} else {
			if hp >= maxHP {
				return ErrCraftConsumable
			}
			if _, err = c.Tx.ExecContext(ctx, `UPDATE match_players SET health=LEAST(max_health,health+$3) WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user, percentHP(maxHP, 40)); err != nil {
				return err
			}
		}
	case "poison":
		if targetState.ImmuneUntil > targetTurn {
			return ErrCraftConsumable
		}
		immediate, tick, count := 15, 10, 1
		if scroll {
			immediate, tick, count = 10, 5, 2
		}
		targetState.Poison = append(targetState.Poison, CraftPeriodic{tick, count, user})
		if err = c.AuraDamageTx(ctx, target, user, percentHP(maxHP, immediate), "poison"); err != nil {
			return err
		}
		if percentHP(maxHP, immediate) >= hp {
			targetState = CraftStatus{}
		}
	case "critical":
		if s.Critical != nil && (s.Critical.Charges > 0 || s.Critical.Until > turn) {
			return ErrCraftConsumable
		}
		if scroll {
			s.Critical = &CraftBuff{Kind: "scroll", Until: turn + 2}
		} else {
			s.Critical = &CraftBuff{Kind: "elixir", Charges: 2}
		}
	case "vampirism":
		if s.Vampirism != nil && s.Vampirism.Until > turn {
			return ErrCraftConsumable
		}
		duration, kind := 1, "elixir"
		if scroll {
			duration, kind = 2, "scroll"
		}
		s.Vampirism = &CraftBuff{Kind: kind, Until: turn + duration}
	case "speed":
		if s.Speed != nil && s.Speed.Until > turn {
			return ErrCraftConsumable
		}
		amount, duration, kind := 4, 1, "elixir"
		if scroll {
			amount, duration, kind = 2, 3, "scroll"
			s.SpeedTicks = 2
		}
		s.Speed = &CraftBuff{Kind: kind, Until: turn + duration}
		if _, err = c.Tx.ExecContext(ctx, `UPDATE match_players SET mp=LEAST(max_mp,mp+$3) WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user, amount); err != nil {
			return err
		}
	case "cleanse":
		if !scroll && s.ImmuneUntil > turn {
			return ErrCraftConsumable
		}
		targetState.Poison = nil
		if !scroll {
			targetState.ImmuneUntil = turn + 5
		}
		if _, err = c.Tx.ExecContext(ctx, `UPDATE match_players SET hex_mp_drain=false,hex_armor_break_stacks=0,hex_armor_break_owner=0,hex_armor_break_turns=0 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user); err != nil {
			return err
		}
	default:
		return ErrCraftConsumable
	}
	if target == user {
		// Healing/cleansing changed recipient state; offensive buffs changed caster state.
		if r.Effect == "heal" || r.Effect == "cleanse" {
			s.Heal, s.Poison, s.ImmuneUntil = targetState.Heal, targetState.Poison, targetState.ImmuneUntil
		}
	} else {
		if err = writeCraftStatusTx(ctx, c.Tx, c.InstanceID, target, targetState); err != nil {
			return err
		}
	}
	s.Cooldowns[r.Effect] = turn + 2
	return writeCraftStatusTx(ctx, c.Tx, c.InstanceID, user, s)
}

// Called once by the existing actual personal-turn transition, after budget refill.
func (c *MatchCommand) TickCraftStatusesTx(ctx context.Context, user int) error {
	s, turn, err := readCraftStatusTx(ctx, c.Tx, c.InstanceID, user)
	if err != nil {
		return err
	}
	var hp, maxHP int
	if err = c.Tx.QueryRowContext(ctx, `SELECT health,max_health FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user).Scan(&hp, &maxHP); err != nil {
		return err
	}
	if hp <= 0 {
		return nil
	}
	heal := 0
	remainingHeal := []CraftPeriodic{}
	for _, tick := range s.Heal {
		heal += percentHP(maxHP, tick.Percent)
		tick.Remaining--
		if tick.Remaining > 0 {
			remainingHeal = append(remainingHeal, tick)
		}
	}
	s.Heal = remainingHeal
	if heal > 0 {
		if _, err = c.Tx.ExecContext(ctx, `UPDATE match_players SET health=LEAST(max_health,health+$3) WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user, heal); err != nil {
			return err
		}
	}
	if s.Speed != nil && s.Speed.Kind == "scroll" && s.SpeedTicks > 0 {
		if _, err = c.Tx.ExecContext(ctx, `UPDATE match_players SET mp=LEAST(max_mp,mp+2) WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user); err != nil {
			return err
		}
		s.SpeedTicks--
	}
	remainingPoison := []CraftPeriodic{}
	for _, tick := range s.Poison {
		if s.ImmuneUntil <= turn {
			if err = c.AuraDamageTx(ctx, user, tick.Source, percentHP(maxHP, tick.Percent), "poisonTick"); err != nil {
				return err
			}
		}
		tick.Remaining--
		if tick.Remaining > 0 {
			remainingPoison = append(remainingPoison, tick)
		}
		var alive bool
		if err = c.Tx.QueryRowContext(ctx, `SELECT health>0 FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user).Scan(&alive); err != nil {
			return err
		}
		if !alive {
			return writeCraftStatusTx(ctx, c.Tx, c.InstanceID, user, CraftStatus{})
		}
	}
	s.Poison = remainingPoison
	if s.Critical != nil && s.Critical.Kind == "scroll" && s.Critical.Until <= turn {
		s.Critical = nil
	}
	if s.Vampirism != nil && s.Vampirism.Until <= turn {
		s.Vampirism = nil
	}
	if s.Speed != nil && s.Speed.Until <= turn {
		s.Speed = nil
		s.SpeedTicks = 0
	}
	return writeCraftStatusTx(ctx, c.Tx, c.InstanceID, user, s)
}

// Only successful main attacks enter this helper. Bonuses share one critical roll;
// the best active critical damage bonus wins, avoiding multiplying Ranger crit twice.
func (c *MatchCommand) CraftCriticalTx(ctx context.Context, user, damage, rangerChance int, roll func(int) bool) (int, bool, error) {
	if damage <= 0 {
		return damage, false, nil
	}
	s, turn, err := readCraftStatusTx(ctx, c.Tx, c.InstanceID, user)
	if err != nil {
		return 0, false, err
	}
	chance, bonus := rangerChance, 42
	if s.Critical != nil {
		if s.Critical.Charges > 0 {
			chance, bonus = 100, 50
			s.Critical.Charges--
			if s.Critical.Charges == 0 {
				s.Critical = nil
			}
		} else if s.Critical.Until > turn {
			chance += 50
		}
	}
	if chance > 100 {
		chance = 100
	}
	crit := chance >= 100 || chance > 0 && roll(chance)
	if err = writeCraftStatusTx(ctx, c.Tx, c.InstanceID, user, s); err != nil {
		return 0, false, err
	}
	if crit {
		damage = damage * (100 + bonus) / 100
	}
	return damage, crit, nil
}
func (c *MatchCommand) CraftLifestealTx(ctx context.Context, user, damage int, classProc bool, roll func(int) bool) (int, error) {
	if damage <= 0 {
		return 0, nil
	}
	s, turn, err := readCraftStatusTx(ctx, c.Tx, c.InstanceID, user)
	if err != nil {
		return 0, err
	}
	percent := 0
	if classProc {
		percent = 50
	}
	if s.Vampirism != nil && s.Vampirism.Until > turn {
		if s.Vampirism.Kind == "elixir" {
			percent += 70
		} else if roll(55) {
			percent += 80
		}
	}
	if percent > 100 {
		percent = 100
	}
	return damage * percent / 100, nil
}

// Public documentation uses this to show the actual restriction beside poison recipes.
func craftEffectDescription(r CraftRecipe) string {
	if r.Category == "equipment" {
		return "Rare equipment crafted in the Artisans' Guild. Durability: 150."
	}
	if r.Category == "processed" {
		return "Crafting component for other professions and finished items."
	}
	descriptions := map[string]string{
		"healing_potion":      "Restore 40% max HP immediately.",
		"toxic_elixir":        "Enemy player within 1 HEX: 15% max HP damage now and 10% at their next personal turn. PvP only.",
		"critical_potion":     "Next 2 successful main attacks are critical with +50% damage bonus.",
		"vampirism_elixir":    "70% lifesteal until your next personal turn. Combined lifesteal is capped at 100% of actual HP damage.",
		"speed_potion":        "Restore 2 MP now, capped at Max MP.",
		"alchemical_antidote": "Remove poison, armor break and MP drain; immunity against these effects for 5 personal turns, including this turn.",
		"healing_scroll":      "Self or adjacent ally: restore 10% max HP at the start of their next 3 personal turns. Stacks.",
		"poison_scroll":       "Enemy player within 1 HEX: 10% max HP damage now and 5% at the start of their next 2 personal turns. Stacks. PvP only.",
		"critical_scroll":     "+50 percentage points to critical chance, +42% critical damage bonus, this and next personal turn. Critical bonuses do not multiply.",
		"vampirism_scroll":    "55% chance to heal 80% of actual main-hit HP damage, this and next personal turn. Combined lifesteal cap: 100%.",
		"speed_scroll":        "Restore 1 MP now and at the start of your next 2 personal turns, capped at Max MP.",
		"purification_scroll": "Remove poison, armor break and MP drain without granting immunity.",
	}
	return descriptions[r.Code]
}
