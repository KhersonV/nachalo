package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gameservice/game"
	"gameservice/models"
	"gameservice/repository"
)

func withEquipmentDropStubs(t *testing.T) {
	t.Helper()

	origRoll := rollEquipmentDrop
	origGrant := grantEquipmentDropItem
	origAvailable := equipmentDropPersistenceAvailable

	t.Cleanup(func() {
		rollEquipmentDrop = origRoll
		grantEquipmentDropItem = origGrant
		equipmentDropPersistenceAvailable = origAvailable
	})

	equipmentDropPersistenceAvailable = func() bool { return true }
}

func TestResolveEquipmentDropChanceOverride(t *testing.T) {
	t.Setenv("EQUIPMENT_DROP_CHANCE_OVERRIDE", "100")
	t.Setenv("EQUIPMENT_DEV_GRANT_ENABLED", "")
	t.Setenv("APP_ENV", "")
	if got := resolveEquipmentDropChancePercent(); got != 100 {
		t.Fatalf("expected override 100, got %d", got)
	}

	t.Setenv("EQUIPMENT_DROP_CHANCE_OVERRIDE", "101")
	if got := resolveEquipmentDropChancePercent(); got != defaultEquipmentDropChancePercent {
		t.Fatalf("out-of-range override should fall back to default, got %d", got)
	}
}

func TestMaybeGrantMonsterEquipmentDropReturnsDropDTO(t *testing.T) {
	withEquipmentDropStubs(t)
	t.Setenv("EQUIPMENT_DROP_CHANCE_OVERRIDE", "")
	t.Setenv("EQUIPMENT_DEV_GRANT_ENABLED", "")
	t.Setenv("APP_ENV", "")

	var gotChance int
	rollEquipmentDrop = func(chancePercent int) bool {
		gotChance = chancePercent
		return true
	}
	grantEquipmentDropItem = func(userID int) (*repository.DroppedEquipment, error) {
		if userID != 77 {
			t.Fatalf("unexpected grant user: %d", userID)
		}
		return &repository.DroppedEquipment{
			InstanceID:   "drop-instance-id",
			OwnerUserID:  77,
			TemplateID:   4,
			TemplateCode: "sagecloth_staff",
			Name:         "Sagecloth Staff",
			Rarity:       "green",
			ImageURL:     "/equipment/mystic/sagecloth/staff.png",
			Slot:         "main_hand",
			ItemType:     "staff",
			ClassID:      "mystic",
			SetCode:      "sagecloth_set",
			SetName:      "Sagecloth Set",
		}, nil
	}

	drops, err := maybeGrantMonsterEquipmentDrop(77)
	if err != nil {
		t.Fatalf("maybeGrantMonsterEquipmentDrop: %v", err)
	}
	if gotChance != defaultEquipmentDropChancePercent {
		t.Fatalf("expected default chance %d, got %d", defaultEquipmentDropChancePercent, gotChance)
	}
	if len(drops) != 1 {
		t.Fatalf("expected one drop, got %+v", drops)
	}
	drop := drops[0]
	if drop.InstanceID != "drop-instance-id" || drop.TemplateCode != "sagecloth_staff" || drop.OwnerUserID != 77 {
		t.Fatalf("unexpected drop payload ids: %+v", drop)
	}
	if drop.Name != "Sagecloth Staff" || drop.Rarity != "green" || drop.ImageURL != "/equipment/mystic/sagecloth/staff.png" {
		t.Fatalf("unexpected drop display payload: %+v", drop)
	}
	if drop.Item.ItemInstanceID != "drop-instance-id" || drop.Item.ClassID != "mystic" || drop.Item.SetCode != "sagecloth_set" {
		t.Fatalf("unexpected nested drop item: %+v", drop.Item)
	}
}

func TestMaybeGrantMonsterEquipmentDropDisabledChanceDoesNotGrant(t *testing.T) {
	withEquipmentDropStubs(t)
	t.Setenv("EQUIPMENT_DROP_CHANCE_OVERRIDE", "0")

	rollEquipmentDrop = func(chancePercent int) bool {
		if chancePercent != 0 {
			t.Fatalf("expected disabled drop chance 0, got %d", chancePercent)
		}
		return false
	}
	grantEquipmentDropItem = func(userID int) (*repository.DroppedEquipment, error) {
		t.Fatal("drop grant should not be called when chance is disabled")
		return nil, nil
	}

	drops, err := maybeGrantMonsterEquipmentDrop(77)
	if err != nil {
		t.Fatalf("maybeGrantMonsterEquipmentDrop: %v", err)
	}
	if len(drops) != 0 {
		t.Fatalf("expected no drops with disabled chance, got %+v", drops)
	}
}

func TestMaybeGrantMonsterEquipmentDropOverrideAlwaysGrants(t *testing.T) {
	withEquipmentDropStubs(t)
	t.Setenv("EQUIPMENT_DROP_CHANCE_OVERRIDE", "100")

	rollEquipmentDrop = func(chancePercent int) bool {
		if chancePercent != 100 {
			t.Fatalf("expected forced drop chance 100, got %d", chancePercent)
		}
		return true
	}
	grantEquipmentDropItem = func(userID int) (*repository.DroppedEquipment, error) {
		return &repository.DroppedEquipment{
			InstanceID:   "forced-drop",
			OwnerUserID:  userID,
			TemplateID:   1,
			TemplateCode: "aegiswarden_sword",
			Name:         "Aegiswarden Sword",
			Rarity:       "green",
			ImageURL:     "/equipment/guardian/aegiswarden/Aegiswarden Sword.png",
			Slot:         "main_hand",
			ItemType:     "sword",
			ClassID:      "guardian",
			SetCode:      "aegiswarden_set",
			SetName:      "Aegiswarden Set",
		}, nil
	}

	drops, err := maybeGrantMonsterEquipmentDrop(78)
	if err != nil {
		t.Fatalf("maybeGrantMonsterEquipmentDrop: %v", err)
	}
	if len(drops) != 1 || drops[0].TemplateCode != "aegiswarden_sword" {
		t.Fatalf("expected forced guardian drop, got %+v", drops)
	}
}

func TestUniversalAttackMonsterKillUsesGlobalEquipmentDropPool(t *testing.T) {
	withEquipmentDropStubs(t)
	const (
		instanceID = "drop-combat"
		killerID   = 10
		monsterID  = 55
	)

	rollEquipmentDrop = func(chancePercent int) bool { return true }
	grantCalls := 0
	grantEquipmentDropItem = func(userID int) (*repository.DroppedEquipment, error) {
		grantCalls++
		if userID != killerID {
			t.Fatalf("unexpected grant user: %d", userID)
		}
		return &repository.DroppedEquipment{
			InstanceID:   "uuid-drop-1",
			OwnerUserID:  killerID,
			TemplateID:   9,
			TemplateCode: "greenwisp_bow",
			Name:         "Greenwisp Bow",
			Rarity:       "green",
			ImageURL:     "/equipment/ranger/greenwisp/Greenwisp Bow.png",
			Slot:         "main_hand",
			ItemType:     "bow",
			ClassID:      "ranger",
			SetCode:      "greenwisp_set",
			SetName:      "Greenwisp Set",
		}, nil
	}

	exchange, response := runMonsterDropAttack(t, monsterDropAttackOptions{
		instanceID: instanceID,
		killerID:   killerID,
		monsterID:  monsterID,
		attack:     12,
		monsterHP:  5,
		classID:    "guardian",
	})

	if grantCalls != 1 {
		t.Fatalf("expected one drop grant, got %d", grantCalls)
	}
	if len(exchange.Drops) != 1 {
		t.Fatalf("expected combat exchange drop payload, got %+v", exchange.Drops)
	}
	if exchange.Drops[0].OwnerUserID != killerID || exchange.Drops[0].TemplateCode != "greenwisp_bow" {
		t.Fatalf("unexpected exchange drop payload: %+v", exchange.Drops[0])
	}
	if rawDrops, ok := response["drops"].([]interface{}); !ok || len(rawDrops) != 1 {
		t.Fatalf("expected HTTP response drops, got %+v", response["drops"])
	}
}

func TestUniversalAttackMonsterKillAwardsDropWithUnknownCharacterType(t *testing.T) {
	withEquipmentDropStubs(t)
	const (
		killerID  = 13
		monsterID = 58
	)

	rollEquipmentDrop = func(chancePercent int) bool { return true }
	grantCalls := 0
	grantEquipmentDropItem = func(userID int) (*repository.DroppedEquipment, error) {
		grantCalls++
		if userID != killerID {
			t.Fatalf("unexpected grant user: %d", userID)
		}
		return &repository.DroppedEquipment{
			InstanceID:   "uuid-drop-unknown-class",
			OwnerUserID:  killerID,
			TemplateID:   12,
			TemplateCode: "sagecloth_staff",
			Name:         "Sagecloth Staff",
			Rarity:       "green",
			ImageURL:     "/equipment/mystic/sagecloth/staff.png",
			Slot:         "main_hand",
			ItemType:     "staff",
			ClassID:      "mystic",
			SetCode:      "sagecloth_set",
			SetName:      "Sagecloth Set",
		}, nil
	}

	exchange, _ := runMonsterDropAttack(t, monsterDropAttackOptions{
		instanceID: "drop-unknown-class",
		killerID:   killerID,
		monsterID:  monsterID,
		attack:     12,
		monsterHP:  5,
		classID:    "unknown",
	})

	if grantCalls != 1 {
		t.Fatalf("expected drop grant with unknown character type, got %d", grantCalls)
	}
	if len(exchange.Drops) != 1 || exchange.Drops[0].TemplateCode != "sagecloth_staff" {
		t.Fatalf("expected mystic drop despite unknown killer class, got %+v", exchange.Drops)
	}
}

func TestUniversalAttackMonsterSurvivesDoesNotRollDrop(t *testing.T) {
	withEquipmentDropStubs(t)

	rollCalls := 0
	rollEquipmentDrop = func(chancePercent int) bool {
		rollCalls++
		return true
	}
	grantEquipmentDropItem = func(userID int) (*repository.DroppedEquipment, error) {
		t.Fatal("drop grant should not be called for surviving monster")
		return nil, nil
	}

	exchange, response := runMonsterDropAttack(t, monsterDropAttackOptions{
		instanceID: "drop-survive",
		killerID:   11,
		monsterID:  56,
		attack:     2,
		monsterHP:  5,
	})

	if rollCalls != 0 {
		t.Fatalf("drop roll should not run for surviving monster, got %d calls", rollCalls)
	}
	if len(exchange.Drops) != 0 {
		t.Fatalf("expected no exchange drops, got %+v", exchange.Drops)
	}
	if _, exists := response["drops"]; exists {
		t.Fatalf("expected no HTTP response drops, got %+v", response["drops"])
	}
}

func TestUniversalAttackMonsterKillNoDropWhenRollFails(t *testing.T) {
	withEquipmentDropStubs(t)

	rollEquipmentDrop = func(chancePercent int) bool { return false }
	grantEquipmentDropItem = func(userID int) (*repository.DroppedEquipment, error) {
		t.Fatal("drop grant should not be called when roll fails")
		return nil, nil
	}

	exchange, response := runMonsterDropAttack(t, monsterDropAttackOptions{
		instanceID: "drop-roll-fails",
		killerID:   12,
		monsterID:  57,
		attack:     12,
		monsterHP:  5,
	})

	if len(exchange.Drops) != 0 {
		t.Fatalf("expected no exchange drops, got %+v", exchange.Drops)
	}
	if _, exists := response["drops"]; exists {
		t.Fatalf("expected no HTTP response drops, got %+v", response["drops"])
	}
}

func TestSaveTargetHealthDuplicateMonsterDeathReturnsFalse(t *testing.T) {
	const (
		instanceID = "duplicate-drop-guard"
		killerID   = 20
		monsterID  = 70
	)

	deleteCalls := 0
	Combat = CombatDeps{
		UpdateMonsterHealth: func(_ string, _ int, _ int) error { return nil },
		GetMonster: func(_ string, mid int) (*repository.MatchMonster, error) {
			return &repository.MatchMonster{MonsterInstanceID: mid, X: 2, Y: 3, Health: 0}, nil
		},
		DeleteMonster: func(_ string, _ int) error {
			deleteCalls++
			return nil
		},
		LoadMap: func(_ string) ([]game.FullCell, error) {
			return []game.FullCell{{
				X:        2,
				Y:        3,
				TileCode: int('M'),
				Monster:  &game.MonsterData{DBInstanceID: monsterID},
			}}, nil
		},
		SaveMap: func(_ string, _ []game.FullCell) error { return nil },
	}
	defer RestoreDefaults()

	ms := &game.MatchState{InstanceID: instanceID}
	game.MatchStatesMu.Lock()
	game.MatchStates[instanceID] = ms
	game.MatchStatesMu.Unlock()
	t.Cleanup(func() {
		game.MatchStatesMu.Lock()
		delete(game.MatchStates, instanceID)
		game.MatchStatesMu.Unlock()
	})

	origBroadcast := broadcastFn
	broadcastFn = func(_ []byte) {}
	t.Cleanup(func() { broadcastFn = origBroadcast })

	first := saveTargetHealth(instanceID, "monster", monsterID, killerID, "player", attackResult{Damage: 5, NewHealth: 0})
	second := saveTargetHealth(instanceID, "monster", monsterID, killerID, "player", attackResult{Damage: 5, NewHealth: 0})
	if !first {
		t.Fatal("expected first monster death to be processed")
	}
	if second {
		t.Fatal("expected duplicate monster death to be ignored")
	}
	if deleteCalls != 1 {
		t.Fatalf("expected one monster delete, got %d", deleteCalls)
	}
}

type monsterDropAttackOptions struct {
	instanceID string
	killerID   int
	monsterID  int
	attack     int
	monsterHP  int
	classID    string
}

func runMonsterDropAttack(t *testing.T, opts monsterDropAttackOptions) (CombatExchangePayload, map[string]interface{}) {
	t.Helper()
	classID := opts.classID
	if classID == "" {
		classID = "mystic"
	}

	monster := repository.MatchMonster{
		MonsterInstanceID: opts.monsterID,
		Attack:            1,
		Defense:           0,
		Health:            opts.monsterHP,
		MaxHealth:         opts.monsterHP,
		X:                 2,
		Y:                 1,
	}
	cells := []game.FullCell{{
		X:        2,
		Y:        1,
		TileCode: int('M'),
		Monster: &game.MonsterData{
			DBInstanceID: opts.monsterID,
			Health:       opts.monsterHP,
			MaxHealth:    opts.monsterHP,
		},
	}}
	ms := &game.MatchState{
		InstanceID:   opts.instanceID,
		TurnOrder:    []int{opts.killerID},
		ActiveUserID: opts.killerID,
		TurnNumber:   1,
	}
	game.MatchStatesMu.Lock()
	game.MatchStates[opts.instanceID] = ms
	game.MatchStatesMu.Unlock()
	t.Cleanup(func() {
		game.MatchStatesMu.Lock()
		delete(game.MatchStates, opts.instanceID)
		game.MatchStatesMu.Unlock()
	})

	Combat = CombatDeps{
		UpdatePlayer: func(_ string, _ *models.PlayerResponse) error { return nil },
		GetPlayer: func(_ string, userID int) (*models.PlayerResponse, error) {
			return &models.PlayerResponse{
				UserID:        userID,
				Attack:        opts.attack,
				Defense:       0,
				Health:        20,
				MaxHealth:     20,
				Energy:        20,
				CharacterType: classID,
				Position: struct {
					X int `json:"x"`
					Y int `json:"y"`
				}{X: 1, Y: 1},
			}, nil
		},
		GetMonster: func(_ string, monsterID int) (*repository.MatchMonster, error) {
			if monsterID != opts.monsterID {
				t.Fatalf("unexpected monster id %d", monsterID)
			}
			copyMonster := monster
			return &copyMonster, nil
		},
		UpdateMonsterHealth: func(_ string, monsterID, hp int) error {
			if monsterID != opts.monsterID {
				t.Fatalf("unexpected monster id %d", monsterID)
			}
			monster.Health = hp
			return nil
		},
		DeleteMonster: func(_ string, monsterID int) error {
			if monsterID != opts.monsterID {
				t.Fatalf("unexpected monster id %d", monsterID)
			}
			return nil
		},
		LoadMap: func(_ string) ([]game.FullCell, error) {
			out := make([]game.FullCell, len(cells))
			copy(out, cells)
			return out, nil
		},
		SaveMap: func(_ string, next []game.FullCell) error {
			cells = make([]game.FullCell, len(next))
			copy(cells, next)
			return nil
		},
		MarkPlayerDead:  func(_ string, _ int) error { return nil },
		ClearPlayerFlag: func(_ string, _ repository.Position) error { return nil },
		UpdateTurn:      func(_ string, _, _ int) error { return nil },
		Finalize:        func(_ string) error { return nil },
		LoadGameState: func(_ string) (*game.MatchState, bool) {
			return ms, true
		},
	}
	t.Cleanup(RestoreDefaults)

	origBroadcast := broadcastFn
	var exchange CombatExchangePayload
	broadcastFn = func(message []byte) {
		var msg CombatExchangeMessage
		if err := json.Unmarshal(message, &msg); err == nil && msg.Type == "COMBAT_EXCHANGE" {
			exchange = msg.Payload
		}
	}
	t.Cleanup(func() { broadcastFn = origBroadcast })

	rec := httptest.NewRecorder()
	universalAttackLocked(rec, AttackRequest{
		InstanceID:   opts.instanceID,
		AttackerType: "player",
		AttackerID:   opts.killerID,
		TargetType:   "monster",
		TargetID:     opts.monsterID,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected attack status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode attack response: %v", err)
	}

	return exchange, response
}
