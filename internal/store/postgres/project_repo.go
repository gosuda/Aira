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

type ProjectRepoRepo struct {
	pool *pgxpool.Pool
}

func NewProjectRepoRepo(pool *pgxpool.Pool) *ProjectRepoRepo {
	return &ProjectRepoRepo{pool: pool}
}

func (r *ProjectRepoRepo) Create(ctx context.Context, pr *domain.ProjectRepo) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO project_repos (id, project_id, tenant_id, name, repo_url, branch, mount_path, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		pr.ID, pr.ProjectID, pr.TenantID, pr.Name, pr.RepoURL, pr.Branch, pr.MountPath, pr.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("projectRepoRepo.Create: %w", err)
	}

	return nil
}

func (r *ProjectRepoRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.ProjectRepo, error) {
	var pr domain.ProjectRepo

	err := r.pool.QueryRow(ctx,
		`SELECT id, project_id, tenant_id, name, repo_url, branch, mount_path, created_at
		 FROM project_repos WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	).Scan(&pr.ID, &pr.ProjectID, &pr.TenantID, &pr.Name, &pr.RepoURL, &pr.Branch, &pr.MountPath, &pr.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("projectRepoRepo.GetByID: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("projectRepoRepo.GetByID: %w", err)
	}

	return &pr, nil
}

func (r *ProjectRepoRepo) ListByProject(ctx context.Context, tenantID, projectID uuid.UUID) ([]*domain.ProjectRepo, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, project_id, tenant_id, name, repo_url, branch, mount_path, created_at
		 FROM project_repos WHERE tenant_id = $1 AND project_id = $2
		 ORDER BY created_at`,
		tenantID, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("projectRepoRepo.ListByProject: %w", err)
	}
	defer rows.Close()

	var repos []*domain.ProjectRepo
	for rows.Next() {
		var pr domain.ProjectRepo

		err = rows.Scan(&pr.ID, &pr.ProjectID, &pr.TenantID, &pr.Name, &pr.RepoURL, &pr.Branch, &pr.MountPath, &pr.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("projectRepoRepo.ListByProject: scan: %w", err)
		}
		repos = append(repos, &pr)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("projectRepoRepo.ListByProject: rows: %w", err)
	}

	return repos, nil
}

func (r *ProjectRepoRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM project_repos WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("projectRepoRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("projectRepoRepo.Delete: %w", domain.ErrNotFound)
	}

	return nil
}
