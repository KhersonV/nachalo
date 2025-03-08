package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"

    "github.com/gorilla/mux"

    "gameservice/repository"
)

func AddInventoryHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    playerID, err := strconv.Atoi(vars["id"])
    if err != nil {
        http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
        return
    }

    var req struct {
        ItemType string `json:"item_type"`
        ItemID   int    `json:"item_id"`
        Count    int    `json:"count"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
        return
    }
    if req.Count <= 0 {
        req.Count = 1
    }

    err = repository.AddInventoryItem(playerID, req.ItemType, req.ItemID, req.Count)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка добавления предмета: %v", err), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Предмет успешно добавлен"))
}

func UseInventoryHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    playerID, err := strconv.Atoi(vars["id"])
    if err != nil {
        http.Error(w, "Некорректный ID игрока", http.StatusBadRequest)
        return
    }

    var req struct {
        ItemType string `json:"item_type"`
        ItemID   int    `json:"item_id"`
        Count    int    `json:"count"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
        return
    }
    if req.Count <= 0 {
        req.Count = 1
    }

    err = repository.RemoveInventoryItem(playerID, req.ItemType, req.ItemID, req.Count)
    if err != nil {
        http.Error(w, fmt.Sprintf("Ошибка использования предмета: %v", err), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Предмет успешно использован/удалён"))
}
