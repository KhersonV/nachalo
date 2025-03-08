
//==============================
//gameservice/middleware/auth.go
//==============================

package middleware

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "strings"

    "github.com/golang-jwt/jwt/v4"
)

// Ключ для хранения user_id в Context
type contextKey string

const userIDKey contextKey = "user_id"

func GameAuthMiddleware(jwtSecretKey string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "Отсутствует токен", http.StatusUnauthorized)
            return
        }
        tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
        if tokenString == "" {
            http.Error(w, "Неверный формат токена", http.StatusUnauthorized)
            return
        }

        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("неожиданный метод подписи: %v", token.Header["alg"])
            }
            return []byte(jwtSecretKey), nil
        })
        if err != nil {
            log.Printf("Ошибка при разборе токена: %v", err)
            http.Error(w, "Неверный или просроченный токен", http.StatusUnauthorized)
            return
        }
        if !token.Valid {
            http.Error(w, "Неверный или просроченный токен", http.StatusUnauthorized)
            return
        }

        if claims, ok := token.Claims.(jwt.MapClaims); ok {
            var uid int
            switch v := claims["user_id"].(type) {
            case float64:
                uid = int(v)
            case int:
                uid = v
            default:
                log.Printf("Неожиданный тип для user_id: %T, значение: %v", claims["user_id"], claims["user_id"])
                http.Error(w, "Неверный токен", http.StatusUnauthorized)
                return
            }
            ctx := context.WithValue(r.Context(), userIDKey, uid)
            next.ServeHTTP(w, r.WithContext(ctx))
        } else {
            http.Error(w, "Неверные данные токена", http.StatusUnauthorized)
            return
        }
    })
}

// Функция, которую можно использовать в хендлерах, чтобы извлечь user_id
func GetUserIDFromContext(ctx context.Context) (int, bool) {
    uid, ok := ctx.Value(userIDKey).(int)
    return uid, ok
}
