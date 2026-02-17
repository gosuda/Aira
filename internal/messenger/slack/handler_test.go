package slack_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	airaslack "github.com/gosuda/aira/internal/messenger/slack"
)

const testSigningSecret = "test-signing-secret-12345"

// --- mock ResponseHandler ---

type mockResponseHandler struct {
	calls []responseHandlerCall
	err   error
}

type responseHandlerCall struct {
	TenantID    uuid.UUID
	ThreadTS    string
	Answer      string
	SlackUserID string
}

func (m *mockResponseHandler) HandleSlackResponse(_ context.Context, tenantID uuid.UUID, threadTS, answer, slackUserID string) error {
	m.calls = append(m.calls, responseHandlerCall{
		TenantID:    tenantID,
		ThreadTS:    threadTS,
		Answer:      answer,
		SlackUserID: slackUserID,
	})
	return m.err
}

// --- signature helpers ---

// computeSlackSignature computes a valid Slack request signature for the given body and timestamp.
func computeSlackSignature(secret, timestamp, body string) string {
	sigBase := fmt.Sprintf("v0:%s:%s", timestamp, body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sigBase))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func signedJSONRequest(body string) *http.Request {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := computeSlackSignature(testSigningSecret, ts, body)

	req := httptest.NewRequest(http.MethodPost, "/slack/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-Request-Timestamp", ts)
	req.Header.Set("X-Slack-Signature", sig)

	return req
}

func signedFormRequest(formBody string) *http.Request {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := computeSlackSignature(testSigningSecret, ts, formBody)

	req := httptest.NewRequest(http.MethodPost, "/slack/interactions", strings.NewReader(formBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", ts)
	req.Header.Set("X-Slack-Signature", sig)

	return req
}

// --- HandleEvents tests ---

func TestHandleEvents(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()

	t.Run("url_verification challenge", func(t *testing.T) {
		t.Parallel()

		responder := &mockResponseHandler{}
		handler := airaslack.NewHandler(testSigningSecret, responder, tenantID)

		body := `{"type":"url_verification","challenge":"test-challenge-xyz"}`
		req := signedJSONRequest(body)
		rec := httptest.NewRecorder()

		handler.HandleEvents(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var result map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "test-challenge-xyz", result["challenge"])
		assert.Empty(t, responder.calls, "url_verification should not dispatch")
	})

	t.Run("event_callback message in thread dispatches to responder", func(t *testing.T) {
		t.Parallel()

		responder := &mockResponseHandler{}
		handler := airaslack.NewHandler(testSigningSecret, responder, tenantID)

		body := `{
			"type": "event_callback",
			"event": {
				"type": "message",
				"channel": "C123",
				"thread_ts": "1234.5678",
				"text": "use PostgreSQL",
				"user": "U123"
			}
		}`
		req := signedJSONRequest(body)
		rec := httptest.NewRecorder()

		handler.HandleEvents(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		require.Len(t, responder.calls, 1)
		assert.Equal(t, tenantID, responder.calls[0].TenantID)
		assert.Equal(t, "1234.5678", responder.calls[0].ThreadTS)
		assert.Equal(t, "use PostgreSQL", responder.calls[0].Answer)
		assert.Equal(t, "U123", responder.calls[0].SlackUserID)
	})

	t.Run("missing signature returns 401", func(t *testing.T) {
		t.Parallel()

		responder := &mockResponseHandler{}
		handler := airaslack.NewHandler(testSigningSecret, responder, tenantID)

		body := `{"type":"event_callback","event":{"type":"message","channel":"C1","thread_ts":"1.2","text":"hi","user":"U1"}}`
		req := httptest.NewRequest(http.MethodPost, "/slack/events", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// No signature headers.
		rec := httptest.NewRecorder()

		handler.HandleEvents(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Empty(t, responder.calls)
	})

	t.Run("invalid signature returns 401", func(t *testing.T) {
		t.Parallel()

		responder := &mockResponseHandler{}
		handler := airaslack.NewHandler(testSigningSecret, responder, tenantID)

		body := `{"type":"event_callback","event":{"type":"message","channel":"C1","thread_ts":"1.2","text":"hi","user":"U1"}}`
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		badSig := computeSlackSignature("wrong-secret", ts, body)

		req := httptest.NewRequest(http.MethodPost, "/slack/events", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Slack-Request-Timestamp", ts)
		req.Header.Set("X-Slack-Signature", badSig)
		rec := httptest.NewRecorder()

		handler.HandleEvents(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Empty(t, responder.calls)
	})

	t.Run("non-message event type ignored", func(t *testing.T) {
		t.Parallel()

		responder := &mockResponseHandler{}
		handler := airaslack.NewHandler(testSigningSecret, responder, tenantID)

		body := `{"type":"event_callback","event":{"type":"app_mention","channel":"C1","text":"hello","user":"U1"}}`
		req := signedJSONRequest(body)
		rec := httptest.NewRecorder()

		handler.HandleEvents(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, responder.calls)
	})

	t.Run("bot message ignored to prevent loops", func(t *testing.T) {
		t.Parallel()

		responder := &mockResponseHandler{}
		handler := airaslack.NewHandler(testSigningSecret, responder, tenantID)

		body := `{
			"type": "event_callback",
			"event": {
				"type": "message",
				"channel": "C123",
				"thread_ts": "1234.5678",
				"text": "bot response",
				"user": "U123",
				"bot_id": "B123"
			}
		}`
		req := signedJSONRequest(body)
		rec := httptest.NewRecorder()

		handler.HandleEvents(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, responder.calls)
	})

	t.Run("message without thread_ts ignored", func(t *testing.T) {
		t.Parallel()

		responder := &mockResponseHandler{}
		handler := airaslack.NewHandler(testSigningSecret, responder, tenantID)

		body := `{
			"type": "event_callback",
			"event": {
				"type": "message",
				"channel": "C123",
				"text": "top-level message",
				"user": "U123"
			}
		}`
		req := signedJSONRequest(body)
		rec := httptest.NewRecorder()

		handler.HandleEvents(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, responder.calls)
	})

	t.Run("responder error still returns 200", func(t *testing.T) {
		t.Parallel()

		responder := &mockResponseHandler{err: errors.New("responder failure")}
		handler := airaslack.NewHandler(testSigningSecret, responder, tenantID)

		body := `{"type":"event_callback","event":{"type":"message","channel":"C1","thread_ts":"1.2","text":"hi","user":"U1"}}`
		req := signedJSONRequest(body)
		rec := httptest.NewRecorder()

		handler.HandleEvents(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		require.Len(t, responder.calls, 1)
	})

	t.Run("malformed JSON returns 400", func(t *testing.T) {
		t.Parallel()

		responder := &mockResponseHandler{}
		handler := airaslack.NewHandler(testSigningSecret, responder, tenantID)

		body := `{not valid json`
		req := signedJSONRequest(body)
		rec := httptest.NewRecorder()

		handler.HandleEvents(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Empty(t, responder.calls)
	})
}

// --- HandleInteractions tests ---

func TestHandleInteractions(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()

	t.Run("button click dispatches to responder", func(t *testing.T) {
		t.Parallel()

		responder := &mockResponseHandler{}
		handler := airaslack.NewHandler(testSigningSecret, responder, tenantID)

		payload := map[string]any{
			"type": "block_actions",
			"user": map[string]any{"id": "U456"},
			"actions": []map[string]any{
				{
					"action_id": "hitl_answer",
					"value":     "option_a",
					"block_id":  "block_1",
					"type":      "button",
				},
			},
			"container": map[string]any{
				"type":       "message",
				"message_ts": "9999.0001",
				"thread_ts":  "9999.0000",
				"channel_id": "C789",
			},
			"channel": map[string]any{"id": "C789"},
			"message": map[string]any{
				"ts":        "9999.0001",
				"thread_ts": "9999.0000",
			},
		}
		payloadJSON, err := json.Marshal(payload)
		require.NoError(t, err)

		formBody := "payload=" + string(payloadJSON)
		req := signedFormRequest(formBody)
		rec := httptest.NewRecorder()

		handler.HandleInteractions(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		require.Len(t, responder.calls, 1)
		assert.Equal(t, tenantID, responder.calls[0].TenantID)
		assert.Equal(t, "9999.0000", responder.calls[0].ThreadTS)
		assert.Equal(t, "option_a", responder.calls[0].Answer)
		assert.Equal(t, "U456", responder.calls[0].SlackUserID)
	})

	t.Run("invalid signature returns 401", func(t *testing.T) {
		t.Parallel()

		responder := &mockResponseHandler{}
		handler := airaslack.NewHandler(testSigningSecret, responder, tenantID)

		formBody := `payload={"type":"block_actions","actions":[{"value":"x"}],"user":{"id":"U1"},"container":{"thread_ts":"1.0"}}`
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		badSig := computeSlackSignature("wrong-secret", ts, formBody)

		req := httptest.NewRequest(http.MethodPost, "/slack/interactions", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Slack-Request-Timestamp", ts)
		req.Header.Set("X-Slack-Signature", badSig)
		rec := httptest.NewRecorder()

		handler.HandleInteractions(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Empty(t, responder.calls)
	})

	t.Run("missing signature returns 401", func(t *testing.T) {
		t.Parallel()

		responder := &mockResponseHandler{}
		handler := airaslack.NewHandler(testSigningSecret, responder, tenantID)

		formBody := `payload={"type":"block_actions","actions":[{"value":"x"}],"user":{"id":"U1"},"container":{"thread_ts":"1.0"}}`
		req := httptest.NewRequest(http.MethodPost, "/slack/interactions", strings.NewReader(formBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.HandleInteractions(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Empty(t, responder.calls)
	})
}
