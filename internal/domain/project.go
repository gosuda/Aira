package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Name      string
	RepoURL   string
	Branch    string // default "main"
	Settings  json.RawMessage
	CreatedAt time.Time
}

// NewProject creates a Project with validated required fields and defaults.
func NewProject(tenantID uuid.UUID, name, repoURL, branch string, settings json.RawMessage) (*Project, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("project: tenant ID is required")
	}
	if name == "" {
		return nil, errors.New("project: name is required")
	}
	if repoURL == "" {
		return nil, errors.New("project: repo URL is required")
	}
	if branch == "" {
		branch = "main"
	}
	if settings == nil {
		settings = json.RawMessage("{}")
	}
	return &Project{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Name:      name,
		RepoURL:   repoURL,
		Branch:    branch,
		Settings:  settings,
		CreatedAt: time.Now(),
	}, nil
}

type ProjectRepository interface {
	Create(ctx context.Context, p *Project) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*Project, error)
	Update(ctx context.Context, p *Project) error
	List(ctx context.Context, tenantID uuid.UUID) ([]*Project, error)
	ListPaginated(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*Project, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}
