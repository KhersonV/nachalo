
//===================
//auth/contextkeys.go
//===================

package common

// ContextKey определяет тип для ключей контекста.
type ContextKey string

// UserIDKey – ключ для хранения идентификатора пользователя.
const UserIDKey ContextKey = "user_id"
