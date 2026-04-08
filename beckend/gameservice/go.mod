module gameservice

go 1.24.0

require (
	auth v0.0.0
	github.com/gorilla/mux v1.8.1
	github.com/lib/pq v1.10.9

)

require github.com/golang-jwt/jwt/v4 v4.5.1

require github.com/gorilla/websocket v1.5.3

replace auth => ../auth
