package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gosuda/aira/internal/domain"
)

type TenantRepo struct {
	pool *pgxpool.Pool
}

func NewTenantRepo(pool *pgxpool.Pool) *TenantRepo {
	return &TenantRepo{pool: pool}
}

func (r *TenantRepo) Create(ctx context.Context, t *domain.Tenant) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug, settings, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		t.ID, t.Name, t.Slug, t.Settings, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("tenantRepo.Create: %w", err)
	}

	return nil
}

func (r *TenantRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	var t domain.Tenant

	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, settings, created_at, updated_at
		 FROM tenants WHERE id = $1`,
		id,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.Settings, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("tenantRepo.GetByID: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("tenantRepo.GetByID: %w", err)
	}

	return &t, nil
}

func (r *TenantRepo) GetBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	var t domain.Tenant

	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, settings, created_at, updated_at
		 FROM tenants WHERE slug = $1`,
		slug,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.Settings, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("tenantRepo.GetBySlug: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("tenantRepo.GetBySlug: %w", err)
	}

	return &t, nil
}

func (r *TenantRepo) Update(ctx context.Context, t *domain.Tenant) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tenants SET name = $1, slug = $2, settings = $3, updated_at = now()
		 WHERE id = $4`,
		t.Name, t.Slug, t.Settings, t.ID,
	)
	if err != nil {
		return fmt.Errorf("tenantRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("tenantRepo.Update: %w", domain.ErrNotFound)
	}

	return nil
}

func (r *TenantRepo) List(ctx context.Context) ([]*domain.Tenant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, slug, settings, created_at, updated_at
		 FROM tenants ORDER BY created_at
		 LIMIT 500`,
	)
	if err != nil {
		return nil, fmt.Errorf("tenantRepo.List: %w", err)
	}
	defer rows.Close()

	var tenants []*domain.Tenant
	for rows.Next() {
		var t domain.Tenant

		err = rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Settings, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("tenantRepo.List: scan: %w", err)
		}

		tenants = append(tenants, &t)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("tenantRepo.List: rows: %w", err)
	}

	return tenants, nil
}
