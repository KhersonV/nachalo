package handlers

import (
	"encoding/json"
	"fmt"
	"sync"

	"gameservice/repository"
)

const (
	artifactDragonEyeTurnDamage = 3
	artifactFireBonusDamage     = 2
	artifactFireBacklashDamage  = 3
)

type artifactTurnMoveState struct {
	Turn int
	Used bool
}

var artifactTurnMoves sync.Map

func playerQuestArtifact(instanceID string, userID int) (repository.QuestArtifactEffect, error) {
	return repository.GetPlayerQuestArtifactEffect(instanceID, userID)
}

func artifactAttackCostBonus(effect repository.QuestArtifactEffect) int {
	switch effect.Name {
	case repository.ArtifactCrownEnlightenment, repository.ArtifactGuardianShield:
		return 1
	default:
		return 0
	}
}

func artifactEnergyRegenBonus(effect repository.QuestArtifactEffect) int {
	switch effect.Name {
	case repository.ArtifactBootsOfStealth, repository.ArtifactGuardianShield:
		return -3
	case repository.ArtifactRingOfWisdom:
		return 1
	default:
		return 0
	}
}

func artifactMoveCostBonus(instanceID string, userID int, turn int, effect repository.QuestArtifactEffect) int {
	switch effect.Name {
	case repository.ArtifactBootsOfStealth:
		key := fmt.Sprintf("%s:%d", instanceID, userID)
		if value, ok := artifactTurnMoves.Load(key); ok {
			state := value.(artifactTurnMoveState)
			if state.Turn == turn && state.Used {
				return 0
			}
		}
		return -1
	case repository.ArtifactTitanBreastplate:
		return 2
	default:
		return 0
	}
}

func markArtifactMoveUsed(instanceID string, userID int, turn int) {
	artifactTurnMoves.Store(
		fmt.Sprintf("%s:%d", instanceID, userID),
		artifactTurnMoveState{Turn: turn, Used: true},
	)
}

func clampEnergyRegen(regen int) int {
	if regen < 0 {
		return 0
	}
	return regen
}

func nonLethalArtifactDamage(health int, damage int) (newHealth int, actualDamage int) {
	if health <= 1 || damage <= 0 {
		return health, 0
	}
	newHealth = health - damage
	if newHealth < 1 {
		newHealth = 1
	}
	return newHealth, health - newHealth
}

func broadcastArtifactDamage(instanceID string, userID int, kind string, damage int, healthAfter int) {
	if damage <= 0 {
		return
	}
	ref := CombatTargetRef{ID: userID, Type: CombatActorPlayer}
	message := CombatExchangeMessage{
		Type: "COMBAT_EXCHANGE",
		Payload: CombatExchangePayload{
			InstanceID:   instanceID,
			ExchangeID:   nextCombatExchangeID(instanceID),
			AttackerID:   userID,
			AttackerType: CombatActorPlayer,
			TargetID:     userID,
			TargetType:   CombatActorPlayer,
			AttackStyle:  AttackStyleMagic,
			Steps: []CombatStep{{
				Kind:          kind,
				Source:        &ref,
				Target:        ref,
				Damage:        damage,
				TargetHPAfter: healthAfter,
			}},
		},
	}
	buf, _ := json.Marshal(message)
	Broadcast(buf)
}

func applyEndTurnArtifactEffect(instanceID string, userID int) error {
	effect, err := playerQuestArtifact(instanceID, userID)
	if err != nil || effect.Name != repository.ArtifactDragonEye {
		return err
	}

	player, err := repository.GetMatchPlayerByID(instanceID, userID)
	if err != nil {
		return err
	}
	newHealth, damage := nonLethalArtifactDamage(player.Health, artifactDragonEyeTurnDamage)
	if damage == 0 {
		return nil
	}
	player.Health = newHealth
	if err := repository.UpdateMatchPlayer(instanceID, player); err != nil {
		return err
	}
	broadcastArtifactDamage(instanceID, userID, "artifactCurse", damage, newHealth)
	sendUpdatePlayerWS(instanceID, userID)
	return nil
}
