module matchmaking

go 1.24.0

require (
	auth v0.0.0
	gameservice v0.0.0
	github.com/google/uuid v1.6.0
	github.com/golang-jwt/jwt/v4 v4.5.1
	github.com/gorilla/mux v1.8.1
)

require github.com/lib/pq v1.10.9 // indirect

replace gameservice => ../gameservice
replace auth => ../auth
