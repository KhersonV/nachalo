// ============================================
// gameservice/handlers/artifacts_http.go
// ============================================
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"gameservice/repository"
	"github.com/gorilla/mux"
)

// JSON-модель для передачи артефакта
type ArtifactDTO struct {
	ID           int      `json:"id"`
	UserID       int      `json:"user_id"`
	ArtifactType string   `json:"artifact_type"`
	ArtifactID   int      `json:"artifact_id"`
	Description  string   `json:"description"`
	Image        string   `json:"image"`
	Rarity       string   `json:"rarity"`
	Durability   int      `json:"durability"`
	BaseValue    float64  `json:"base_value"`
	NPCPrice     float64  `json:"npc_price"`
	AcquiredAt   string   `json:"acquired_at"`
	ExpiresAt    *string  `json:"expires_at,omitempty"`
}

// GetUserArtifactsHandler — возвращает список всех артефактов пользователя
func GetUserArtifactsHandler(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	userID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	arts, err := repository.GetUserArtifacts(userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("fetch artifacts: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(arts)
}

// GetArtifactHandler — возвращает один артефакт по ID
func GetArtifactHandler(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	artifactID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid artifact id", http.StatusBadRequest)
		return
	}

	art, err := repository.GetArtifactByID(artifactID)
	if err == repository.ErrNotFound {
		http.Error(w, "artifact not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, fmt.Sprintf("fetch artifact: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(art)
}

// DeleteArtifactHandler — удаляет артефакт по ID
func DeleteArtifactHandler(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	artifactID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid artifact id", http.StatusBadRequest)
		return
	}

	if err := repository.DeletePersistedArtifact(artifactID); err != nil {
		http.Error(w, fmt.Sprintf("delete artifact: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TransferArtifactRequest — модель запроса на смену владельца
type TransferArtifactRequest struct {
	ArtifactID  int `json:"artifact_id"`
	NewUserID   int `json:"new_user_id"`
}

// TransferArtifactHandler — меняет владельца артефакта
func TransferArtifactHandler(w http.ResponseWriter, r *http.Request) {
	var req TransferArtifactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := repository.TransferPersistedArtifact(req.ArtifactID, req.NewUserID); err != nil {
		http.Error(w, fmt.Sprintf("transfer artifact: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// UpdateArtifactHandler — частичное обновление артефакта
func UpdateArtifactHandler(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	artifactID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid artifact id", http.StatusBadRequest)
		return
	}

	// считываем новую модель полностью
	var a repository.PersistedArtifact
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	a.ID = artifactID

	if err := repository.UpdatePersistedArtifact(a); err != nil {
		http.Error(w, fmt.Sprintf("update artifact: %v", err), http.StatusInternalServerError)
		return
	}

	// возвращаем обновлённый объект
	updated, err := repository.GetArtifactByID(artifactID)
	if err != nil {
		http.Error(w, fmt.Sprintf("fetch updated artifact: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}
