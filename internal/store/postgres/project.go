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

type ProjectRepo struct {
	pool *pgxpool.Pool
}

func NewProjectRepo(pool *pgxpool.Pool) *ProjectRepo {
	return &ProjectRepo{pool: pool}
}

func (r *ProjectRepo) Create(ctx context.Context, p *domain.Project) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO projects (id, tenant_id, name, repo_url, branch, settings, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		p.ID, p.TenantID, p.Name, p.RepoURL, p.Branch, p.Settings, p.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("projectRepo.Create: %w", err)
	}

	return nil
}

func (r *ProjectRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Project, error) {
	var p domain.Project

	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, repo_url, branch, settings, created_at
		 FROM projects WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	).Scan(&p.ID, &p.TenantID, &p.Name, &p.RepoURL, &p.Branch, &p.Settings, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("projectRepo.GetByID: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("projectRepo.GetByID: %w", err)
	}

	return &p, nil
}

func (r *ProjectRepo) Update(ctx context.Context, p *domain.Project) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE projects SET name = $1, repo_url = $2, branch = $3, settings = $4
		 WHERE tenant_id = $5 AND id = $6`,
		p.Name, p.RepoURL, p.Branch, p.Settings, p.TenantID, p.ID,
	)
	if err != nil {
		return fmt.Errorf("projectRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("projectRepo.Update: %w", domain.ErrNotFound)
	}

	return nil
}

func (r *ProjectRepo) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Project, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, name, repo_url, branch, settings, created_at
		 FROM projects WHERE tenant_id = $1 ORDER BY created_at`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("projectRepo.List: %w", err)
	}
	defer rows.Close()

	var projects []*domain.Project
	for rows.Next() {
		var p domain.Project

		err = rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.RepoURL, &p.Branch, &p.Settings, &p.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("projectRepo.List: scan: %w", err)
		}
		projects = append(projects, &p)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("projectRepo.List: rows: %w", err)
	}

	return projects, nil
}

func (r *ProjectRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM projects WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("projectRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("projectRepo.Delete: %w", domain.ErrNotFound)
	}

	return nil
}
