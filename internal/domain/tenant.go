package domain

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	Settings  json.RawMessage
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TenantRepository interface {
	Create(ctx context.Context, t *Tenant) error
	GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error)
	GetBySlug(ctx context.Context, slug string) (*Tenant, error)
	Update(ctx context.Context, t *Tenant) error
	List(ctx context.Context) ([]*Tenant, error)
	ListPaginated(ctx context.Context, limit, offset int) ([]*Tenant, error)
}
