package repository

import (
	"testing"

	"gameservice/models"
)

func TestApplyQuestArtifactStats(t *testing.T) {
	tests := []struct {
		name       string
		artifact   string
		wantAttack int
		wantDef    int
		wantHP     int
		wantEnergy int
	}{
		{"axe", ArtifactBerserkerAxe, 12, 6, 100, 100},
		{"crown", ArtifactCrownEnlightenment, 10, 10, 85, 105},
		{"gloves", ArtifactGlovesOfPrecision, 11, 7, 90, 100},
		{"shield", ArtifactGuardianShield, 10, 12, 100, 100},
		{"sword", ArtifactKnightSword, 11, 11, 80, 100},
		{"ring", ArtifactRingOfWisdom, 10, 10, 100, 75},
		{"titan", ArtifactTitanBreastplate, 7, 12, 100, 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			player := models.PlayerResponse{
				Attack:    10,
				Defense:   10,
				Health:    100,
				MaxHealth: 100,
				Energy:    100,
				MaxEnergy: 100,
			}
			applyQuestArtifactStats(&player, QuestArtifactEffect{Name: tc.artifact})
			if player.Attack != tc.wantAttack ||
				player.Defense != tc.wantDef ||
				player.MaxHealth != tc.wantHP ||
				player.MaxEnergy != tc.wantEnergy {
				t.Fatalf("unexpected stats: %+v", player)
			}
		})
	}
}

func TestApplyQuestArtifactStatsClampsCurrentPools(t *testing.T) {
	player := models.PlayerResponse{
		Attack:    1,
		Defense:   1,
		Health:    100,
		MaxHealth: 100,
		Energy:    100,
		MaxEnergy: 100,
	}
	applyQuestArtifactStats(&player, QuestArtifactEffect{Name: ArtifactKnightSword})
	if player.Health != 80 || player.MaxHealth != 80 {
		t.Fatalf("health was not clamped: %+v", player)
	}
}
