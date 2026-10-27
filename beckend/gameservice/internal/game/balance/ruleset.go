package balance

import (
	"fmt"
	"strings"
)

type CombatRuleset string

const (
	Legacy          CombatRuleset = "legacy"
	ExperimentalV34 CombatRuleset = "experimental_v34"
)

func ParseCombatRuleset(value string) (CombatRuleset, error) {
	switch strings.TrimSpace(value) {
	case string(Legacy):
		return Legacy, nil
	case "", string(ExperimentalV34):
		return ExperimentalV34, nil
	default:
		return "", fmt.Errorf("invalid NACHALO_COMBAT_RULESET %q: expected legacy or experimental_v34", value)
	}
}

func RequirePlayableRuleset(ruleset CombatRuleset) error {
	if ruleset != Legacy && ruleset != ExperimentalV34 {
		return fmt.Errorf("unknown combat ruleset %q", ruleset)
	}
	return nil
}

// PARAM semantics were read from the supplied v20 -> v22 -> v34 source chain.
// This is an experimental Go adaptation, not Python RNG/arena parity.
type V34SlotBudget struct {
	HP      int `json:"hp"`
	Attack  int `json:"attack"`
	Defense int `json:"defense"`
	Reflex  int `json:"reflex"`
}

type V34ResearchConfig struct {
	RareSlotReflex          map[string]int            `json:"rare_slot_reflex,omitempty"`
	JewelryReflex           map[string]map[string]int `json:"jewelry_reflex,omitempty"`
	MysticMagicThroughBlock bool                      `json:"mystic_magic_through_block,omitempty"`
	EquipmentModel          string                    `json:"equipment_model,omitempty"`
	SlotBudgets             map[string]V34SlotBudget  `json:"slot_budgets,omitempty"`
	BaseHP                  [4]int                    `json:"hp"` // Guardian, Berserker, Ranger, Mystic.
	BaseAttack              [4]int                    `json:"atk"`
	BaseDefense             [4]int                    `json:"def"`
	HatredPercent           int                       `json:"hatred_percent"`
	FlatDefenseBasisPoints  int                       `json:"flat_def_basis_points"`
	Factors                 map[string]float64        `json:"fx"`
	Mechanics               map[string]float64        `json:"mechanics"`
	GearHP                  int                       `json:"gear_hp"`
	GearAttack              int                       `json:"gear_atk"`
	GearDefense             int                       `json:"gear_def"`
	GearMain                int                       `json:"gear_main"`
}

func ExperimentalV34Config() V34ResearchConfig {
	return V34ResearchConfig{
		EquipmentModel:          "slots_v3_reflex",
		MysticMagicThroughBlock: true,
		RareSlotReflex:          map[string]int{"helmet": 2, "chest": 2, "pants": 1, "boots": 2, "gloves": 1, "main_hand": 1, "off_hand": 1},
		JewelryReflex:           map[string]map[string]int{"green": {"ring": 1, "amulet": 2}, "blue": {"ring": 2, "amulet": 3}},
		SlotBudgets: map[string]V34SlotBudget{
			"helmet": {HP: 2, Reflex: 1}, "chest": {HP: 6, Defense: 1, Reflex: 1}, "pants": {HP: 4, Defense: 1, Reflex: 1},
			"boots": {HP: 2, Reflex: 1}, "gloves": {HP: 2, Reflex: 1}, "main_hand": {HP: 2, Attack: 3, Reflex: 1}, "off_hand": {HP: 2, Defense: 1, Reflex: 1},
		},
		BaseHP:      [4]int{218, 176, 146, 134},
		BaseAttack:  [4]int{12, 17, 21, 24},
		BaseDefense: [4]int{16, 7, 9, 4},
		// Task specifies exactly +16%; research JSON's hate=16.0502 is not used.
		HatredPercent: 16, FlatDefenseBasisPoints: 3044,
		Factors:   map[string]float64{"block": 0.74, "counter": 0.5938, "heal": 0.6215, "followup": 0.905, "crit": 1.3487, "armor_break": 1.3858, "overload": 1.4585, "aura": 0.5692, "magic": 1.6393},
		Mechanics: map[string]float64{"opening": 0.1811, "dodge": 0.0827, "arcane": 0.0088, "glance": 0.1469, "surge": 0.1644, "close_ward": 0.0278, "guardian_melee_resist": 0, "mystic_ranged_resist": 0.0031, "ranger_armor_focus": 0.0143},
		GearHP:    20, GearAttack: 3, GearDefense: 3, GearMain: 7,
	}
}

// Exact rational DEF calculation; nearest integer, ties to even (Python round).
// Positive-ATK results below one clamp to one, including negative intermediate values.
func V34DefenseDamage(attack, defense int, configs ...*V34ResearchConfig) int {
	if attack <= 0 {
		return 0
	}
	if defense < 0 {
		defense = 0
	}
	k := 3044
	if len(configs) > 0 && configs[0] != nil {
		k = configs[0].FlatDefenseBasisPoints
	}
	value := int64(attack)*10000 - int64(k)*int64(defense)
	if value <= 10000 {
		return 1
	}
	whole, fraction := value/10000, value%10000
	if fraction > 5000 || fraction == 5000 && whole%2 != 0 {
		whole++
	}
	return int(whole)
}

// The sole counter-class bonus, intended after the resolved positive main hit.
// Use floor once here; callers must never apply it to counters or bonus damage.
func V34HatredDamage(damage int, attacker, defender string, configs ...*V34ResearchConfig) (int, bool) {
	if damage <= 0 {
		return 0, false
	}
	advantage, known := classAdvantage[attacker]
	if !known || advantage != defender {
		return damage, false
	}
	percent := 16
	if len(configs) > 0 && configs[0] != nil {
		percent = configs[0].HatredPercent
	}
	return int(int64(damage) * int64(100+percent) / 100), true
}
