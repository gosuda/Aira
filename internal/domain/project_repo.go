package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ProjectRepo links an additional repository to a project.
// The primary repo remains on Project.RepoURL; these are additional repos
// for monorepo or multi-repo setups.
type ProjectRepo struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	TenantID  uuid.UUID
	Name      string // human-friendly name, e.g. "frontend", "shared-lib"
	RepoURL   string // git clone URL
	Branch    string // default branch for this repo
	MountPath string // relative path inside the workspace, e.g. "packages/frontend"
	CreatedAt time.Time
}

// ProjectRepoRepository manages additional repository links per project.
type ProjectRepoRepository interface {
	Create(ctx context.Context, r *ProjectRepo) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*ProjectRepo, error)
	ListByProject(ctx context.Context, tenantID, projectID uuid.UUID) ([]*ProjectRepo, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}
