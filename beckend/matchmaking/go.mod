module matchmaking

go 1.24.0

require (
	gameservice v0.0.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
)

require github.com/lib/pq v1.10.9 // indirect

replace gameservice => ../gameservice
