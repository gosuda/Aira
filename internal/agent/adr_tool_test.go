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

// --- mock repositories ---

type mockADRRepository struct {
	createErr error
	created   []*domain.ADR
}

func (m *mockADRRepository) Create(_ context.Context, adr *domain.ADR) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, adr)
	return nil
}

func (m *mockADRRepository) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.ADR, error) {
	return nil, nil
}

func (m *mockADRRepository) ListByProject(context.Context, uuid.UUID, uuid.UUID) ([]*domain.ADR, error) {
	return nil, nil
}

func (m *mockADRRepository) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, domain.ADRStatus) error {
	return nil
}

type mockProjectRepository struct {
	project  *domain.Project
	getByErr error
}

func (m *mockProjectRepository) Create(context.Context, *domain.Project) error { return nil }
func (m *mockProjectRepository) GetByID(_ context.Context, _, _ uuid.UUID) (*domain.Project, error) {
	if m.getByErr != nil {
		return nil, m.getByErr
	}
	return m.project, nil
}
func (m *mockProjectRepository) Update(context.Context, *domain.Project) error { return nil }
func (m *mockProjectRepository) List(context.Context, uuid.UUID) ([]*domain.Project, error) {
	return nil, nil
}
func (m *mockProjectRepository) Delete(context.Context, uuid.UUID, uuid.UUID) error { return nil }

// --- ValidateADRToolInput tests ---

func TestValidateADRToolInput(t *testing.T) {
	t.Parallel()

	validInput := func() *agent.ADRToolInput {
		return &agent.ADRToolInput{
			Title:     "Use PostgreSQL",
			Context:   "Need a relational database",
			Decision:  "PostgreSQL for ACID compliance",
			ProjectID: uuid.New(),
			TenantID:  uuid.New(),
			SessionID: uuid.New(),
		}
	}

	tests := []struct {
		name    string
		modify  func(*agent.ADRToolInput)
		wantErr bool
		errMsg  string
	}{
		{
			name:    "all valid",
			modify:  func(_ *agent.ADRToolInput) {},
			wantErr: false,
		},
		{
			name:    "missing title",
			modify:  func(in *agent.ADRToolInput) { in.Title = "" },
			wantErr: true,
			errMsg:  "title is required",
		},
		{
			name:    "missing context",
			modify:  func(in *agent.ADRToolInput) { in.Context = "" },
			wantErr: true,
			errMsg:  "context is required",
		},
		{
			name:    "missing decision",
			modify:  func(in *agent.ADRToolInput) { in.Decision = "" },
			wantErr: true,
			errMsg:  "decision is required",
		},
		{
			name:    "missing project_id",
			modify:  func(in *agent.ADRToolInput) { in.ProjectID = uuid.Nil },
			wantErr: true,
			errMsg:  "project_id is required",
		},
		{
			name:    "missing tenant_id",
			modify:  func(in *agent.ADRToolInput) { in.TenantID = uuid.Nil },
			wantErr: true,
			errMsg:  "tenant_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input := validInput()
			tt.modify(input)

			err := agent.ValidateADRToolInput(input)

			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, agent.ErrInvalidADRInput)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- ProcessADRToolCall tests ---

func TestProcessADRToolCall(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	projectID := uuid.New()
	sessionID := uuid.New()

	newInput := func() *agent.ADRToolInput {
		return &agent.ADRToolInput{
			Title:     "Use Redis for caching",
			Context:   "Need in-memory caching",
			Decision:  "Redis chosen for performance",
			Drivers:   []string{"performance", "simplicity"},
			Options:   []string{"Redis", "Memcached"},
			ProjectID: projectID,
			TenantID:  tenantID,
			SessionID: sessionID,
			Consequences: agent.ADRConsequences{
				Good:    []string{"fast"},
				Bad:     []string{"extra infra"},
				Neutral: []string{"common choice"},
			},
		}
	}

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		adrRepo := &mockADRRepository{}
		projRepo := &mockProjectRepository{
			project: &domain.Project{ID: projectID, TenantID: tenantID, Name: "test"},
		}

		deps := agent.ADRToolDeps{ADRs: adrRepo, Projects: projRepo}
		input := newInput()

		adr, err := agent.ProcessADRToolCall(ctx, deps, input)

		require.NoError(t, err)
		require.NotNil(t, adr)
		assert.Equal(t, tenantID, adr.TenantID)
		assert.Equal(t, projectID, adr.ProjectID)
		assert.Equal(t, 0, adr.Sequence) // sequence allocated atomically by the store
		assert.Equal(t, "Use Redis for caching", adr.Title)
		assert.Equal(t, domain.ADRStatusProposed, adr.Status)
		assert.Equal(t, input.Context, adr.Context)
		assert.Equal(t, input.Decision, adr.Decision)
		assert.NotEqual(t, uuid.Nil, adr.ID)
		assert.Equal(t, &sessionID, adr.AgentSessionID)
		assert.Nil(t, adr.CreatedBy)
		require.Len(t, adrRepo.created, 1)
	})

	t.Run("project not found", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		adrRepo := &mockADRRepository{}
		projRepo := &mockProjectRepository{getByErr: errors.New("not found")}

		deps := agent.ADRToolDeps{ADRs: adrRepo, Projects: projRepo}
		input := newInput()

		adr, err := agent.ProcessADRToolCall(ctx, deps, input)

		require.Error(t, err)
		assert.Nil(t, adr)
		assert.Contains(t, err.Error(), "get project")
	})

	t.Run("Create error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		adrRepo := &mockADRRepository{createErr: errors.New("unique constraint")}
		projRepo := &mockProjectRepository{
			project: &domain.Project{ID: projectID, TenantID: tenantID},
		}

		deps := agent.ADRToolDeps{ADRs: adrRepo, Projects: projRepo}
		input := newInput()

		adr, err := agent.ProcessADRToolCall(ctx, deps, input)

		require.Error(t, err)
		assert.Nil(t, adr)
		assert.Contains(t, err.Error(), "create ADR")
	})

	t.Run("validation error propagates", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		adrRepo := &mockADRRepository{}
		projRepo := &mockProjectRepository{
			project: &domain.Project{ID: projectID, TenantID: tenantID},
		}

		deps := agent.ADRToolDeps{ADRs: adrRepo, Projects: projRepo}
		input := newInput()
		input.Title = "" // invalid

		adr, err := agent.ProcessADRToolCall(ctx, deps, input)

		require.Error(t, err)
		assert.Nil(t, adr)
		assert.ErrorIs(t, err, agent.ErrInvalidADRInput)
	})
}
