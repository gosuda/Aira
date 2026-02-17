package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/cors"
	slacklib "github.com/slack-go/slack"

	"github.com/gosuda/aira/internal/agent"
	"github.com/gosuda/aira/internal/api/ws"
	"github.com/gosuda/aira/internal/auth"
	"github.com/gosuda/aira/internal/config"
	"github.com/gosuda/aira/internal/messenger"
	airaslack "github.com/gosuda/aira/internal/messenger/slack"
	"github.com/gosuda/aira/internal/server/middleware"
	"github.com/gosuda/aira/internal/store/postgres"
	redisstore "github.com/gosuda/aira/internal/store/redis"
)

// Server is the HTTP server that wires all application routes and middleware.
type Server struct {
	router          chi.Router
	httpServer      *http.Server
	store           *postgres.Store
	auth            *auth.Service
	pubsub          *redisstore.PubSub
	wsHub           *ws.Hub
	orchestrator    *agent.Orchestrator
	messengerRouter *messenger.Router // nil when Slack is not configured
	cfg             *config.Config
}

// New creates a Server with all routes wired.
// webAssets may be nil; when provided, the SvelteKit SPA is served on all
// unmatched routes (embedded via go:embed for single-binary distribution).
func New(cfg *config.Config, store *postgres.Store, pubsub *redisstore.PubSub, authSvc *auth.Service, orchestrator *agent.Orchestrator, webAssets fs.FS) *Server {
	router := chi.NewRouter()

	// Global middleware stack.
	router.Use(chimw.RequestID)
	router.Use(chimw.RealIP)
	router.Use(chimw.Logger)
	router.Use(chimw.Recoverer)
	router.Use(cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}).Handler)

	hub := ws.NewHub(pubsub)

	s := &Server{
		router:       router,
		store:        store,
		auth:         authSvc,
		pubsub:       pubsub,
		wsHub:        hub,
		orchestrator: orchestrator,
		cfg:          cfg,
		httpServer: &http.Server{
			Addr:         cfg.Server.Addr,
			Handler:      router,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		},
	}

	// Mount API routes on /api/v1 with two sub-groups:
	// 1. Unauthenticated group for auth endpoints.
	// 2. Authenticated group for all other endpoints.
	router.Route("/api/v1", func(r chi.Router) {
		// Unauthenticated auth routes (register, login, refresh).
		r.Group(func(r chi.Router) {
			authConfig := huma.DefaultConfig("Aira Auth API", "1.0.0")
			authConfig.Servers = []*huma.Server{
				{URL: "/api/v1"},
			}
			authAPI := humachi.New(r, authConfig)
			registerAuthRoutes(authAPI, store, authSvc)
		})

		// Authenticated routes (everything else).
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(cfg.JWT.Secret, store.Users()))
			r.Use(middleware.RequireTenant())
			r.Use(middleware.RateLimit(100, 200))

			apiConfig := huma.DefaultConfig("Aira API", "1.0.0")
			apiConfig.Servers = []*huma.Server{
				{URL: "/api/v1"},
			}
			api := humachi.New(r, apiConfig)
			registerAPIRoutes(api, store, orchestrator)
		})
	})

	// WebSocket routes.
	router.Route("/ws", func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWT.Secret, store.Users()))
		registerWSRoutes(r, hub)
	})

	// Slack webhook routes: real handler if configured, 501 placeholder otherwise.
	router.Route("/slack", func(r chi.Router) {
		slackHandler := s.buildSlackHandler(cfg, store, orchestrator)
		if slackHandler != nil {
			registerSlackRoutes(r, slackHandler)
		} else {
			r.Post("/events", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotImplemented)
			})
			r.Post("/interactions", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotImplemented)
			})
		}
	})

	// Health check (unauthenticated).
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Serve embedded SvelteKit SPA on all unmatched routes.
	// This must be the last route registered so API/WS/Slack routes take priority.
	if webAssets != nil {
		router.NotFound(spaFileServer(webAssets).ServeHTTP)
		slog.Info("embedded SvelteKit dashboard enabled")
	}

	return s
}

// buildSlackHandler creates the Slack handler stack when Slack is configured.
// Returns nil if Slack signing secret is not set.
func (s *Server) buildSlackHandler(cfg *config.Config, store *postgres.Store, orchestrator *agent.Orchestrator) *airaslack.Handler {
	if cfg.Slack.SigningSecret == "" {
		return nil
	}

	// Build the Slack messenger from the bot token.
	slackClient := slacklib.New(cfg.Slack.BotToken)
	slackMessenger := airaslack.NewSlackMessenger(slackClient)

	// Build the messenger router that bridges HITL questions to Slack threads.
	msgRouter := messenger.NewRouter(
		store.HITL(),
		slackMessenger,
		orchestrator.HandleHITLResponse,
	)
	s.messengerRouter = msgRouter

	// Build the response adapter that maps Slack user IDs to internal users.
	adapter := &slackResponseAdapter{
		router:   msgRouter,
		userRepo: store.Users(),
	}

	// Single-tenant mode: use a fixed tenant ID.
	// In production, this would be resolved per-workspace via Slack team ID.
	tenantID := uuid.Nil

	handler := airaslack.NewHandler(cfg.Slack.SigningSecret, adapter, tenantID)

	slog.Info("Slack integration enabled")

	return handler
}

// Start begins listening for HTTP requests.
func (s *Server) Start(_ context.Context) error {
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server.Start: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server.Shutdown: %w", err)
	}
	return nil
}
