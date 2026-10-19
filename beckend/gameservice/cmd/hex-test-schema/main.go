// hex-test-schema creates only the existing base schema in a local test DB.
// Stage 1 and Stage 2 SQL are still applied manually by the developer.
package main

import (
	"database/sql"
	"log"
	"net/url"
	"os"
	"strings"

	"gameservice/repository"
)

func main() {
	dsn := os.Getenv("HEX_TEST_DB_DSN")
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || !strings.Contains(strings.TrimPrefix(u.Path, "/"), "test") {
		log.Fatal("HEX_TEST_DB_DSN must be a local PostgreSQL URL whose database name contains 'test'")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	repository.DB = db
	repository.RunMigrations()
	log.Println("base schema ready; apply stage1_geometry.sql and stage2_commands.sql manually")
}
