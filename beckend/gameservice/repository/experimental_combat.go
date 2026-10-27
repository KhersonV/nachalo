package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"gameservice/internal/game/balance"
)

func DecodeMatchCombatConfig(ruleset balance.CombatRuleset, raw []byte) (*balance.V34ResearchConfig, error) {
	if ruleset != balance.ExperimentalV34 {
		return nil, nil
	}
	var config balance.V34ResearchConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	for i := range config.BaseHP {
		if config.BaseHP[i] <= 0 || config.BaseAttack[i] <= 0 || config.BaseDefense[i] < 0 {
			return nil, fmt.Errorf("invalid frozen v34 base stats")
		}
	}
	for _, key := range []string{"block", "counter", "heal", "followup", "crit", "armor_break", "overload", "aura", "magic"} {
		if value, ok := config.Factors[key]; !ok || value < 0 {
			return nil, fmt.Errorf("invalid frozen v34 factor %s", key)
		}
	}
	for _, key := range []string{"opening", "dodge", "arcane", "glance", "surge", "close_ward", "guardian_melee_resist", "mystic_ranged_resist", "ranger_armor_focus"} {
		if value, ok := config.Mechanics[key]; !ok || value < 0 || value > 1 {
			return nil, fmt.Errorf("invalid frozen v34 mechanic %s", key)
		}
	}
	if config.FlatDefenseBasisPoints < 0 || config.HatredPercent < 0 || config.GearHP < 0 || config.GearAttack < 0 || config.GearDefense < 0 || config.GearMain < 0 {
		return nil, fmt.Errorf("invalid frozen v34 configuration")
	}
	if config.EquipmentModel != "" && config.EquipmentModel != "slots_v2" && config.EquipmentModel != "slots_v3_reflex" {
		return nil, fmt.Errorf("unknown frozen equipment model %q", config.EquipmentModel)
	}
	if config.EquipmentModel == "slots_v2" || config.EquipmentModel == "slots_v3_reflex" {
		total := balance.V34SlotBudget{}
		for _, slot := range []string{"helmet", "chest", "pants", "boots", "gloves", "main_hand", "off_hand"} {
			b, ok := config.SlotBudgets[slot]
			if !ok || b.HP < 0 || b.Attack < 0 || b.Defense < 0 || b.Reflex < 0 {
				return nil, fmt.Errorf("invalid frozen slot budget %s", slot)
			}
			total.HP += b.HP
			total.Attack += b.Attack
			total.Defense += b.Defense
			total.Reflex += b.Reflex
		}
		if total.HP != config.GearHP || total.Attack != config.GearAttack || total.Defense != config.GearDefense || total.Reflex != config.GearMain {
			return nil, fmt.Errorf("frozen slot budgets do not match gear totals")
		}
	}
	if config.EquipmentModel == "slots_v3_reflex" {
		sum := 0
		for _, slot := range []string{"helmet", "chest", "pants", "boots", "gloves", "main_hand", "off_hand"} {
			v, ok := config.RareSlotReflex[slot]
			if !ok || v < 0 {
				return nil, fmt.Errorf("invalid rare Reflex slot %s", slot)
			}
			sum += v
		}
		if sum != 10 {
			return nil, fmt.Errorf("invalid rare Reflex total")
		}
		for _, rarity := range []string{"green", "blue"} {
			for _, slot := range []string{"ring", "amulet"} {
				if v, ok := config.JewelryReflex[rarity][slot]; !ok || v < 0 {
					return nil, fmt.Errorf("invalid jewelry Reflex %s/%s", rarity, slot)
				}
			}
		}
	}
	return &config, nil
}

func ReadMatchCombatRulesTx(ctx context.Context, tx storageQuerier, instance string) (balance.CombatRuleset, *balance.V34ResearchConfig, error) {
	var ruleset balance.CombatRuleset
	var raw []byte
	if err := tx.QueryRowContext(ctx, `SELECT combat_ruleset,combat_config FROM matches WHERE instance_id=$1`, instance).Scan(&ruleset, &raw); err != nil {
		return ruleset, nil, err
	}
	if err := balance.RequirePlayableRuleset(ruleset); err != nil {
		return ruleset, nil, err
	}
	config, err := DecodeMatchCombatConfig(ruleset, raw)
	return ruleset, config, err
}

func (c *MatchCommand) RandomUnit() float64 {
	value := balance.V34RandomUnit(c.RandomSeed, c.RandomCounter)
	c.RandomCounter++
	return value
}

func (c *MatchCommand) RollResearchChance(chance float64) bool {
	return float64(int(c.RandomUnit()*100)) < chance
}

// Only the caller's existing transaction updates the buffer. HP, deaths, rewards
// and idempotency stay in the ordinary attack/aura commands.
func (c *MatchCommand) ResolveResearchDamageTx(ctx context.Context, target int, hit balance.V34Incoming) (balance.V34Damage, error) {
	if c.CombatConfig == nil {
		return balance.V34Damage{Damage: hit.Damage}, nil
	}
	if err := c.Tx.QueryRowContext(ctx, `SELECT combat_started,combat_magic_shield FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, target).Scan(&hit.Started, &hit.Shield); err != nil {
		return balance.V34Damage{}, err
	}
	result := balance.ResolveV34Damage(c.CombatConfig, hit, c.RandomUnit)
	if result.Shield != hit.Shield {
		if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET combat_magic_shield=$3 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, target, result.Shield); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (c *MatchCommand) StartResearchPersonalTurnTx(ctx context.Context, user int) error {
	if c.CombatConfig == nil {
		return nil
	}
	var class string
	var hp int
	if err := c.Tx.QueryRowContext(ctx, `SELECT character_type,max_health FROM match_players WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user).Scan(&class, &hp); err != nil {
		return err
	}
	shield := 0
	if class == "mystic" {
		shield = balance.ResearchRound(float64(hp) * c.CombatConfig.Mechanics["arcane"])
	}
	_, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET combat_started=true,combat_magic_shield=$3 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, user, shield)
	return err
}
