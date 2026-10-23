//=======================
//gameservice/cmd/main.go
//=======================

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
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
	if err := handlers.RegisterMarketShopCatalog(context.Background()); err != nil {
		log.Fatalf("market catalog: %v", err)
	}
	defer repository.DB.Close()
	if err := repository.MarkHexPresenceUnknownOnStart(context.Background()); err != nil {
		log.Fatalf("HEX presence initialization: %v", err)
	}
	hexWorkerContext, stopHexWorker := context.WithCancel(context.Background())
	defer stopHexWorker()
	handlers.StartHexTurnWorker(hexWorkerContext)
	handlers.StartSocketHeartbeat(hexWorkerContext)
	repository.StartMarketExpiryWorker(hexWorkerContext)
	go func() {
		prune := func() {
			ctx, cancel := context.WithTimeout(hexWorkerContext, 30*time.Second)
			defer cancel()
			if err := (repository.ChatStore{DB: repository.DB}).PruneExpired(ctx); err != nil {
				log.Printf("chat retention cleanup failed: %v", err)
			}
		}
		prune()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-hexWorkerContext.Done():
				return
			case <-ticker.C:
				prune()
			}
		}
	}()

	// Настраиваем маршруты
	router := mux.NewRouter()
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := repository.PingDB(); err != nil {
			http.Error(w, "database is not ready", http.StatusServiceUnavailable)
			return
		}

		ready, err := repository.SchemaReady()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if !ready {
			http.Error(w, "schema is not ready", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}).Methods("GET")
	if os.Getenv("HEX_PREVIEW_ENABLED") == "true" {
		router.HandleFunc("/game/hex-preview", handlers.HexPreviewHandler).Methods("GET")
	}
	if os.Getenv("HEX_TEST_MATCH_ENABLED") == "true" {
		router.Handle("/game/hex-test", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.CreateHexTestMatchHandler))).Methods("POST")
		router.Handle("/game/hex-test/{instance_id}", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetHexTestMatchHandler))).Methods("GET")
		router.Handle("/game/hex-test/{instance_id}/route", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.PreviewHexRouteHandler))).Methods("POST")
		router.Handle("/game/hex-test/{instance_id}/move", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.MoveHexTestMatchHandler))).Methods("POST")
		router.Handle("/game/hex-test/{instance_id}/collect", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.CollectHexResourceHandler))).Methods("POST")
		router.Handle("/game/hex-test/{instance_id}/open-barrel", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.OpenHexBarrelHandler))).Methods("POST")
		router.Handle("/game/hex-test/{instance_id}/use-item", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.UseHexItemHandler))).Methods("POST")
		router.Handle("/game/hex-test/{instance_id}/place-blueprint", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.PlaceHexBlueprintHandler))).Methods("POST")
		router.Handle("/game/hex-test/{instance_id}/attack-structure", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.AttackHexStructureHandler))).Methods("POST")
		router.Handle("/game/hex-test/{instance_id}/extract", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ExtractHexMatchHandler))).Methods("POST")
		router.Handle("/game/hex-test/{instance_id}/end-turn", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.EndHexTurnHandler))).Methods("POST")
		router.Handle("/game/hex-test/{instance_id}/attack-preview", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.PreviewHexAttackHandler))).Methods("POST")
		router.Handle("/game/hex-test/{instance_id}/attack", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.AttackHexMonsterHandler))).Methods("POST")
	}
	router.Handle("/game/hex/{instance_id}", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetHexTestMatchHandler))).Methods("GET")
	router.Handle("/game/hex/{instance_id}/route", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.PreviewHexRouteHandler))).Methods("POST")
	router.Handle("/game/hex/{instance_id}/move", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.MoveHexTestMatchHandler))).Methods("POST")
	router.Handle("/game/hex/{instance_id}/collect", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.CollectHexResourceHandler))).Methods("POST")
	router.Handle("/game/hex/{instance_id}/open-barrel", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.OpenHexBarrelHandler))).Methods("POST")
	router.Handle("/game/hex/{instance_id}/use-item", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.UseHexItemHandler))).Methods("POST")
	router.Handle("/game/hex/{instance_id}/place-blueprint", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.PlaceHexBlueprintHandler))).Methods("POST")
	router.Handle("/game/hex/{instance_id}/attack-structure", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.AttackHexStructureHandler))).Methods("POST")
	router.Handle("/game/hex/{instance_id}/extract", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ExtractHexMatchHandler))).Methods("POST")
	router.Handle("/game/hex/{instance_id}/end-turn", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.EndHexTurnHandler))).Methods("POST")
	router.Handle("/game/hex/{instance_id}/attack-preview", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.PreviewHexAttackHandler))).Methods("POST")
	router.Handle("/game/hex/{instance_id}/attack", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.AttackHexMonsterHandler))).Methods("POST")
	// === websocket ===
	router.HandleFunc("/ws", handlers.WsHandler)
	router.Handle("/game/chat/messages", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ChatSendHandler))).Methods("POST")
	router.Handle("/game/chat/messages", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ChatHistoryHandler))).Methods("GET")
	router.Handle("/game/chat/conversations", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ChatConversationsHandler))).Methods("GET", "POST")
	router.Handle("/game/chat/conversations/{id}/messages", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ChatConversationMessagesHandler))).Methods("GET")
	router.Handle("/game/chat/conversations/{id}/read", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ChatConversationReadHandler))).Methods("POST")
	router.Handle("/game/chat/conversations/{id}/leave", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ChatConversationLeaveHandler))).Methods("POST")
	router.Handle("/game/chat/players", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ChatPlayerSearchHandler))).Methods("GET")
	router.Handle("/game/chat/online", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ChatOnlinePlayersHandler))).Methods("GET")
	router.Handle("/game/chat/mutes", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ChatMutesHandler))).Methods("GET")
	router.Handle("/game/chat/mutes/{id}", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ChatMuteTargetHandler))).Methods("PUT", "DELETE")

	router.Handle("/game/market/{kind}", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.MarketReadHandler))).Methods("GET")
	router.Handle("/game/market/{kind}/{action}", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.MarketActionHandler))).Methods("POST")
	router.Handle("/game/market/{kind}/{id}/{action}", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.MarketActionHandler))).Methods("POST")

	// === Эндпоинты для игроков ===
	router.HandleFunc("/create/player", handlers.CreatePlayerHandler).Methods("POST")

	router.HandleFunc("/game/player/{id}", handlers.GetPlayerHandler).Methods("GET")

	router.HandleFunc("/game/player/{id}/gain_experience", handlers.GainExperienceHandler).Methods("POST")
	router.HandleFunc("/game/shop/items", handlers.GetShopItemsHandler).Methods("GET")
	router.Handle(
		"/game/shop/buy",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.BuyShopItemHandler)),
	).Methods("POST")
	router.Handle(
		"/game/storage",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.HexStorageHandler)),
	).Methods("GET", "PUT")
	router.Handle("/game/professions", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetProfessionsHandler))).Methods("GET")
	router.Handle("/game/professions/choose", middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ChooseProfessionHandler))).Methods("POST")
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
		"/game/base/tavern/build",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.BuildTavernHandler)),
	).Methods("POST")
	router.Handle(
		"/game/heroes",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetHeroesHandler)),
	).Methods("GET")
	router.Handle(
		"/game/heroes/{heroClassId}/hire",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.HireHeroHandler)),
	).Methods("POST")
	router.Handle(
		"/game/heroes/active",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.SetActiveHeroHandler)),
	).Methods("POST")
	router.Handle(
		"/game/equipment",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetEquipmentHandler)),
	).Methods("GET")
	router.Handle(
		"/game/equipment/equip",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.EquipItemHandler)),
	).Methods("POST")
	router.Handle(
		"/game/equipment/unequip",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.UnequipItemHandler)),
	).Methods("POST")
	router.Handle(
		"/game/equipment/sell",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.SellEquipmentItemHandler)),
	).Methods("POST")
	router.Handle(
		"/game/equipment/discard",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.DiscardEquipmentItemHandler)),
	).Methods("POST")
	router.Handle(
		"/game/equipment/grant-sagecloth-dev",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GrantSageclothDevHandler)),
	).Methods("POST")
	router.Handle(
		"/game/equipment/grant-item-dev",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GrantItemDevHandler)),
	).Methods("POST")
	router.Handle(
		"/game/equipment/dev/grant-class-set",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GrantClassSetDevHandler)),
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

	// === Эндпоинты для матчей ===
	router.HandleFunc("/game/createMatch", handlers.CreateMatchHandler).Methods("POST")
	router.Handle(
		"/game/matches/history",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.ListCompletedMatchesHandler)),
	).Methods("GET")
	router.Handle(
		"/game/matches/history/{instance_id}",
		middleware.GameAuthMiddleware(jwtSecretKey, http.HandlerFunc(handlers.GetCompletedMatchDetailsHandler)),
	).Methods("GET")

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
