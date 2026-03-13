//=======================
//gameservice/cmd/main.go
//=======================

package main

import (
	"log"
	"net/http"
	"os"

	"gameservice/handlers"
	"gameservice/middleware"
	"gameservice/repository"
	"github.com/gorilla/mux"
)

// Пример CORS middleware (можно вынести отдельно, если захотите)
func enableCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Считываем секретный ключ для JWT
	jwtSecretKey := os.Getenv("JWT_SECRET_KEY")
	if jwtSecretKey == "" {
		log.Fatal("JWT_SECRET_KEY environment variable is not set")
	}
	log.Println("JWT_SECRET_KEY загружен")

	// Инициализация БД
	repository.InitDB()
	defer repository.DB.Close()

	// Настраиваем маршруты
	router := mux.NewRouter()
	// === websocket ===
	router.HandleFunc("/ws", handlers.WsHandler)

	// === Эндпоинты для игроков ===
	router.HandleFunc("/create/player", handlers.CreatePlayerHandler).Methods("POST")

	router.HandleFunc("/game/player/{id}", handlers.GetPlayerHandler).Methods("GET")

	router.HandleFunc("/game/matches/{instance_id}/players/{id}", handlers.GetMatchPlayerHandler).Methods("GET")

	router.HandleFunc("/game/player/{id}/gain_experience", handlers.GainExperienceHandler).Methods("POST")

	// === Эндпоинты для инвентаря ===
	router.HandleFunc("/game/player/{id}/inventory/add", handlers.AddInventoryHandler).Methods("POST")
	router.HandleFunc("/game/player/{id}/inventory/use", handlers.UseInventoryHandler).Methods("POST")

	// === Эндпоинты для матчей ===
	router.HandleFunc("/game/createMatch", handlers.CreateMatchHandler).Methods("POST")
	router.Handle(
		"/game/match",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetMatchHandler)),
	).Methods("GET")
	router.Handle(
		"/game/match/{instance_id}/my-stats",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetMyMatchStatsHandler)),
	).Methods("GET")

	// Эндпоинт для финализации матча
	router.Handle("/game/finishMatch",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.FinishMatchHandler)),
	).Methods("POST")

	// === Эндпоинты для перемещения и атаки (используют JWT middleware) ===
	router.Handle("/game/{instance_id}/player/{id}/move",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.MoveOrAttackHandler))).
		Methods("POST")

	router.Handle("/game/attack",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.UniversalAttackHandler))).
		Methods("POST")

	router.Handle("/game/endTurn",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.EndTurnHandler)),
	).Methods("POST")

	// === Эндпоинты для ресурсов и монстров ===
	// В файле gameservice/cmd/main.go, внутри настройки маршрутов:
	router.HandleFunc("/game/collectResource", handlers.CollectResourceHandler).Methods("POST")
	router.HandleFunc("/game/openBarrel", handlers.OpenBarrelHandler).Methods("POST")

	router.HandleFunc("/api/resources", handlers.GetResourcesHandler).Methods("GET")
	router.HandleFunc("/api/monsters", handlers.GetMonstersHandler).Methods("GET")

	router.HandleFunc("/game/user/{id}/artifacts", handlers.GetUserArtifactsHandler).Methods("GET")
	router.HandleFunc("/game/artifact/{id}", handlers.GetArtifactHandler).Methods("GET")
	router.HandleFunc("/game/artifact/{id}", handlers.DeleteArtifactHandler).Methods("DELETE")
	router.HandleFunc("/game/artifact/transfer", handlers.TransferArtifactHandler).Methods("POST")
	router.HandleFunc("/game/artifact/{id}", handlers.UpdateArtifactHandler).Methods("PATCH")

	// Подключаем CORS
	handler := enableCors(router)

	// Запускаем сервер на 8001 порту
	port := "8001"
	log.Printf("Game-сервис запущен на порту %s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
