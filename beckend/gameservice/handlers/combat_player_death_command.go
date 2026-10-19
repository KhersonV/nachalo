package handlers

import (
	"context"

	"gameservice/game"
	"gameservice/repository"
)

// finishCombatPlayerDeathTx is called after the damage and surviving actors'
// vitals have been written. It shares the attack's MatchCommand and never
// publishes or changes process-local match state.
func finishCombatPlayerDeathTx(ctx context.Context, c *repository.MatchCommand, victimID int, killerType string, killerID, killingDamage int, pos repository.Position) (repository.PlayerDeathTurnResult, int, bool, error) {
	var turn repository.PlayerDeathTurnResult
	if _, err := c.RecordKillEventTx(ctx, game.KillEvent{SourceType: killerType, KillerID: killerID, VictimType: "player", VictimID: victimID, Damage: killingDamage}); err != nil {
		return turn, 0, false, err
	}
	if err := c.MarkCombatPlayerDeadTx(ctx, victimID); err != nil {
		return turn, 0, false, err
	}
	var err error
	turn, err = c.RemoveCombatPlayerFromTurnOrderTx(ctx, victimID)
	if err != nil {
		return turn, 0, false, err
	}
	// The live combat path cannot use the prepared finalizer while real matches
	// still have combat_results_complete=false. Callers preflight this case.
	if turn.ActiveUserID == 0 {
		return turn, 0, false, repository.ErrLegacyAction
	}
	next, err := c.MatchCombatPlayerTx(ctx, turn.ActiveUserID)
	if err != nil {
		return turn, 0, false, err
	}
	artifact, err := c.QuestArtifactTx(ctx, turn.ActiveUserID)
	if err != nil {
		return turn, 0, false, err
	}
	regen := clampEnergyRegen(resolveCharacterRegen(next.CharacterType) + artifactEnergyRegenBonus(artifact))
	energy := next.Energy + regen
	if energy > next.MaxEnergy {
		energy = next.MaxEnergy
	}
	if err := c.SetCombatPlayerVitalsTx(ctx, turn.ActiveUserID, next.Health, energy); err != nil {
		return turn, 0, false, err
	}
	changed, err := c.ClearCombatPlayerCellTx(ctx, pos)
	if err != nil {
		return turn, 0, false, err
	}
	cell, err := c.CombatMapCellTx(ctx, pos)
	if err != nil {
		return turn, 0, false, err
	}
	for _, event := range []map[string]interface{}{
		{"type": "PLAYER_DEFEATED", "payload": map[string]interface{}{"instanceId": c.InstanceID, "userId": victimID, "updatedCell": serialiseUpdatedCell(*cell)}},
		{"type": "SET_ACTIVE_USER", "payload": map[string]interface{}{"instanceId": c.InstanceID, "instance_id": c.InstanceID, "active_user": turn.ActiveUserID, "energy": energy, "turnNumber": turn.TurnNumber}},
		{"type": "TURN_PASSED", "payload": map[string]interface{}{"instanceId": c.InstanceID, "instance_id": c.InstanceID, "userId": turn.ActiveUserID, "active_user": turn.ActiveUserID, "turnNumber": turn.TurnNumber}},
	} {
		if err := c.QueueEvent(event); err != nil {
			return turn, 0, false, err
		}
	}
	return turn, energy, changed, nil
}
