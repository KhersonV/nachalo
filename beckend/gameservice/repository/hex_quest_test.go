package repository

import (
	"testing"

	"gameservice/game"
)

func TestHexQuestBarrelGuaranteeAndNoRepeat(t *testing.T) {
	barrel := game.ResourceData{ID: 6, Type: "barrel"}
	quest := game.ResourceData{ID: 71, Type: "quest"}
	ordinary := game.ResourceData{ID: 72, Type: "ordinary"}
	original := hexBarrelRoll
	defer func() { hexBarrelRoll = original }()
	hexBarrelRoll = func(_ game.FullCell, _ []game.ResourceData, artifacts []game.ResourceData) (any, error) {
		for _, artifact := range artifacts {
			if artifact.ID == quest.ID {
				t.Fatal("quest remained in rolls after its first appearance")
			}
		}
		return ordinary, nil
	}
	got, err := rollHexQuestBarrel(barrel, nil, []game.ResourceData{quest, ordinary}, quest.ID, false, 1)
	first, ok := got.(game.ResourceData)
	if err != nil || !ok || first.ID != quest.ID {
		t.Fatalf("last barrel must contain quest artifact: %v, %v", got, err)
	}
	got, err = rollHexQuestBarrel(barrel, nil, []game.ResourceData{quest, ordinary}, quest.ID, true, 1)
	second, ok := got.(game.ResourceData)
	if err != nil || !ok || second.ID != ordinary.ID {
		t.Fatalf("spawned quest must be excluded: %v, %v", got, err)
	}
}
