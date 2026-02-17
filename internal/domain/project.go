package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Name      string
	RepoURL   string
	Branch    string // default "main"
	Settings  map[string]any
	CreatedAt time.Time
}

type ProjectRepository interface {
	Create(ctx context.Context, p *Project) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*Project, error)
	Update(ctx context.Context, p *Project) error
	List(ctx context.Context, tenantID uuid.UUID) ([]*Project, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}
