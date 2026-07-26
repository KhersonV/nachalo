package handlers

import (
	"fmt"
	"testing"

	"gameservice/repository"
)

func TestArtifactEnergyAndAttackCosts(t *testing.T) {
	if got := artifactAttackCostBonus(repository.QuestArtifactEffect{Name: repository.ArtifactGuardianShield}); got != 1 {
		t.Fatalf("guardian shield attack cost bonus = %d", got)
	}
	if got := artifactEnergyRegenBonus(repository.QuestArtifactEffect{Name: repository.ArtifactBootsOfStealth}); got != -3 {
		t.Fatalf("boots regen bonus = %d", got)
	}
	if got := artifactEnergyRegenBonus(repository.QuestArtifactEffect{Name: repository.ArtifactRingOfWisdom}); got != 1 {
		t.Fatalf("ring regen bonus = %d", got)
	}
}

func TestBootsDiscountOnlyFirstMoveOfTurn(t *testing.T) {
	instanceID := "artifact-boots-test"
	userID := 42
	key := fmt.Sprintf("%s:%d", instanceID, userID)
	artifactTurnMoves.Delete(key)
	t.Cleanup(func() { artifactTurnMoves.Delete(key) })

	effect := repository.QuestArtifactEffect{Name: repository.ArtifactBootsOfStealth}
	if got := artifactMoveCostBonus(instanceID, userID, 7, effect); got != -1 {
		t.Fatalf("first move bonus = %d", got)
	}
	markArtifactMoveUsed(instanceID, userID, 7)
	if got := artifactMoveCostBonus(instanceID, userID, 7, effect); got != 0 {
		t.Fatalf("second move bonus = %d", got)
	}
	if got := artifactMoveCostBonus(instanceID, userID, 8, effect); got != -1 {
		t.Fatalf("next-turn first move bonus = %d", got)
	}
}

func TestArtifactDamageCannotKillCarrier(t *testing.T) {
	if hp, damage := nonLethalArtifactDamage(2, 3); hp != 1 || damage != 1 {
		t.Fatalf("got hp=%d damage=%d", hp, damage)
	}
	if hp, damage := nonLethalArtifactDamage(1, 3); hp != 1 || damage != 0 {
		t.Fatalf("got hp=%d damage=%d", hp, damage)
	}
}
