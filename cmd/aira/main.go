package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gosuda/aira/internal/auth"
	"github.com/gosuda/aira/internal/config"
	"github.com/gosuda/aira/internal/server"
	"github.com/gosuda/aira/internal/store/postgres"
	redisstore "github.com/gosuda/aira/internal/store/redis"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("aira: %v", err)
	}
}

func run() error {
	ctx := context.Background()

	// Load configuration from environment.
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.Database.MaxConns < 0 || cfg.Database.MaxConns > math.MaxInt32 {
		return fmt.Errorf("database max_conns %d out of int32 range", cfg.Database.MaxConns)
	}

	// Connect to PostgreSQL.
	store, err := postgres.New(ctx, cfg.Database.DSN(), int32(cfg.Database.MaxConns)) //nolint:gosec // bounds checked above
	if err != nil {
		return err
	}
	defer store.Close()

	// Connect to Redis.
	pubsub, err := redisstore.New(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		return err
	}
	defer pubsub.Close()

	// Create auth service.
	authSvc := auth.NewService(store.Users(), cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)

	// Create HTTP server with all routes wired.
	srv := server.New(cfg, store, pubsub, authSvc)

	// Graceful shutdown on SIGINT / SIGTERM.
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start server in background goroutine.
	go func() {
		log.Printf("aira: starting server on %s", cfg.Server.Addr)
		if startErr := srv.Start(ctx); startErr != nil {
			log.Printf("aira: server error: %v", startErr)
		}
	}()

	// Block until shutdown signal.
	<-ctx.Done()
	log.Println("aira: shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
		return shutdownErr
	}

	log.Println("aira: stopped")
	return nil
}
