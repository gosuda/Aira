package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	slacklib "github.com/slack-go/slack"
)

// ResponseHandler dispatches answers received from Slack back to the HITL router.
type ResponseHandler interface {
	HandleSlackResponse(ctx context.Context, tenantID uuid.UUID, threadTS, answer, slackUserID string) error
}

// Handler processes Slack webhook events (Events API + Interactive Components).
type Handler struct {
	signingSecret string
	responder     ResponseHandler
	tenantID      uuid.UUID
}

// NewHandler creates a new Slack webhook handler.
func NewHandler(signingSecret string, responder ResponseHandler, tenantID uuid.UUID) *Handler {
	return &Handler{
		signingSecret: signingSecret,
		responder:     responder,
		tenantID:      tenantID,
	}
}

// slackEvent represents the outer envelope of Slack Events API payloads.
type slackEvent struct {
	Type      string          `json:"type"`
	Challenge string          `json:"challenge,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"`
}

// innerEvent represents the inner event within an event_callback.
type innerEvent struct {
	Type     string `json:"type"`
	Channel  string `json:"channel"`
	ThreadTS string `json:"thread_ts,omitempty"`
	Text     string `json:"text"`
	User     string `json:"user"`
	BotID    string `json:"bot_id,omitempty"`
}

// HandleEvents is an http.HandlerFunc for POST /slack/events.
func (h *Handler) HandleEvents(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	if verifyErr := h.verifySignature(r.Header, body); verifyErr != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var envelope slackEvent
	if unmarshalErr := json.Unmarshal(body, &envelope); unmarshalErr != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	switch envelope.Type {
	case "url_verification":
		h.handleURLVerification(w, envelope.Challenge)
		return
	case "event_callback":
		h.handleEventCallback(r.Context(), w, envelope.Event)
		return
	default:
		w.WriteHeader(http.StatusOK)
	}
}

// handleURLVerification responds to Slack's URL verification challenge.
func (h *Handler) handleURLVerification(w http.ResponseWriter, challenge string) {
	w.Header().Set("Content-Type", "application/json")

	resp := map[string]string{"challenge": challenge}
	if encodeErr := json.NewEncoder(w).Encode(resp); encodeErr != nil {
		slog.Error("encode url verification response", "error", encodeErr)
	}
}

// handleEventCallback processes an event_callback payload.
func (h *Handler) handleEventCallback(ctx context.Context, w http.ResponseWriter, rawEvent json.RawMessage) {
	var evt innerEvent
	if unmarshalErr := json.Unmarshal(rawEvent, &evt); unmarshalErr != nil {
		http.Error(w, "invalid event JSON", http.StatusBadRequest)
		return
	}

	// Only handle threaded human messages.
	if evt.Type != "message" || evt.ThreadTS == "" || evt.BotID != "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if respondErr := h.responder.HandleSlackResponse(ctx, h.tenantID, evt.ThreadTS, evt.Text, evt.User); respondErr != nil {
		slog.Error("dispatch event callback response", "error", respondErr)
	}

	w.WriteHeader(http.StatusOK)
}

// HandleInteractions is an http.HandlerFunc for POST /slack/interactions.
func (h *Handler) HandleInteractions(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	if verifyErr := h.verifySignature(r.Header, body); verifyErr != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// Interactions use form-encoded body; the payload is in the "payload" field.
	// We already consumed the body for signature verification, so re-create it
	// and let the stdlib parse the form.
	r.Body = io.NopCloser(bytes.NewReader(body))

	parseErr := r.ParseForm()
	if parseErr != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	payloadStr := r.FormValue("payload")
	if payloadStr == "" {
		// Fallback: manually URL-decode from raw body.
		payloadStr = extractFormPayload(string(body))
	}

	if payloadStr == "" {
		http.Error(w, "missing payload", http.StatusBadRequest)
		return
	}

	var callback slacklib.InteractionCallback
	if unmarshalErr := json.Unmarshal([]byte(payloadStr), &callback); unmarshalErr != nil {
		http.Error(w, "invalid payload JSON", http.StatusBadRequest)
		return
	}

	// Extract the action value and thread timestamp.
	actionValue := extractActionValue(&callback)
	threadTS := extractInteractionThreadTS(&callback)
	userID := callback.User.ID

	if actionValue == "" || threadTS == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if respondErr := h.responder.HandleSlackResponse(r.Context(), h.tenantID, threadTS, actionValue, userID); respondErr != nil {
		slog.Error("dispatch interaction response", "error", respondErr)
	}

	w.WriteHeader(http.StatusOK)
}

// verifySignature validates the Slack request signature using the signing secret.
func (h *Handler) verifySignature(header http.Header, body []byte) error {
	sv, err := slacklib.NewSecretsVerifier(header, h.signingSecret)
	if err != nil {
		return fmt.Errorf("slack.Handler.verifySignature: create verifier: %w", err)
	}

	if _, writeErr := sv.Write(body); writeErr != nil {
		return fmt.Errorf("slack.Handler.verifySignature: write body: %w", writeErr)
	}

	if ensureErr := sv.Ensure(); ensureErr != nil {
		return fmt.Errorf("slack.Handler.verifySignature: ensure: %w", ensureErr)
	}

	return nil
}

// extractActionValue pulls the first action value from an interaction callback.
func extractActionValue(callback *slacklib.InteractionCallback) string {
	if len(callback.ActionCallback.BlockActions) > 0 {
		return callback.ActionCallback.BlockActions[0].Value
	}
	if len(callback.ActionCallback.AttachmentActions) > 0 {
		return callback.ActionCallback.AttachmentActions[0].Value
	}

	return ""
}

// extractInteractionThreadTS pulls the thread timestamp from an interaction callback.
func extractInteractionThreadTS(callback *slacklib.InteractionCallback) string {
	if callback.Container.ThreadTs != "" {
		return callback.Container.ThreadTs
	}
	if callback.Message.ThreadTimestamp != "" {
		return callback.Message.ThreadTimestamp
	}

	return ""
}

// extractFormPayload parses the "payload" value from a URL-encoded form body.
func extractFormPayload(body string) string {
	values, err := url.ParseQuery(body)
	if err != nil {
		return ""
	}

	return values.Get("payload")
}
