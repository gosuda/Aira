package server

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/gosuda/aira/internal/agent"
	v1 "github.com/gosuda/aira/internal/api/v1"
	"github.com/gosuda/aira/internal/api/ws"
	"github.com/gosuda/aira/internal/auth"
	airaslack "github.com/gosuda/aira/internal/messenger/slack"
	"github.com/gosuda/aira/internal/store/postgres"
)

func registerAuthRoutes(api huma.API, store *postgres.Store, authSvc *auth.Service) {
	v1.RegisterAuthRoutes(api, store, authSvc)
}

func registerAPIRoutes(api huma.API, store *postgres.Store, orchestrator *agent.Orchestrator) {
	v1.RegisterTenantRoutes(api, store)
	v1.RegisterProjectRoutes(api, store)
	v1.RegisterTaskRoutes(api, store)
	v1.RegisterADRRoutes(api, store)
	v1.RegisterAgentRoutes(api, store, orchestrator)
	v1.RegisterBoardRoutes(api, store)
}

func registerWSRoutes(r chi.Router, hub *ws.Hub) {
	r.Get("/board/{projectID}", hub.ServeBoard)
	r.Get("/agent/{sessionID}", hub.ServeAgent)
}

func registerSlackRoutes(r chi.Router, handler *airaslack.Handler) {
	r.Post("/events", handler.HandleEvents)
	r.Post("/interactions", handler.HandleInteractions)
}
