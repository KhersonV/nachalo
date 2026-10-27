package handlers

import (
	"context"
	"gameservice/internal/game/balance"
	"gameservice/repository"
)

func combatRoll(c *repository.MatchCommand) func(int) bool {
	if c.CombatConfig == nil {
		return rollReflexProc
	}
	return func(chance int) bool { return c.RollResearchChance(float64(chance)) }
}

func incomingCombatTx(ctx context.Context, c *repository.MatchCommand, source, target CombatTargetRef, attacker, defender stats, health, damage int, kind string, distance int) (attackResult, []CombatEffect, error) {
	if c.CombatConfig == nil {
		return applyFlatDamage(health, damage), nil, nil
	}
	hit := balance.V34Incoming{Kind: kind, Damage: damage, Attacker: attacker.CharacterType, Defender: defender.CharacterType, Distance: distance, Attack: attacker.Attack, Defense: defender.Defense, Started: true}
	var result balance.V34Damage
	var err error
	if target.Type == CombatActorPlayer {
		result, err = c.ResolveResearchDamageTx(ctx, target.ID, hit)
	} else {
		result = balance.ResolveV34Damage(c.CombatConfig, hit, c.RandomUnit)
	}
	if err != nil {
		return attackResult{}, nil, err
	}
	return applyFlatDamage(health, result.Damage), researchEffects(nil, result.Effects, source, target), nil
}

// A physical block and a dodge are exclusive outcomes. A blocked v34 main
// attack still connects for magic; an actual dodge cannot advance the series.
// Missing/false flags preserve the exact behavior of older saved matches.
func mysticAttackConnects(cfg *balance.V34ResearchConfig, triggered, blocked bool, effects []CombatEffect) bool {
	if cfg == nil || !cfg.MysticMagicThroughBlock {
		return triggered
	}
	if blocked {
		return true
	}
	for _, effect := range effects {
		if effect.Kind == "dodge" && effect.Succeeded {
			return false
		}
	}
	return triggered
}
