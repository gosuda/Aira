package redis_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	redisstore "github.com/gosuda/aira/internal/store/redis"
)

func TestBoardChannel(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	projectID := uuid.MustParse("11111111-2222-3333-4444-555555555555")

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		got := redisstore.BoardChannel(tenantID, projectID)
		assert.Equal(t, "board:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee:11111111-2222-3333-4444-555555555555", got)
	})

	t.Run("nil UUIDs", func(t *testing.T) {
		t.Parallel()

		got := redisstore.BoardChannel(uuid.Nil, uuid.Nil)
		assert.Equal(t, "board:00000000-0000-0000-0000-000000000000:00000000-0000-0000-0000-000000000000", got)
	})

	t.Run("prefix", func(t *testing.T) {
		t.Parallel()

		got := redisstore.BoardChannel(tenantID, projectID)
		assert.True(t, strings.HasPrefix(got, "board:"), "expected prefix 'board:', got %q", got)
	})

	t.Run("contains both UUIDs", func(t *testing.T) {
		t.Parallel()

		got := redisstore.BoardChannel(tenantID, projectID)
		assert.Contains(t, got, tenantID.String())
		assert.Contains(t, got, projectID.String())
	})

	t.Run("deterministic", func(t *testing.T) {
		t.Parallel()

		a := redisstore.BoardChannel(tenantID, projectID)
		b := redisstore.BoardChannel(tenantID, projectID)
		assert.Equal(t, a, b)
	})

	t.Run("different inputs produce different outputs", func(t *testing.T) {
		t.Parallel()

		otherProject := uuid.MustParse("99999999-8888-7777-6666-555544443333")
		a := redisstore.BoardChannel(tenantID, projectID)
		b := redisstore.BoardChannel(tenantID, otherProject)
		assert.NotEqual(t, a, b)
	})
}

func TestAgentChannel(t *testing.T) {
	t.Parallel()

	sessionID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		got := redisstore.AgentChannel(sessionID)
		assert.Equal(t, "agent:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", got)
	})

	t.Run("nil UUID", func(t *testing.T) {
		t.Parallel()

		got := redisstore.AgentChannel(uuid.Nil)
		assert.Equal(t, "agent:00000000-0000-0000-0000-000000000000", got)
	})

	t.Run("prefix", func(t *testing.T) {
		t.Parallel()

		got := redisstore.AgentChannel(sessionID)
		assert.True(t, strings.HasPrefix(got, "agent:"), "expected prefix 'agent:', got %q", got)
	})

	t.Run("contains UUID", func(t *testing.T) {
		t.Parallel()

		got := redisstore.AgentChannel(sessionID)
		assert.Contains(t, got, sessionID.String())
	})

	t.Run("deterministic", func(t *testing.T) {
		t.Parallel()

		a := redisstore.AgentChannel(sessionID)
		b := redisstore.AgentChannel(sessionID)
		assert.Equal(t, a, b)
	})

	t.Run("different inputs produce different outputs", func(t *testing.T) {
		t.Parallel()

		other := uuid.MustParse("11111111-2222-3333-4444-555555555555")
		a := redisstore.AgentChannel(sessionID)
		b := redisstore.AgentChannel(other)
		assert.NotEqual(t, a, b)
	})
}

func TestTenantChannel(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		got := redisstore.TenantChannel(tenantID)
		assert.Equal(t, "tenant:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", got)
	})

	t.Run("nil UUID", func(t *testing.T) {
		t.Parallel()

		got := redisstore.TenantChannel(uuid.Nil)
		assert.Equal(t, "tenant:00000000-0000-0000-0000-000000000000", got)
	})

	t.Run("prefix", func(t *testing.T) {
		t.Parallel()

		got := redisstore.TenantChannel(tenantID)
		assert.True(t, strings.HasPrefix(got, "tenant:"), "expected prefix 'tenant:', got %q", got)
	})

	t.Run("contains UUID", func(t *testing.T) {
		t.Parallel()

		got := redisstore.TenantChannel(tenantID)
		assert.Contains(t, got, tenantID.String())
	})

	t.Run("deterministic", func(t *testing.T) {
		t.Parallel()

		a := redisstore.TenantChannel(tenantID)
		b := redisstore.TenantChannel(tenantID)
		assert.Equal(t, a, b)
	})

	t.Run("different inputs produce different outputs", func(t *testing.T) {
		t.Parallel()

		other := uuid.MustParse("11111111-2222-3333-4444-555555555555")
		a := redisstore.TenantChannel(tenantID)
		b := redisstore.TenantChannel(other)
		assert.NotEqual(t, a, b)
	})
}

func TestChannelFunctions_NoCollisionAcrossTypes(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")

	board := redisstore.BoardChannel(id, id)
	agent := redisstore.AgentChannel(id)
	tenant := redisstore.TenantChannel(id)

	assert.NotEqual(t, board, agent, "board and agent channels must not collide")
	assert.NotEqual(t, board, tenant, "board and tenant channels must not collide")
	assert.NotEqual(t, agent, tenant, "agent and tenant channels must not collide")
}
