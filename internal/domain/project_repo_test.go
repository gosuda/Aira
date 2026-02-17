package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/gosuda/aira/internal/domain"
)

func TestProjectRepo_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	id := uuid.New()
	projectID := uuid.New()
	tenantID := uuid.New()

	pr := domain.ProjectRepo{
		ID:        id,
		ProjectID: projectID,
		TenantID:  tenantID,
		Name:      "frontend",
		RepoURL:   "https://github.com/org/frontend.git",
		Branch:    "main",
		MountPath: "packages/frontend",
		CreatedAt: now,
	}

	assert.Equal(t, id, pr.ID)
	assert.Equal(t, projectID, pr.ProjectID)
	assert.Equal(t, tenantID, pr.TenantID)
	assert.Equal(t, "frontend", pr.Name)
	assert.Equal(t, "https://github.com/org/frontend.git", pr.RepoURL)
	assert.Equal(t, "main", pr.Branch)
	assert.Equal(t, "packages/frontend", pr.MountPath)
	assert.Equal(t, now, pr.CreatedAt)
}

func TestProjectRepo_ZeroValue(t *testing.T) {
	t.Parallel()

	var pr domain.ProjectRepo

	assert.Equal(t, uuid.Nil, pr.ID)
	assert.Equal(t, uuid.Nil, pr.ProjectID)
	assert.Equal(t, uuid.Nil, pr.TenantID)
	assert.Empty(t, pr.Name)
	assert.Empty(t, pr.RepoURL)
	assert.Empty(t, pr.Branch)
	assert.Empty(t, pr.MountPath)
	assert.True(t, pr.CreatedAt.IsZero())
}

// Compile-time interface satisfaction check.
var _ domain.ProjectRepoRepository = (*projectRepoRepoStub)(nil)

type projectRepoRepoStub struct{}

func (s *projectRepoRepoStub) Create(_ context.Context, _ *domain.ProjectRepo) error { return nil }
func (s *projectRepoRepoStub) GetByID(_ context.Context, _, _ uuid.UUID) (*domain.ProjectRepo, error) {
	return nil, nil
}
func (s *projectRepoRepoStub) ListByProject(_ context.Context, _, _ uuid.UUID) ([]*domain.ProjectRepo, error) {
	return nil, nil
}
func (s *projectRepoRepoStub) Delete(_ context.Context, _, _ uuid.UUID) error { return nil }
