package handlers

import "gameservice/game"

// Combat steps contain authoritative HP after each hit. Convert them to
// effective HP loss for durable statistics, including overkill and lifesteal.
func hexDamageEventsFromSteps(steps []CombatStep, effects []CombatEffect, initial map[CombatTargetRef]int) []game.DamageEvent {
	health := make(map[CombatTargetRef]int, len(initial))
	for actor, hp := range initial {
		health[actor] = hp
	}
	heals := map[CombatTargetRef]int{}
	for _, effect := range effects {
		if effect.Kind == "lifesteal" && effect.Succeeded && effect.Source != nil && effect.Amount > 0 {
			heals[*effect.Source] += effect.Amount
		}
	}
	events := []game.DamageEvent{}
	for _, step := range steps {
		if step.Source == nil || step.Damage <= 0 {
			continue
		}
		before, ok := health[step.Target]
		if !ok {
			continue
		}
		after := step.TargetHPAfter
		if after < 0 {
			after = 0
		}
		actual := before - after
		if actual < 0 {
			actual = 0
		}
		if actual > step.Damage {
			actual = step.Damage
		}
		health[step.Target] = after
		if actual > 0 {
			source := game.ResultSourcePlayer
			if step.Source.Type == CombatActorMonster {
				source = game.ResultSourceMonster
			}
			events = append(events, game.DamageEvent{SourceType: source, DealerID: step.Source.ID, TargetType: string(step.Target.Type), TargetID: step.Target.ID, Amount: actual})
		}
		if step.Kind == "hit" {
			if heal := heals[*step.Source]; heal > 0 {
				health[*step.Source] += heal
				delete(heals, *step.Source)
			}
		}
	}
	return events
}
