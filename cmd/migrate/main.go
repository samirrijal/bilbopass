package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/samirrijal/bilbopass/internal/pkg/config"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: migrate <up|down>")
	}

	cfg, err := config.Load("bilbopass-migrate")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.Database.DSN())
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	switch os.Args[1] {
	case "up":
		runMigrations(ctx, pool)
	case "down":
		log.Println("down migration not yet implemented")
	default:
		log.Fatalf("unknown command: %s", os.Args[1])
	}
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) {
	files := []string{
		"migrations/001_init_extensions.sql",
		"migrations/002_core_tables.sql",
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("read %s: %v", f, err)
		}

		_, err = pool.Exec(ctx, string(data))
		if err != nil {
			log.Fatalf("exec %s: %v", f, err)
		}

		fmt.Printf("OK  %s\n", f)
	}

	log.Println("all migrations applied")
}
