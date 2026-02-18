package main

import (
	"context"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/gosuda/aira/internal/agent"
	"github.com/gosuda/aira/internal/agent/backends"
	"github.com/gosuda/aira/internal/auth"
	"github.com/gosuda/aira/internal/config"
	"github.com/gosuda/aira/internal/server"
	"github.com/gosuda/aira/internal/store/postgres"
	redisstore "github.com/gosuda/aira/internal/store/redis"
	"github.com/gosuda/aira/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("startup failed")
	}
}

func run() error {
	// Initialize structured logging from environment.
	logLevel := os.Getenv("AIRA_LOG_LEVEL")
	level, parseErr := zerolog.ParseLevel(logLevel)
	if parseErr != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	logFormat := os.Getenv("AIRA_LOG_FORMAT")
	if logFormat == "text" {
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	} else {
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

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

	// Create Docker runtime for agent containers.
	dockerRuntime, err := agent.NewDockerRuntime(
		cfg.Docker.Host,
		cfg.Docker.ImageDefault,
		cfg.Docker.CPULimit,
		cfg.Docker.MemLimit,
	)
	if err != nil {
		return fmt.Errorf("docker runtime: %w", err)
	}
	defer dockerRuntime.Close()

	// Create agent registry and register backends.
	registry := agent.NewRegistry()
	registry.Register("claude", backends.NewClaudeBackend)
	registry.Register("codex", backends.NewCodexBackend)
	registry.Register("opencode", backends.NewOpenCodeBackend)
	registry.Register("acp", backends.NewACPBackend)

	// Create volume manager using Docker client.
	volumes := agent.NewVolumeManager(dockerRuntime.Client())

	// Create orchestrator.
	orchestrator := agent.NewOrchestrator(
		registry,
		dockerRuntime,
		volumes,
		store.AgentSessions(),
		store.Tasks(),
		store.Projects(),
		store.ADRs(),
		pubsub,
	)

	// Prepare embedded SvelteKit assets (strip "build/" prefix from fs paths).
	webAssets, err := fs.Sub(web.Assets, "build")
	if err != nil {
		return fmt.Errorf("web assets: %w", err)
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create HTTP server with all routes wired.
	srv := server.New(ctx, cfg, store, pubsub, authSvc, orchestrator, webAssets)

	// Start server in background goroutine.
	go func() {
		log.Info().Str("addr", cfg.Server.Addr).Msg("starting server")
		if startErr := srv.Start(ctx); startErr != nil {
			log.Error().Err(startErr).Msg("server error")
		}
	}()

	// Block until shutdown signal.
	<-ctx.Done()
	log.Info().Msg("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
		return shutdownErr
	}

	log.Info().Msg("stopped")
	return nil
}
