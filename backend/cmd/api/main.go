package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"kalkurasa/backend/internal/config"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/server"
)

func main() {
	// Load .env if present (development only); never overwrite existing env vars.
	// Silently ignored if file does not exist (e.g. production on Vercel).
	_ = godotenv.Load() // looks for .env in working directory

	cfg := config.Load()

	pool := initDB(cfg)
	if pool != nil {
		defer pool.Close()
	}

	r := server.New(cfg, pool)

	// In development, bind to loopback only; in production bind to all interfaces.
	addr := ":" + cfg.Port
	if cfg.AppEnv == "development" {
		addr = "127.0.0.1:" + cfg.Port
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}
	}()

	log.Printf("Kalkurasa API listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
}

func initDB(cfg *config.Config) *pgxpool.Pool {
	if cfg.DatabaseURL == "" {
		log.Println("WARNING: DATABASE_URL not set — running without database")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.New(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		// Log error without exposing the DSN or password.
		log.Printf("WARNING: Could not connect to database: %v", err)
		return nil
	}

	log.Println("Database connected")
	return pool
}
