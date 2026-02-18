package ws

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/server/middleware"
	redisstore "github.com/gosuda/aira/internal/store/redis"
)

// Hub manages WebSocket connections backed by Redis pub/sub.
type Hub struct {
	pubsub *redisstore.PubSub
}

// NewHub creates a new WebSocket hub.
func NewHub(pubsub *redisstore.PubSub) *Hub {
	return &Hub{pubsub: pubsub}
}

// ServeBoard handles WebSocket connections for kanban board updates.
// Subscribes to Redis channel "board:<tenantID>:<projectID>".
// Sends task state changes to connected clients.
func (h *Hub) ServeBoard(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		http.Error(w, "missing tenant", http.StatusBadRequest)
		return
	}

	projectIDStr := chi.URLParam(r, "projectID")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("websocket accept")
		return
	}
	defer conn.CloseNow()

	ctx := r.Context()
	channel := redisstore.BoardChannel(tenantID, projectID)

	messages, cleanup, err := h.pubsub.Subscribe(ctx, channel)
	if err != nil {
		log.Error().Err(err).Msg("websocket subscribe")
		_ = conn.Close(websocket.StatusInternalError, "subscribe failed")
		return
	}
	defer cleanup()

	for {
		select {
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusNormalClosure, "connection closed")
			return
		case msg, msgOK := <-messages:
			if !msgOK {
				_ = conn.Close(websocket.StatusNormalClosure, "channel closed")
				return
			}
			if writeErr := conn.Write(ctx, websocket.MessageText, msg); writeErr != nil {
				log.Debug().Err(writeErr).Msg("websocket write")
				return
			}
		}
	}
}

// ServeAgent handles WebSocket connections for agent session output.
// Subscribes to Redis channel "agent:<sessionID>".
// Streams agent output lines to connected clients.
func (h *Hub) ServeAgent(w http.ResponseWriter, r *http.Request) {
	sessionIDStr := chi.URLParam(r, "sessionID")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("websocket accept")
		return
	}
	defer conn.CloseNow()

	ctx := r.Context()
	channel := redisstore.AgentChannel(sessionID)

	messages, cleanup, err := h.pubsub.Subscribe(ctx, channel)
	if err != nil {
		log.Error().Err(err).Msg("websocket subscribe")
		_ = conn.Close(websocket.StatusInternalError, "subscribe failed")
		return
	}
	defer cleanup()

	for {
		select {
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusNormalClosure, "connection closed")
			return
		case msg, msgOK := <-messages:
			if !msgOK {
				_ = conn.Close(websocket.StatusNormalClosure, "channel closed")
				return
			}
			if writeErr := conn.Write(ctx, websocket.MessageText, msg); writeErr != nil {
				log.Debug().Err(writeErr).Msg("websocket write")
				return
			}
		}
	}
}

// Publish sends an event payload to a Redis channel. This is a convenience
// wrapper for use by API handlers when mutating board or agent state.
func (h *Hub) Publish(ctx context.Context, channel string, payload []byte) error {
	if err := h.pubsub.Publish(ctx, channel, payload); err != nil {
		return fmt.Errorf("ws.Hub.Publish: %w", err)
	}
	return nil
}
