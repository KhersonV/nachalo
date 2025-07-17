//======================================
// gameservice/handlers/match_player.go
//======================================

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"gameservice/repository"
	"github.com/gorilla/mux"
)

// GetMatchPlayerHandler обрабатывает GET-запрос для получения данных игрока по инстансу и ID.
func GetMatchPlayerHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	instanceID, ok1 := vars["instance_id"]
	idStr, ok2 := vars["id"]

	if !ok2 {
		http.Error(w, "Player ID обязателен", http.StatusBadRequest)
		return
	}
	if !ok1 {
		http.Error(w, "Instance ID обязателен", http.StatusBadRequest)
		return
	}
	playerID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
		return
	}
	player, err := repository.GetMatchPlayerByID(instanceID, playerID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения игрока: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(player)
}
