package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gosuda/aira/internal/secrets"
)

// SecretRepo implements secrets.SecretRepository using PostgreSQL.
type SecretRepo struct {
	pool *pgxpool.Pool
}

// NewSecretRepo creates a new SecretRepo.
func NewSecretRepo(pool *pgxpool.Pool) *SecretRepo {
	return &SecretRepo{pool: pool}
}

// Create inserts an encrypted secret.
func (r *SecretRepo) Create(ctx context.Context, s *secrets.Secret) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO project_secrets (id, project_id, tenant_id, name, value, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		s.ID, s.ProjectID, s.TenantID, s.Name, s.Value, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("secretRepo.Create: %w", err)
	}

	return nil
}

// GetByName retrieves an encrypted secret by project and name.
func (r *SecretRepo) GetByName(ctx context.Context, tenantID, projectID uuid.UUID, name string) (*secrets.Secret, error) {
	var s secrets.Secret

	err := r.pool.QueryRow(ctx,
		`SELECT id, project_id, tenant_id, name, value, created_at, updated_at
		 FROM project_secrets WHERE tenant_id = $1 AND project_id = $2 AND name = $3`,
		tenantID, projectID, name,
	).Scan(&s.ID, &s.ProjectID, &s.TenantID, &s.Name, &s.Value, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("secretRepo.GetByName: %w", secrets.ErrSecretNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("secretRepo.GetByName: %w", err)
	}

	return &s, nil
}

// ListByProject returns all encrypted secrets for a project.
func (r *SecretRepo) ListByProject(ctx context.Context, tenantID, projectID uuid.UUID) ([]*secrets.Secret, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, project_id, tenant_id, name, value, created_at, updated_at
		 FROM project_secrets WHERE tenant_id = $1 AND project_id = $2
		 ORDER BY name`,
		tenantID, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("secretRepo.ListByProject: %w", err)
	}
	defer rows.Close()

	var list []*secrets.Secret
	for rows.Next() {
		var s secrets.Secret

		scanErr := rows.Scan(&s.ID, &s.ProjectID, &s.TenantID, &s.Name, &s.Value, &s.CreatedAt, &s.UpdatedAt)
		if scanErr != nil {
			return nil, fmt.Errorf("secretRepo.ListByProject: scan: %w", scanErr)
		}

		list = append(list, &s)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("secretRepo.ListByProject: rows: %w", rowsErr)
	}

	return list, nil
}

// Delete removes an encrypted secret by project and name.
func (r *SecretRepo) Delete(ctx context.Context, tenantID, projectID uuid.UUID, name string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM project_secrets WHERE tenant_id = $1 AND project_id = $2 AND name = $3`,
		tenantID, projectID, name,
	)
	if err != nil {
		return fmt.Errorf("secretRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("secretRepo.Delete: %w", secrets.ErrSecretNotFound)
	}

	return nil
}
