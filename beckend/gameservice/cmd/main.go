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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
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
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := repository.PingDB(); err != nil {
			http.Error(w, "database is not ready", http.StatusServiceUnavailable)
			return
		}

		ready, err := repository.SchemaReady()
		if err != nil {
			http.Error(w, "schema check failed", http.StatusServiceUnavailable)
			return
		}
		if !ready {
			http.Error(w, "schema is not ready", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}).Methods("GET")
	// === websocket ===
	router.HandleFunc("/ws", handlers.WsHandler)

	// === Эндпоинты для игроков ===
	router.HandleFunc("/create/player", handlers.CreatePlayerHandler).Methods("POST")

	router.HandleFunc("/game/player/{id}", handlers.GetPlayerHandler).Methods("GET")

	router.HandleFunc("/game/matches/{instance_id}/players/{id}", handlers.GetMatchPlayerHandler).Methods("GET")

	router.HandleFunc("/game/player/{id}/gain_experience", handlers.GainExperienceHandler).Methods("POST")
	router.HandleFunc("/game/shop/items", handlers.GetShopItemsHandler).Methods("GET")
	router.Handle(
		"/game/shop/buy",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.BuyShopItemHandler)),
	).Methods("POST")
	router.Handle(
		"/game/base/state",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetBaseStateHandler)),
	).Methods("GET")
	router.Handle(
		"/game/base/forge/build",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.BuildForgeHandler)),
	).Methods("POST")
	router.Handle(
		"/game/base/library/build",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.BuildLibraryHandler)),
	).Methods("POST")
	router.Handle(
		"/game/profile",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetProfileHandler)),
	).Methods("GET")
	router.Handle(
		"/game/profile",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.UpdateProfileHandler)),
	).Methods("PATCH")
	router.Handle(
		"/game/profile/{id}",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetPublicProfileHandler)),
	).Methods("GET")
	router.Handle(
		"/game/friends",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetFriendsHandler)),
	).Methods("GET")
	router.Handle(
		"/game/friends/add",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.AddFriendHandler)),
	).Methods("POST")
	router.Handle(
		"/game/friends/{id}",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.RemoveFriendHandler)),
	).Methods("DELETE")
	router.Handle(
		"/game/players/search",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.SearchPlayersHandler)),
	).Methods("GET")
	router.Handle(
		"/game/friends/requests/incoming",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetIncomingFriendRequestsHandler)),
	).Methods("GET")
	router.Handle(
		"/game/friends/requests/outgoing",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetOutgoingFriendRequestsHandler)),
	).Methods("GET")
	router.Handle(
		"/game/friends/requests/{id}/accept",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.AcceptFriendRequestHandler)),
	).Methods("POST")
	router.Handle(
		"/game/friends/requests/{id}/reject",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.RejectFriendRequestHandler)),
	).Methods("POST")
	router.Handle(
		"/game/friends/requests/{id}",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.CancelOutgoingFriendRequestHandler)),
	).Methods("DELETE")

	// === Эндпоинты для инвентаря ===
	router.HandleFunc("/game/player/{id}/inventory/add", handlers.AddInventoryHandler).Methods("POST")
	router.HandleFunc("/game/player/{id}/inventory/use", handlers.UseInventoryHandler).Methods("POST")
	router.Handle(
		"/game/blueprint/place",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.PlaceBlueprintHandler)),
	).Methods("POST")

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

	router.Handle(
		"/game/match/{instance_id}/use-scroll",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.UseScrollHandler)),
	).Methods("POST")

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
	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}
	log.Printf("Game-сервис запущен на порту %s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
