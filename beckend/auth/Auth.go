
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
	"time"

	_ "github.com/lib/pq"

	"github.com/golang-jwt/jwt/v4"
	"github.com/rs/cors"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecretKey = os.Getenv("JWT_SECRET_KEY")

const connStr = "user=admin password=yourpassword dbname=admin sslmode=disable"

// User описывает структуру пользователя, соответствующую таблице в БД
type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	Rating       int       `json:"rating"`
	CreatedAt    time.Time `json:"created_at"`
}

// LoginResponse – расширенный тип для ответа на логин, включает токен
type LoginResponse struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Rating    int       `json:"rating"`
	CreatedAt time.Time `json:"created_at"`
	Token     string    `json:"token"`
}

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatalf("Невозможно подключиться к БД: %v", err)
	}
	log.Println("Подключение к БД установлено")
}

func createUsersTable() {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		rating INT DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы пользователей: %v", err)
	}
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
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
	query := `INSERT INTO users (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id`
	err = db.QueryRow(query, email, string(hashedPassword), name).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("ошибка создания пользователя: %w", err)
	}
	return userID, nil
}

func getUserByEmail(email string) (*User, error) {
	user := &User{}
	query := `SELECT id, email, password_hash, name, rating, created_at FROM users WHERE email=$1`
	row := db.QueryRow(query, email)
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Rating, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func getUserByID(userID int) (*User, error) {
	user := &User{}
	query := `SELECT id, email, name, rating, created_at FROM users WHERE id=$1`
	row := db.QueryRow(query, userID)
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.Rating, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// Обработчик регистрации
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

	// После регистрации можно вызвать Game-сервис для создания персонажа.
	payload := map[string]int{"user_id": userID}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Ошибка маршаллинга для Game Service: %v", err)
	} else {
		resp, err := http.Post("http://localhost:8001/create/player", "application/json", bytes.NewBuffer(jsonData))
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

// Обработчик логина, возвращающий данные пользователя вместе с токеном.
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

	// Создаем JWT-токен с данными пользователя
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
		Rating:    user.Rating,
		CreatedAt: user.CreatedAt,
		Token:     tokenString,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loginResp)
}

type contextKey string

const userIDKey contextKey = "user_id"

// Middleware для проверки JWT
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
			ctx := context.WithValue(r.Context(), userIDKey, int(userID))
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			http.Error(w, "Неверные данные токена", http.StatusUnauthorized)
			return
		}
	})
}

// Обработчик получения профиля пользователя
func profileHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(int)
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

func main() {
	jwtSecretKey = os.Getenv("JWT_SECRET_KEY")
	if jwtSecretKey == "" {
		log.Fatal("JWT_SECRET_KEY environment variable is not set")
	}
	log.Println("JWT_SECRET_KEY загружен:", jwtSecretKey)
	initDB()
	defer db.Close()
	createUsersTable()

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/register", registerHandler)
	mux.HandleFunc("/auth/login", loginHandler)
	mux.Handle("/auth/profile", authMiddleware(http.HandlerFunc(profileHandler)))

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
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
