package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/agent"
	"github.com/gosuda/aira/internal/domain"
)

// --- mock ADR repository for postprocess tests ---

type mockPostprocessADRRepo struct {
	existing  []*domain.ADR
	listErr   error
	createErr error
	created   []*domain.ADR
}

func (m *mockPostprocessADRRepo) Create(_ context.Context, adr *domain.ADR) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, adr)
	return nil
}

func (m *mockPostprocessADRRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.ADR, error) {
	return nil, nil
}

func (m *mockPostprocessADRRepo) ListByProject(_ context.Context, _, _ uuid.UUID) ([]*domain.ADR, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.existing, nil
}

func (m *mockPostprocessADRRepo) ListByProjectPaginated(context.Context, uuid.UUID, uuid.UUID, int, int) ([]*domain.ADR, error) {
	return nil, nil
}

func (m *mockPostprocessADRRepo) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, domain.ADRStatus) error {
	return nil
}

// --- mock session repository (unused by PostProcessor but required for NewPostProcessor) ---

type mockAgentSessionRepo struct{}

func (m *mockAgentSessionRepo) Create(context.Context, *domain.AgentSession) error { return nil }
func (m *mockAgentSessionRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.AgentSession, error) {
	return nil, nil
}
func (m *mockAgentSessionRepo) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, domain.AgentSessionStatus) error {
	return nil
}
func (m *mockAgentSessionRepo) UpdateContainer(context.Context, uuid.UUID, uuid.UUID, string, string) error {
	return nil
}
func (m *mockAgentSessionRepo) SetCompleted(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (m *mockAgentSessionRepo) ListByTask(context.Context, uuid.UUID, uuid.UUID) ([]*domain.AgentSession, error) {
	return nil, nil
}
func (m *mockAgentSessionRepo) ListByProject(context.Context, uuid.UUID, uuid.UUID) ([]*domain.AgentSession, error) {
	return nil, nil
}

func (m *mockAgentSessionRepo) ListByProjectPaginated(context.Context, uuid.UUID, uuid.UUID, int, int) ([]*domain.AgentSession, error) {
	return nil, nil
}

func (m *mockAgentSessionRepo) CountByProject(context.Context, uuid.UUID, uuid.UUID) (int64, error) {
	return 0, nil
}

// --- extractDecisions pattern matching tests (tested via ExtractImplicitADRs) ---

func TestExtractImplicitADRs_PatternMatching(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	projectID := uuid.New()
	sessionID := uuid.New()

	tests := []struct {
		name         string
		conversation []string
		gitDiff      string
		wantCreated  int
		wantTitleSub string // substring expected in first created ADR title
	}{
		{
			name:         "chose X over Y",
			conversation: []string{"We chose PostgreSQL over MySQL for better JSON support."},
			wantCreated:  1,
			wantTitleSub: "PostgreSQL",
		},
		{
			name:         "decided to use X",
			conversation: []string{"We decided to use Redis for our caching layer."},
			wantCreated:  1,
			wantTitleSub: "Redis",
		},
		{
			name:         "decided to go with X",
			conversation: []string{"We decided to go with gRPC for internal services."},
			wantCreated:  1,
			wantTitleSub: "gRPC",
		},
		{
			name:         "switched from X to Y",
			conversation: []string{"We switched from REST to GraphQL for the API."},
			wantCreated:  1,
			wantTitleSub: "GraphQL",
		},
		{
			name:         "selected X instead of Y",
			conversation: []string{"We selected Kubernetes instead of Docker Swarm."},
			wantCreated:  1,
			wantTitleSub: "Kubernetes",
		},
		{
			name:         "replaced X with Y",
			conversation: []string{"We replaced Jenkins with GitHub Actions."},
			wantCreated:  1,
			wantTitleSub: "GitHub Actions",
		},
		{
			name:         "no matches returns zero",
			conversation: []string{"The weather is nice today.", "Let's discuss the architecture."},
			wantCreated:  0,
		},
		{
			name:         "empty conversation",
			conversation: nil,
			wantCreated:  0,
		},
		{
			name: "dedup same decision mentioned twice",
			conversation: []string{
				"We chose PostgreSQL over MySQL.",
				"Again, we chose PostgreSQL over SQLite.",
			},
			wantCreated: 1,
		},
		{
			name: "multiple distinct decisions",
			conversation: []string{
				"We chose PostgreSQL over MySQL.",
				"We decided to use Redis for caching.",
			},
			wantCreated: 2,
		},
		{
			name:         "decision in git diff",
			conversation: nil,
			gitDiff:      "+// We decided to use Go for the backend.\n+++ b/main.go",
			wantCreated:  1,
			wantTitleSub: "Go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()

			adrRepo := &mockPostprocessADRRepo{}
			sessRepo := &mockAgentSessionRepo{}
			pp := agent.NewPostProcessor(adrRepo, sessRepo)

			created, err := pp.ExtractImplicitADRs(ctx, sessionID, tenantID, projectID, tt.conversation, tt.gitDiff)

			require.NoError(t, err)
			assert.Equal(t, tt.wantCreated, created)

			if tt.wantTitleSub != "" && len(adrRepo.created) > 0 {
				assert.Contains(t, adrRepo.created[0].Title, tt.wantTitleSub)
			}
		})
	}
}

func TestExtractImplicitADRs_DeduplicatesAgainstExisting(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	tenantID := uuid.New()
	projectID := uuid.New()
	sessionID := uuid.New()

	adrRepo := &mockPostprocessADRRepo{
		existing: []*domain.ADR{
			{Title: "Use PostgreSQL instead of MySQL"},
		},
	}
	sessRepo := &mockAgentSessionRepo{}
	pp := agent.NewPostProcessor(adrRepo, sessRepo)

	// This decision matches the existing ADR title (case-insensitive).
	conversation := []string{"We chose PostgreSQL over MySQL."}

	created, err := pp.ExtractImplicitADRs(ctx, sessionID, tenantID, projectID, conversation, "")

	require.NoError(t, err)
	assert.Equal(t, 0, created)
	assert.Empty(t, adrRepo.created)
}

func TestExtractImplicitADRs_CreateError(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	tenantID := uuid.New()
	projectID := uuid.New()
	sessionID := uuid.New()

	adrRepo := &mockPostprocessADRRepo{
		createErr: errors.New("write failure"),
	}
	sessRepo := &mockAgentSessionRepo{}
	pp := agent.NewPostProcessor(adrRepo, sessRepo)

	conversation := []string{"We decided to use Kafka for messaging."}

	created, err := pp.ExtractImplicitADRs(ctx, sessionID, tenantID, projectID, conversation, "")

	require.Error(t, err)
	assert.Equal(t, 0, created)
	assert.Contains(t, err.Error(), "create ADR")
}

func TestExtractImplicitADRs_ListByProjectError(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	tenantID := uuid.New()
	projectID := uuid.New()
	sessionID := uuid.New()

	adrRepo := &mockPostprocessADRRepo{
		listErr: errors.New("list failure"),
	}
	sessRepo := &mockAgentSessionRepo{}
	pp := agent.NewPostProcessor(adrRepo, sessRepo)

	conversation := []string{"We decided to use Kafka for messaging."}

	created, err := pp.ExtractImplicitADRs(ctx, sessionID, tenantID, projectID, conversation, "")

	require.Error(t, err)
	assert.Equal(t, 0, created)
	assert.Contains(t, err.Error(), "list existing")
}

func TestExtractImplicitADRs_ADRFieldsPopulatedCorrectly(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	tenantID := uuid.New()
	projectID := uuid.New()
	sessionID := uuid.New()

	adrRepo := &mockPostprocessADRRepo{}
	sessRepo := &mockAgentSessionRepo{}
	pp := agent.NewPostProcessor(adrRepo, sessRepo)

	conversation := []string{"We chose Go over Rust for simplicity."}

	created, err := pp.ExtractImplicitADRs(ctx, sessionID, tenantID, projectID, conversation, "")

	require.NoError(t, err)
	assert.Equal(t, 1, created)
	require.Len(t, adrRepo.created, 1)

	adr := adrRepo.created[0]
	assert.Equal(t, tenantID, adr.TenantID)
	assert.Equal(t, projectID, adr.ProjectID)
	assert.Equal(t, 0, adr.Sequence) // sequence allocated atomically by the store
	assert.Equal(t, domain.ADRStatusDraft, adr.Status)
	assert.NotEqual(t, uuid.Nil, adr.ID)
	assert.Equal(t, &sessionID, adr.AgentSessionID)
	assert.Nil(t, adr.CreatedBy)
	assert.Contains(t, adr.Decision, "Go")
	assert.Contains(t, adr.Options, "Go")
	// The "rejected" capture from "chose Go over Rust for simplicity" includes trailing text.
	require.Len(t, adr.Options, 2)
	assert.Contains(t, adr.Options[1], "Rust")
}
