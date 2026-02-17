package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"

	"github.com/gosuda/aira/internal/auth"
	"github.com/gosuda/aira/internal/config"
	"github.com/gosuda/aira/internal/server/middleware"
	"github.com/gosuda/aira/internal/store/postgres"
	redisstore "github.com/gosuda/aira/internal/store/redis"
)

type Server struct {
	router     chi.Router
	httpServer *http.Server
	store      *postgres.Store
	auth       *auth.Service
	pubsub     *redisstore.PubSub
	cfg        *config.Config
}

func New(cfg *config.Config, store *postgres.Store, pubsub *redisstore.PubSub, authSvc *auth.Service) *Server {
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

	s := &Server{
		router: router,
		store:  store,
		auth:   authSvc,
		pubsub: pubsub,
		cfg:    cfg,
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
			registerAPIRoutes(api, store)
		})
	})

	// WebSocket routes (placeholder).
	router.Route("/ws", func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWT.Secret, store.Users()))
		registerWSRoutes(r)
	})

	// Slack webhook routes (placeholder).
	router.Route("/slack", func(r chi.Router) {
		registerSlackRoutes(r)
	})

	// Health check (unauthenticated).
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	return s
}

func (s *Server) Start(_ context.Context) error {
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server.Start: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server.Shutdown: %w", err)
	}
	return nil
}
