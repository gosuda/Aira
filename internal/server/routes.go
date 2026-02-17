package server

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	v1 "github.com/gosuda/aira/internal/api/v1"
	"github.com/gosuda/aira/internal/auth"
	"github.com/gosuda/aira/internal/store/postgres"
)

func registerAuthRoutes(api huma.API, store *postgres.Store, authSvc *auth.Service) {
	v1.RegisterAuthRoutes(api, store, authSvc)
}

func registerAPIRoutes(api huma.API, store *postgres.Store) {
	v1.RegisterTenantRoutes(api, store)
	v1.RegisterProjectRoutes(api, store)
	v1.RegisterTaskRoutes(api, store)
	v1.RegisterADRRoutes(api, store)
	v1.RegisterAgentRoutes(api, store)
	v1.RegisterBoardRoutes(api, store)
}

func registerWSRoutes(r chi.Router) {
	// Placeholder: WebSocket endpoints.
	r.Get("/board/{projectID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
	r.Get("/agent/{sessionID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
}

func registerSlackRoutes(r chi.Router) {
	// Placeholder: Slack webhook endpoints.
	r.Post("/events", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
	r.Post("/interactions", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
}
