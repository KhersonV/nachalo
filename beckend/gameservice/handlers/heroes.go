package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"gameservice/middleware"
	"gameservice/repository"

	"github.com/gorilla/mux"
)

const tavernHeroUnlockPrice = 2500

type heroCatalogEntry struct {
	ID             string
	DisplayName    string
	Description    string
	UnlockPrice    int
	RequiresTavern bool
	Enabled        bool
	SortOrder      int
	AlwaysOwned    bool
	Hireable       bool
}

type heroStateResponse struct {
	ID             string  `json:"id"`
	DisplayName    string  `json:"displayName"`
	Description    string  `json:"description"`
	Image          string  `json:"image"`
	Owned          bool    `json:"owned"`
	CharacterID    *int    `json:"characterId,omitempty"`
	Active         bool    `json:"active"`
	Locked         bool    `json:"locked"`
	UnlockPrice    int     `json:"unlockPrice"`
	RequiresTavern bool    `json:"requiresTavern"`
	CanHire        bool    `json:"canHire"`
	LockReason     *string `json:"lockReason"`
	Enabled        bool    `json:"enabled"`
	SortOrder      int     `json:"sortOrder"`
}

type heroesStateResponse struct {
	ActiveHeroClassID string              `json:"activeHeroClassId"`
	TavernBuilt       bool                `json:"tavernBuilt"`
	Gold              int                 `json:"gold"`
	Heroes            []heroStateResponse `json:"heroes"`
}

type setActiveHeroRequest struct {
	HeroClassID string `json:"heroClassId"`
}

var heroCatalog = []heroCatalogEntry{
	{
		ID:             "guardian",
		DisplayName:    "Guardian",
		Description:    "Durable frontline defender.",
		UnlockPrice:    tavernHeroUnlockPrice,
		RequiresTavern: true,
		Enabled:        true,
		SortOrder:      10,
		Hireable:       true,
	},
	{
		ID:             "berserker",
		DisplayName:    "Berserker",
		Description:    "High-risk melee finisher.",
		UnlockPrice:    tavernHeroUnlockPrice,
		RequiresTavern: true,
		Enabled:        true,
		SortOrder:      20,
		Hireable:       true,
	},
	{
		ID:             "ranger",
		DisplayName:    "Ranger",
		Description:    "Mobile ranged scout.",
		UnlockPrice:    tavernHeroUnlockPrice,
		RequiresTavern: true,
		Enabled:        true,
		SortOrder:      30,
		Hireable:       true,
	},
	{
		ID:             "mystic",
		DisplayName:    "Mystic",
		Description:    "Long-range caster with strong energy control.",
		UnlockPrice:    tavernHeroUnlockPrice,
		RequiresTavern: true,
		Enabled:        true,
		SortOrder:      40,
		Hireable:       true,
	},
}

func enabledHeroCatalogByID() map[string]heroCatalogEntry {
	byID := make(map[string]heroCatalogEntry, len(heroCatalog))
	for _, hero := range heroCatalog {
		if hero.Enabled {
			byID[repository.NormalizeHeroClassID(hero.ID)] = hero
		}
	}
	return byID
}

func findEnabledHeroCatalogEntry(heroClassID string) (heroCatalogEntry, bool) {
	normalized := repository.NormalizeHeroClassID(heroClassID)
	for _, hero := range heroCatalog {
		if !hero.Enabled {
			continue
		}
		if repository.NormalizeHeroClassID(hero.ID) == normalized {
			return hero, true
		}
	}
	return heroCatalogEntry{}, false
}

func buildHeroState(hero heroCatalogEntry, owned bool, characterID *int, active bool, tavernBuilt bool, gold int) heroStateResponse {
	meta := repository.ResolveHeroClassMeta(hero.ID)
	if owned {
		return heroStateResponse{
			ID:             hero.ID,
			DisplayName:    hero.DisplayName,
			Description:    hero.Description,
			Image:          meta.Image,
			Owned:          true,
			CharacterID:    characterID,
			Active:         active,
			Locked:         false,
			UnlockPrice:    hero.UnlockPrice,
			RequiresTavern: hero.RequiresTavern,
			CanHire:        false,
			LockReason:     nil,
			Enabled:        hero.Enabled,
			SortOrder:      hero.SortOrder,
		}
	}

	var lockReason *string
	canHire := false
	if hero.RequiresTavern && !tavernBuilt {
		reason := "tavern_required"
		lockReason = &reason
	} else if gold < hero.UnlockPrice {
		reason := "not_enough_gold"
		lockReason = &reason
	} else {
		canHire = true
	}

	return heroStateResponse{
		ID:             hero.ID,
		DisplayName:    hero.DisplayName,
		Description:    hero.Description,
		Image:          meta.Image,
		Owned:          false,
		Active:         active,
		Locked:         true,
		UnlockPrice:    hero.UnlockPrice,
		RequiresTavern: hero.RequiresTavern,
		CanHire:        canHire,
		LockReason:     lockReason,
		Enabled:        hero.Enabled,
		SortOrder:      hero.SortOrder,
	}
}

func buildHeroesStateResponse(userID int) (*heroesStateResponse, error) {
	player, err := repository.GetPlayerByUserID(userID)
	if err != nil {
		return nil, err
	}

	// Map of hero_class_id -> character_id for owned characters
	charactersMap, err := repository.GetPlayerCharactersMap(userID)
	if err != nil {
		return nil, err
	}

	tavernLevel, err := repository.GetTavernLevel(userID)
	if err != nil {
		return nil, err
	}
	tavernBuilt := tavernLevel > 0

	catalogByID := enabledHeroCatalogByID()
	activeHeroClassID := repository.NormalizeHeroClassID(player.CharacterType)
	if _, ok := catalogByID[activeHeroClassID]; !ok {
		log.Printf("GetHeroesHandler: user %d active character has unknown hero %q, falling back to %q", userID, activeHeroClassID, repository.DefaultHeroClassID)
		activeHeroClassID = repository.DefaultHeroClassID
	}

	heroes := make([]heroStateResponse, 0, len(heroCatalog))
	for _, hero := range heroCatalog {
		if !hero.Enabled {
			continue
		}

		heroID := repository.NormalizeHeroClassID(hero.ID)
		var characterIDPtr *int
		if cid, ok := charactersMap[heroID]; ok {
			v := cid
			characterIDPtr = &v
		}
		owned := hero.AlwaysOwned || characterIDPtr != nil
		active := heroID == activeHeroClassID
		heroes = append(heroes, buildHeroState(hero, owned, characterIDPtr, active, tavernBuilt, player.Balance))
	}

	return &heroesStateResponse{
		ActiveHeroClassID: activeHeroClassID,
		TavernBuilt:       tavernBuilt,
		Gold:              player.Balance,
		Heroes:            heroes,
	}, nil
}

func GetHeroesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	state, err := buildHeroesStateResponse(userID)
	if err != nil {
		http.Error(w, `{"error":"failed_to_load_hero_state"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func SetActiveHeroHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req setActiveHeroRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	hero, ok := findEnabledHeroCatalogEntry(req.HeroClassID)
	if !ok {
		http.Error(w, `{"error":"hero_not_found"}`, http.StatusNotFound)
		return
	}

	charactersMap, err := repository.GetPlayerCharactersMap(userID)
	if err != nil {
		http.Error(w, `{"error":"failed_to_load_hero_ownership"}`, http.StatusInternalServerError)
		return
	}
	heroID := repository.NormalizeHeroClassID(hero.ID)
	owned := hero.AlwaysOwned || charactersMap[heroID] != 0
	if !owned {
		http.Error(w, `{"error":"hero_not_owned"}`, http.StatusForbidden)
		return
	}

	if err := repository.SetSelectedCharacterByHeroClass(userID, hero.ID); err != nil {
		if errors.Is(err, repository.ErrPlayerNotFound) {
			http.Error(w, `{"error":"player_not_found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed_to_set_active_hero"}`, http.StatusInternalServerError)
		return
	}

	state, err := buildHeroesStateResponse(userID)
	if err != nil {
		http.Error(w, `{"error":"failed_to_load_hero_state"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func HireHeroHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	heroClassID := mux.Vars(r)["heroClassId"]
	hero, ok := findEnabledHeroCatalogEntry(heroClassID)
	if !ok {
		http.Error(w, `{"error":"hero_not_found"}`, http.StatusNotFound)
		return
	}
	if !hero.Hireable {
		http.Error(w, `{"error":"hero_not_hireable"}`, http.StatusBadRequest)
		return
	}

	if _, err := repository.HireHero(userID, hero.ID, hero.UnlockPrice, hero.RequiresTavern); err != nil {
		switch {
		case errors.Is(err, repository.ErrTavernRequired):
			http.Error(w, `{"error":"tavern_required"}`, http.StatusBadRequest)
			return
		case errors.Is(err, repository.ErrNotEnoughGold):
			http.Error(w, `{"error":"not_enough_gold"}`, http.StatusBadRequest)
			return
		case errors.Is(err, repository.ErrHeroAlreadyOwned):
			http.Error(w, `{"error":"hero_already_owned"}`, http.StatusConflict)
			return
		case errors.Is(err, repository.ErrPlayerNotFound):
			http.Error(w, `{"error":"player_not_found"}`, http.StatusNotFound)
			return
		default:
			http.Error(w, `{"error":"failed_to_hire_hero"}`, http.StatusInternalServerError)
			return
		}
	}

	state, err := buildHeroesStateResponse(userID)
	if err != nil {
		http.Error(w, `{"error":"failed_to_load_hero_state"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}
