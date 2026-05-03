package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"gameservice/middleware"
	"gameservice/repository"
)

type equipmentStateDataResponse struct {
	ActiveCharacterID           int                                             `json:"activeCharacterId"`
	ActiveHeroClassID           string                                          `json:"activeHeroClassId"`
	Characters                  []repository.EquipmentCharacterSummary          `json:"characters"`
	Inventory                   []repository.EquipmentItem                      `json:"inventory"`
	OwnedItems                  []repository.EquipmentItem                      `json:"ownedItems"`
	Equipped                    map[string]*repository.EquipmentItem            `json:"equipped"`
	EquippedByCharacter         map[string]map[string]*repository.EquipmentItem `json:"equippedByCharacter"`
	BaseStats                   repository.EquipmentStats                       `json:"baseStats"`
	EffectiveStats              repository.EquipmentStats                       `json:"effectiveStats"`
	ActiveSetBonuses            []repository.ActiveEquipmentSetBonus            `json:"activeSetBonuses"`
	StatsByCharacter            map[string]characterEquipmentStatsResponse      `json:"statsByCharacter"`
	ActiveSetBonusesByCharacter map[string][]repository.ActiveEquipmentSetBonus `json:"activeSetBonusesByCharacter"`
}

type characterEquipmentStatsResponse struct {
	BaseStats      repository.EquipmentStats   `json:"baseStats"`
	BonusStats     repository.EquipmentBonuses `json:"bonusStats"`
	EffectiveStats repository.EquipmentStats   `json:"effectiveStats"`
}

type equipmentStateResponse struct {
	Status string                     `json:"status"`
	Data   equipmentStateDataResponse `json:"data"`
}

type equipItemRequest struct {
	CharacterID    int    `json:"characterId"`
	ItemInstanceID string `json:"itemInstanceId"`
}

type unequipItemRequest struct {
	CharacterID int    `json:"characterId"`
	Slot        string `json:"slot"`
}

type grantItemDevRequest struct {
	TemplateCode string `json:"templateCode"`
}

type grantClassSetDevRequest struct {
	HeroClassID string `json:"heroClassId"`
	SetSlug     string `json:"setSlug"`
}

func equipmentDevGrantEnabled() bool {
	return strings.EqualFold(os.Getenv("EQUIPMENT_DEV_GRANT_ENABLED"), "true") ||
		strings.EqualFold(os.Getenv("APP_ENV"), "development")
}

func equipmentBonusFromStats(base repository.EquipmentStats, effective repository.EquipmentStats) repository.EquipmentBonuses {
	return repository.EquipmentBonuses{
		Attack:      effective.Attack - base.Attack,
		Defense:     effective.Defense - base.Defense,
		Mobility:    effective.Mobility - base.Mobility,
		Agility:     effective.Agility - base.Agility,
		MaxHealth:   effective.MaxHealth - base.MaxHealth,
		MaxEnergy:   effective.MaxEnergy - base.MaxEnergy,
		SightRange:  effective.SightRange - base.SightRange,
		AttackRange: effective.AttackRange - base.AttackRange,
	}
}

func buildEquipmentStateResponse(userID int) (*equipmentStateResponse, error) {
	character, err := repository.GetSelectedCharacterForUser(userID)
	if err != nil {
		return nil, err
	}
	activeHeroClassID := repository.NormalizeHeroClassID(character.HeroClassID)

	characters, err := repository.ListPlayerEquipmentCharacters(userID)
	if err != nil {
		return nil, err
	}

	inventory, err := repository.GetEquipmentInventoryForUser(userID)
	if err != nil {
		return nil, err
	}
	ownedItems, err := repository.GetAllEquipmentForUser(userID)
	if err != nil {
		return nil, err
	}

	stats, err := repository.GetCharacterEffectiveStats(character.ID)
	if err != nil {
		return nil, err
	}

	equippedByCharacter := map[string]map[string]*repository.EquipmentItem{}
	statsByCharacter := map[string]characterEquipmentStatsResponse{}
	activeSetBonusesByCharacter := map[string][]repository.ActiveEquipmentSetBonus{}
	for _, ownedCharacter := range characters {
		characterStats, err := repository.GetCharacterEffectiveStats(ownedCharacter.CharacterID)
		if err != nil {
			return nil, err
		}
		key := strconv.Itoa(ownedCharacter.CharacterID)
		equippedByCharacter[key] = characterStats.Equipped
		statsByCharacter[key] = characterEquipmentStatsResponse{
			BaseStats:      characterStats.BaseStats,
			BonusStats:     equipmentBonusFromStats(characterStats.BaseStats, characterStats.EffectiveStats),
			EffectiveStats: characterStats.EffectiveStats,
		}
		activeSetBonusesByCharacter[key] = characterStats.ActiveSetBonuses
	}

	return &equipmentStateResponse{
		Status: "ok",
		Data: equipmentStateDataResponse{
			ActiveCharacterID:           character.ID,
			ActiveHeroClassID:           activeHeroClassID,
			Characters:                  characters,
			Inventory:                   inventory,
			OwnedItems:                  ownedItems,
			Equipped:                    stats.Equipped,
			EquippedByCharacter:         equippedByCharacter,
			BaseStats:                   stats.BaseStats,
			EffectiveStats:              stats.EffectiveStats,
			ActiveSetBonuses:            stats.ActiveSetBonuses,
			StatsByCharacter:            statsByCharacter,
			ActiveSetBonusesByCharacter: activeSetBonusesByCharacter,
		},
	}, nil
}

func writeEquipmentError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")

	switch {
	case errors.Is(err, repository.ErrPlayerNotFound),
		errors.Is(err, repository.ErrEquipmentCharacterNotFound):
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "character_not_found"})
	case errors.Is(err, repository.ErrEquipmentCharacterNotOwned):
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "character_not_owned"})
	case errors.Is(err, repository.ErrEquipmentItemNotFound):
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "item_not_found"})
	case errors.Is(err, repository.ErrEquipmentItemNotOwned):
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "item_not_owned"})
	case errors.Is(err, repository.ErrEquipmentSetNotFound):
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "equipment_set_not_found"})
	case errors.Is(err, repository.ErrEquipmentInvalidSlot):
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_slot"})
	case errors.Is(err, repository.ErrEquipmentClassRestricted):
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "class_restricted"})
	case errors.Is(err, repository.ErrEquipmentLevelTooLow):
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "level_too_low"})
	case errors.Is(err, repository.ErrEquipmentSlotMismatch),
		errors.Is(err, repository.ErrEquipmentInvalidTemplate):
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_item_template"})
	case errors.Is(err, repository.ErrEquipmentOffHandBlocked):
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "off_hand_blocked_by_two_handed_weapon"})
	case errors.Is(err, repository.ErrEquipmentItemLocked):
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "item_locked"})
	case errors.Is(err, repository.ErrEquipmentItemListed):
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "item_listed"})
	case errors.Is(err, repository.ErrEquipmentItemTradeLocked):
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "item_trade_locked"})
	case errors.Is(err, repository.ErrEquipmentItemDeleted):
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "item_deleted"})
	case errors.Is(err, repository.ErrEquipmentItemUnavailable):
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "item_unavailable"})
	case errors.Is(err, repository.ErrEquipmentItemNotInInventory):
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "item_not_in_inventory"})
	default:
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "equipment_error"})
	}
}

func GetEquipmentHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	MarkUserHTTPActive(userID)

	state, err := buildEquipmentStateResponse(userID)
	if err != nil {
		writeEquipmentError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func GrantSageclothDevHandler(w http.ResponseWriter, r *http.Request) {
	if !equipmentDevGrantEnabled() {
		http.NotFound(w, r)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	MarkUserHTTPActive(userID)

	if err := repository.GrantEquipmentSetToUser(userID, "mystic", "sagecloth_set", "admin"); err != nil {
		writeEquipmentError(w, err)
		return
	}

	state, err := buildEquipmentStateResponse(userID)
	if err != nil {
		writeEquipmentError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

// GrantClassSetDevHandler is intentionally dev-only. It grants one copy of
// every item template from a class set for local equipment testing.
func GrantClassSetDevHandler(w http.ResponseWriter, r *http.Request) {
	if !equipmentDevGrantEnabled() {
		http.NotFound(w, r)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	MarkUserHTTPActive(userID)

	var req grantClassSetDevRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	if err := repository.GrantEquipmentSetToUser(userID, req.HeroClassID, req.SetSlug, "admin"); err != nil {
		writeEquipmentError(w, err)
		return
	}

	state, err := buildEquipmentStateResponse(userID)
	if err != nil {
		writeEquipmentError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

// GrantItemDevHandler is intentionally dev-only. It exists for local equipment
// testing before real drop/shop/crafting flows are wired into gameplay.
func GrantItemDevHandler(w http.ResponseWriter, r *http.Request) {
	if !equipmentDevGrantEnabled() {
		http.NotFound(w, r)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	MarkUserHTTPActive(userID)

	var req grantItemDevRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}
	req.TemplateCode = strings.TrimSpace(req.TemplateCode)
	if req.TemplateCode == "" {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	if _, err := repository.GrantItemInstanceToUser(userID, req.TemplateCode, "admin"); err != nil {
		writeEquipmentError(w, err)
		return
	}

	state, err := buildEquipmentStateResponse(userID)
	if err != nil {
		writeEquipmentError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func EquipItemHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	MarkUserHTTPActive(userID)

	var req equipItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}
	if req.CharacterID <= 0 || req.ItemInstanceID == "" {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	if err := repository.EquipItemToCharacter(userID, req.CharacterID, req.ItemInstanceID); err != nil {
		log.Printf("equipment equip failed: userID=%d characterID=%d itemInstanceID=%s err=%v", userID, req.CharacterID, req.ItemInstanceID, err)
		writeEquipmentError(w, err)
		return
	}

	state, err := buildEquipmentStateResponse(userID)
	if err != nil {
		log.Printf("equipment equip failed: userID=%d characterID=%d itemInstanceID=%s err=%v", userID, req.CharacterID, req.ItemInstanceID, err)
		writeEquipmentError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func UnequipItemHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	MarkUserHTTPActive(userID)

	var req unequipItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}
	if req.CharacterID <= 0 || req.Slot == "" {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	if err := repository.UnequipItemFromCharacter(userID, req.CharacterID, req.Slot); err != nil {
		writeEquipmentError(w, err)
		return
	}

	state, err := buildEquipmentStateResponse(userID)
	if err != nil {
		writeEquipmentError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}
