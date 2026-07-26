package repository

import (
	"database/sql"
	"fmt"

	"gameservice/models"
)

const (
	ArtifactBerserkerAxe       = "berserker_axe"
	ArtifactBootsOfStealth     = "boots_of_stealth"
	ArtifactCrownEnlightenment = "crown_of_enlightenment"
	ArtifactDragonEye          = "dragon_eye"
	ArtifactFireAmulet         = "fire_amulet"
	ArtifactGlovesOfPrecision  = "gloves_of_precision"
	ArtifactGuardianShield     = "guardian_shield"
	ArtifactKnightSword        = "knight_sword"
	ArtifactRingOfWisdom       = "ring_of_wisdom"
	ArtifactTitanBreastplate   = "titan_breastplate"
)

type QuestArtifactEffect struct {
	ID   int
	Name string
}

// GetPlayerQuestArtifactEffect returns an effect only when the player carries
// the one quest artifact selected for this match. Regular artifact loot does
// not affect combat stats and effects never stack.
func GetPlayerQuestArtifactEffect(instanceID string, userID int) (QuestArtifactEffect, error) {
	// Unit tests exercise combat handlers with repository.DB intentionally nil.
	// In that isolated mode there is no match inventory and therefore no
	// artifact effect to apply.
	if DB == nil {
		return QuestArtifactEffect{}, nil
	}

	const query = `
		SELECT a.id, a.name
		FROM matches m
		JOIN artifacts a ON a.id = m.quest_artifact_id
		JOIN inventory_items ii
		  ON ii.instance_id = m.instance_id
		 AND ii.user_id = $2
		 AND ii.item_type = 'artifact'
		 AND ii.item_id = m.quest_artifact_id
		 AND ii.item_count > 0
		WHERE m.instance_id = $1
		LIMIT 1`

	var effect QuestArtifactEffect
	err := DB.QueryRow(query, instanceID, userID).Scan(&effect.ID, &effect.Name)
	if err == sql.ErrNoRows {
		return QuestArtifactEffect{}, nil
	}
	if err != nil {
		return QuestArtifactEffect{}, fmt.Errorf("GetPlayerQuestArtifactEffect: %w", err)
	}
	return effect, nil
}

func applyQuestArtifactStats(player *models.PlayerResponse, effect QuestArtifactEffect) {
	if player == nil {
		return
	}

	switch effect.Name {
	case ArtifactBerserkerAxe:
		player.Attack += 2
		player.Defense -= 4
	case ArtifactCrownEnlightenment:
		player.MaxEnergy += 5
		player.MaxHealth -= 15
	case ArtifactGlovesOfPrecision:
		player.Attack++
		player.Defense -= 3
		player.MaxHealth -= 10
	case ArtifactGuardianShield:
		player.Defense += 2
	case ArtifactKnightSword:
		player.Attack++
		player.Defense++
		player.MaxHealth -= 20
	case ArtifactRingOfWisdom:
		player.MaxEnergy -= 25
	case ArtifactTitanBreastplate:
		player.Attack -= 3
		player.Defense += 2
	}

	if player.Attack < 0 {
		player.Attack = 0
	}
	if player.Defense < 0 {
		player.Defense = 0
	}
	if player.MaxHealth < 1 {
		player.MaxHealth = 1
	}
	if player.MaxEnergy < 1 {
		player.MaxEnergy = 1
	}
	if player.Health > player.MaxHealth {
		player.Health = player.MaxHealth
	}
	if player.Energy > player.MaxEnergy {
		player.Energy = player.MaxEnergy
	}
}

func ApplyQuestArtifactStats(instanceID string, player *models.PlayerResponse) error {
	if player == nil {
		return nil
	}
	effect, err := GetPlayerQuestArtifactEffect(instanceID, player.UserID)
	if err != nil {
		return err
	}
	applyQuestArtifactStats(player, effect)
	return nil
}
