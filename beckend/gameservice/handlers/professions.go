package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"gameservice/middleware"
	"gameservice/repository"
)

func professionError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	status, code := http.StatusInternalServerError, "profession_unavailable"
	switch {
	case errors.Is(err, repository.ErrCraftInvalid), errors.Is(err, repository.ErrCraftIngredients), errors.Is(err, repository.ErrCraftLevel), errors.Is(err, repository.ErrCraftProfession), errors.Is(err, repository.ErrCraftReplay):
		status, code = http.StatusBadRequest, err.Error()
	case errors.Is(err, repository.ErrProfessionGuild):
		status, code = http.StatusForbidden, "guild_required"
	case errors.Is(err, repository.ErrProfessionCharacter):
		status, code = http.StatusForbidden, "character_not_owned"
	case errors.Is(err, repository.ErrProfessionInvalid):
		status, code = http.StatusBadRequest, "invalid_profession"
	case errors.Is(err, repository.ErrProfessionConfirmation):
		status, code = http.StatusConflict, "reset_confirmation_required"
	case errors.Is(err, repository.ErrProfessionStale):
		status, code = http.StatusConflict, "profession_changed"
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func GetProfessionsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID <= 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	characterID := 0
	if raw := r.URL.Query().Get("characterId"); raw != "" {
		var err error
		characterID, err = strconv.Atoi(raw)
		if err != nil || characterID <= 0 {
			professionError(w, repository.ErrProfessionInvalid)
			return
		}
	}
	state, err := repository.GetCharacterProfessions(r.Context(), userID, characterID)
	if err != nil {
		professionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func ChooseProfessionHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID <= 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var request struct {
		CharacterID  int    `json:"characterId"`
		Code         string `json:"code"`
		ExpectedCode string `json:"expectedCode"`
		ConfirmReset bool   `json:"confirmReset"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		professionError(w, repository.ErrProfessionInvalid)
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		professionError(w, repository.ErrProfessionInvalid)
		return
	}
	if err := repository.ChooseProcessingProfession(r.Context(), userID, request.CharacterID, request.Code, request.ExpectedCode, request.ConfirmReset); err != nil {
		professionError(w, err)
		return
	}
	state, err := repository.GetCharacterProfessions(r.Context(), userID, request.CharacterID)
	if err != nil {
		professionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

// Only public catalog data: no match positions, seeds, or roll outcomes.
func GetProfessionLootCatalogHandler(w http.ResponseWriter, r *http.Request) {
	catalog, err := repository.ProfessionLootCatalog(r.Context())
	if err != nil {
		professionError(w, err)
		return
	}
	gathering, err := repository.GatheringLootCatalog(r.Context())
	if err != nil {
		professionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	recipes, err := repository.PublicCraftRecipes(r.Context())
	if err != nil {
		professionError(w, err)
		return
	}
	sets, err := repository.EquipmentSetsCatalog(r.Context())
	if err != nil {
		professionError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"equipmentSets":       sets,
		"recipes":             recipes,
		"gathering":           gathering,
		"gatheringLimit":      repository.ProfessionGatherLimit,
		"gatheringXP":         repository.ProfessionGatherXP,
		"monsters":            catalog,
		"successLimit":        repository.ProfessionMonsterDropLimit,
		"baseEquipmentChance": float64(repository.DefaultEquipmentDropChancePercent) / 100,
	})
}

func GetCraftRecipesHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}
	character := 0
	if raw := r.URL.Query().Get("characterId"); raw != "" {
		var err error
		character, err = strconv.Atoi(raw)
		if err != nil || character < 1 {
			professionError(w, repository.ErrCraftInvalid)
			return
		}
	}
	recipes, err := repository.GetCraftRecipes(r.Context(), user, character)
	if err != nil {
		professionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"recipes": recipes})
}
func CraftProfessionItemHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}
	var req struct {
		CharacterID int    `json:"characterId"`
		RecipeCode  string `json:"recipeCode"`
		Quantity    int    `json:"quantity"`
		CommandID   string `json:"commandId"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		professionError(w, repository.ErrCraftInvalid)
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		professionError(w, repository.ErrCraftInvalid)
		return
	}
	result, err := repository.CraftProfessionItem(r.Context(), user, req.CharacterID, req.RecipeCode, req.Quantity, req.CommandID)
	if err != nil {
		professionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
