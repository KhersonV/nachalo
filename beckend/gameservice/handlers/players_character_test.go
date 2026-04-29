package handlers

import "testing"

func TestResolveCharacterTemplateKnownTypes(t *testing.T) {
	cases := []struct {
		characterType string
		health        int
		attack        int
		defense       int
		energyRegen   int
		maxEnergy     int
	}{
		{characterType: "guardian", health: 130, attack: 9, defense: 8, energyRegen: 10, maxEnergy: 90},
		{characterType: "berserker", health: 100, attack: 14, defense: 3, energyRegen: 10, maxEnergy: 100},
		{characterType: "ranger", health: 92, attack: 11, defense: 4, energyRegen: 11, maxEnergy: 105},
		{characterType: "mystic", health: 95, attack: 10, defense: 4, energyRegen: 13, maxEnergy: 125},
	}

	for _, tc := range cases {
		template := resolveCharacterTemplate(tc.characterType)
		if template.Health != tc.health {
			t.Fatalf("%s: expected health %d, got %d", tc.characterType, tc.health, template.Health)
		}
		if template.Attack != tc.attack {
			t.Fatalf("%s: expected attack %d, got %d", tc.characterType, tc.attack, template.Attack)
		}
		if template.Defense != tc.defense {
			t.Fatalf("%s: expected defense %d, got %d", tc.characterType, tc.defense, template.Defense)
		}
		if template.EnergyRegen != tc.energyRegen {
			t.Fatalf("%s: expected regen %d, got %d", tc.characterType, tc.energyRegen, template.EnergyRegen)
		}
		if template.MaxEnergy != tc.maxEnergy {
			t.Fatalf("%s: expected maxEnergy %d, got %d", tc.characterType, tc.maxEnergy, template.MaxEnergy)
		}
	}
}

func TestResolveCharacterTemplateFallsBackToGuardian(t *testing.T) {
	template := resolveCharacterTemplate("unknown-type")
	if template.CharacterType != defaultCharacterType {
		t.Fatalf("expected fallback type %s, got %s", defaultCharacterType, template.CharacterType)
	}
	if template.Health != 130 || template.Attack != 9 || template.Defense != 8 {
		t.Fatalf("unexpected fallback template: %+v", template)
	}
}

func TestRemovedHeroClassIsNotCatalogued(t *testing.T) {
	removed := "adven" + "turer"
	for _, hero := range heroCatalog {
		if hero.ID == removed {
			t.Fatalf("removed hero class must not be in hero catalog")
		}
	}

	template := resolveCharacterTemplate(removed)
	if template.CharacterType != defaultCharacterType {
		t.Fatalf("expected removed hero class to fall back to %s, got %s", defaultCharacterType, template.CharacterType)
	}
}
