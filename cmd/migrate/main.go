package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const defaultDB = "postgres://postgres:postgres@localhost:5432/mercor?sslmode=disable"

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = defaultDB
	}

	// Directory with *.sql files relative to the working directory
	const sourceURL = "file://migrations"

	m, err := migrate.New(sourceURL, dbURL)
	if err != nil {
		log.Fatalf("cannot create migrate instance: %v", err)
	}
	defer m.Close() //nolint:errcheck

	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migrate up: %v", err)
		}
		fmt.Println("✅ migrations applied (up)")

	case "down":
		// Roll back ONE step; change to m.Down() to roll back everything
		if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migrate down: %v", err)
		}
		fmt.Println("✅ migrations rolled back (down one)")

	default:
		usage()
	}
}

func usage() {
	fmt.Println("Usage: go run cmd/migrate <up|down>")
	os.Exit(1)
}
