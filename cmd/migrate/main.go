package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/ChargePi/chargeflow-registry/deployments/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	dsn := os.Getenv("CHARGEFLOW_REGISTRY_DATABASE_DSN")
	if dsn == "" {
		log.Fatal("CHARGEFLOW_REGISTRY_DATABASE_DSN is required")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("failed to open database connection: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("failed to set goose dialect: %v", err)
	}

	if err := goose.Up(db, "."); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	log.Println("migrations applied successfully")
}
