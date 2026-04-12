//================
//auth/Auth.go
//================

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"auth/common"
	"github.com/golang-jwt/jwt/v4"
	"github.com/rs/cors"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecretKey = os.Getenv("JWT_SECRET_KEY")
var connStr = os.Getenv("AUTH_DB_DSN")
var gameServiceURL = os.Getenv("GAME_SERVICE_URL")
var frontendOrigin = os.Getenv("FRONTEND_ORIGIN")

type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
}

type LoginResponse struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Token     string    `json:"token"`
}

var db *sql.DB

const dbConnectTimeout = 90 * time.Second
const dbConnectRetryInterval = 2 * time.Second

func initDB() {
	var err error

	if connStr == "" {
		log.Fatal("AUTH_DB_DSN environment variable is not set")
	}

	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}

	deadline := time.Now().Add(dbConnectTimeout)
	for {
		err = db.Ping()
		if err == nil {
			log.Println("Подключение к БД установлено")
			return
		}
		if time.Now().After(deadline) {
			log.Fatalf("Невозможно подключиться к БД: %v", err)
		}
		log.Printf("Ожидание готовности БД auth: %v", err)
		time.Sleep(dbConnectRetryInterval)
	}
}

func createUsersTable() {
	query := `
	CREATE TABLE IF NOT EXISTS public.users (
		id SERIAL PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы пользователей: %v", err)
	}

	log.Println("Таблица users успешно создана (или уже существует)")
}

type RegisterRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	Name          string `json:"name"`
	Image         string `json:"image"`
	CharacterType string `json:"characterType"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func createUser(email, password, name string) (int, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("ошибка хэширования пароля: %w", err)
	}

	var userID int
	query := `INSERT INTO public.users (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id`

	err = db.QueryRow(query, email, string(hashedPassword), name).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("ошибка создания пользователя: %w", err)
	}

	return userID, nil
}

func getUserByEmail(email string) (*User, error) {
	user := &User{}
	query := `SELECT id, email, password_hash, name, created_at FROM public.users WHERE email=$1`

	row := db.QueryRow(query, email)
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.CreatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func getUserByID(userID int) (*User, error) {
	user := &User{}
	query := `SELECT id, email, name, created_at FROM public.users WHERE id=$1`

	row := db.QueryRow(query, userID)
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" || req.Name == "" {
		http.Error(w, "Все поля (email, password, name) обязательны", http.StatusBadRequest)
		return
	}

	userID, err := createUser(req.Email, req.Password, req.Name)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка регистрации: %v", err), http.StatusInternalServerError)
		return
	}

	payload := map[string]interface{}{
		"user_id":        userID,
		"name":           req.Name,
		"image":          req.Image,
		"character_type": req.CharacterType,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Ошибка маршаллинга для Game Service: %v", err)
	} else {
		resp, err := http.Post(gameServiceURL+"/create/player", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("Ошибка создания персонажа в Game Service: %v", err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				log.Printf("Game Service вернул статус: %v", resp.Status)
			} else {
				log.Printf("Персонаж для пользователя %d успешно создан в Game Service", userID)
			}
		}
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Пользователь успешно зарегистрирован"))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email и password обязательны", http.StatusBadRequest)
		return
	}

	user, err := getUserByEmail(req.Email)
	if err != nil {
		http.Error(w, "Пользователь не найден", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Неверный пароль", http.StatusUnauthorized)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(6 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(jwtSecretKey))
	if err != nil {
		http.Error(w, "Ошибка генерации токена", http.StatusInternalServerError)
		return
	}

	loginResp := LoginResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt,
		Token:     tokenString,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loginResp)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Отсутствует токен", http.StatusUnauthorized)
			return
		}

		var tokenString string
		_, err := fmt.Sscanf(authHeader, "Bearer %s", &tokenString)
		if err != nil || tokenString == "" {
			http.Error(w, "Неверный формат токена", http.StatusUnauthorized)
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("неожиданный метод подписи: %v", token.Header["alg"])
			}
			return []byte(jwtSecretKey), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Неверный или просроченный токен", http.StatusUnauthorized)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			userID, ok := claims["user_id"].(float64)
			if !ok {
				http.Error(w, "Неверный токен", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), common.UserIDKey, int(userID))
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			http.Error(w, "Неверные данные токена", http.StatusUnauthorized)
			return
		}
	})
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(common.UserIDKey).(int)
	if !ok {
		http.Error(w, "Не удалось определить пользователя", http.StatusUnauthorized)
		return
	}

	user, err := getUserByID(userID)
	if err != nil {
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func authReady() error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return err
	}

	var exists bool
	if err := db.QueryRowContext(
		ctx,
		`SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'users'
		)`,
	).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("users table is not ready")
	}

	return nil
}

func main() {
	jwtSecretKey = os.Getenv("JWT_SECRET_KEY")
	connStr = os.Getenv("AUTH_DB_DSN")
	gameServiceURL = os.Getenv("GAME_SERVICE_URL")
	frontendOrigin = os.Getenv("FRONTEND_ORIGIN")

	if jwtSecretKey == "" {
		log.Fatal("JWT_SECRET_KEY environment variable is not set")
	}
	if connStr == "" {
		log.Fatal("AUTH_DB_DSN environment variable is not set")
	}
	if gameServiceURL == "" {
		gameServiceURL = "http://gameservice:8001"
	}
	if frontendOrigin == "" {
		frontendOrigin = "http://localhost:3000"
	}

	initDB()
	defer db.Close()
	createUsersTable()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := authReady(); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/auth/register", registerHandler)
	mux.HandleFunc("/auth/login", loginHandler)
	mux.Handle("/auth/profile", authMiddleware(http.HandlerFunc(profileHandler)))

	allowedOrigins := []string{"http://localhost:3000", "https://*.run.app"}
	for _, origin := range strings.Split(frontendOrigin, ",") {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			allowedOrigins = append(allowedOrigins, trimmed)
		}
	}

	allowedOriginSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin != "" {
			allowedOriginSet[origin] = struct{}{}
		}
	}

	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowOriginFunc: func(origin string) bool {
			if _, ok := allowedOriginSet[origin]; ok {
				return true
			}
			return strings.HasPrefix(origin, "https://") && strings.HasSuffix(origin, ".run.app")
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	})

	handler := c.Handler(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Auth-сервис запущен на порту %s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}
}
