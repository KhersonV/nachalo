
//======================================
// gameservice/handlers/match_player.go
//======================================

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"gameservice/repository"
)

// GetMatchPlayerHandler обрабатывает GET-запрос для получения данных игрока по его ID.
// Использует функцию repository.GetMatchPlayerByUserID для получения данных из таблицы match_players.
func GetMatchPlayerHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Player ID обязателен", http.StatusBadRequest)
		return
	}
	playerID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
		return
	}

	player, err := repository.GetMatchPlayerByUserID(playerID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка получения игрока: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(player)
}

