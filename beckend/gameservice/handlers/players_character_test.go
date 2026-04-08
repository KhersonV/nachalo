package handlers

import "testing"

func TestResolveCharacterTemplateKnownTypes(t *testing.T) {
    cases := []struct {
        characterType string
        health        int
        attack        int
        defense       int
        maxEnergy     int
    }{
        {characterType: "guardian", health: 130, attack: 9, defense: 8, maxEnergy: 90},
        {characterType: "berserker", health: 100, attack: 14, defense: 3, maxEnergy: 100},
        {characterType: "ranger", health: 92, attack: 11, defense: 4, maxEnergy: 105},
        {characterType: "mystic", health: 95, attack: 10, defense: 4, maxEnergy: 125},
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
        if template.MaxEnergy != tc.maxEnergy {
            t.Fatalf("%s: expected maxEnergy %d, got %d", tc.characterType, tc.maxEnergy, template.MaxEnergy)
        }
    }
}

func TestResolveCharacterTemplateFallsBackToAdventurer(t *testing.T) {
    template := resolveCharacterTemplate("unknown-type")
    if template.CharacterType != defaultCharacterType {
        t.Fatalf("expected fallback type %s, got %s", defaultCharacterType, template.CharacterType)
    }
    if template.Health != 100 || template.Attack != 10 || template.Defense != 5 {
        t.Fatalf("unexpected fallback template: %+v", template)
    }
}
