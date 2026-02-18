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

type TaskRepo struct {
	pool *pgxpool.Pool
}

func NewTaskRepo(pool *pgxpool.Pool) *TaskRepo {
	return &TaskRepo{pool: pool}
}

func (r *TaskRepo) Create(ctx context.Context, t *domain.Task) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tasks (id, tenant_id, project_id, adr_id, title, description, status, priority, assigned_to, agent_session_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		t.ID, t.TenantID, t.ProjectID, t.ADRID, t.Title, t.Description,
		t.Status, t.Priority, t.AssignedTo, t.AgentSessionID,
		t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("taskRepo.Create: %w", err)
	}

	return nil
}

func (r *TaskRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Task, error) {
	var t domain.Task

	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, project_id, adr_id, title, description, status, priority,
		        assigned_to, agent_session_id, created_at, updated_at
		 FROM tasks WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	).Scan(
		&t.ID, &t.TenantID, &t.ProjectID, &t.ADRID, &t.Title, &t.Description,
		&t.Status, &t.Priority, &t.AssignedTo, &t.AgentSessionID,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("taskRepo.GetByID: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("taskRepo.GetByID: %w", err)
	}

	return &t, nil
}

func (r *TaskRepo) ListByProject(ctx context.Context, tenantID, projectID uuid.UUID) ([]*domain.Task, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, project_id, adr_id, title, description, status, priority,
		        assigned_to, agent_session_id, created_at, updated_at
		 FROM tasks WHERE tenant_id = $1 AND project_id = $2
		 ORDER BY priority, created_at
		 LIMIT 1000`,
		tenantID, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("taskRepo.ListByProject: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows, "taskRepo.ListByProject")
}

func (r *TaskRepo) ListByStatus(ctx context.Context, tenantID, projectID uuid.UUID, status domain.TaskStatus) ([]*domain.Task, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, project_id, adr_id, title, description, status, priority,
		        assigned_to, agent_session_id, created_at, updated_at
		 FROM tasks WHERE tenant_id = $1 AND project_id = $2 AND status = $3
		 ORDER BY priority, created_at
		 LIMIT 1000`,
		tenantID, projectID, status,
	)
	if err != nil {
		return nil, fmt.Errorf("taskRepo.ListByStatus: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows, "taskRepo.ListByStatus")
}

func (r *TaskRepo) UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status domain.TaskStatus) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tasks SET status = $1, updated_at = now() WHERE tenant_id = $2 AND id = $3`,
		status, tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("taskRepo.UpdateStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("taskRepo.UpdateStatus: %w", domain.ErrNotFound)
	}

	return nil
}

func (r *TaskRepo) Update(ctx context.Context, t *domain.Task) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tasks SET project_id = $1, adr_id = $2, title = $3, description = $4,
		        status = $5, priority = $6, assigned_to = $7, agent_session_id = $8, updated_at = now()
		 WHERE tenant_id = $9 AND id = $10`,
		t.ProjectID, t.ADRID, t.Title, t.Description,
		t.Status, t.Priority, t.AssignedTo, t.AgentSessionID,
		t.TenantID, t.ID,
	)
	if err != nil {
		return fmt.Errorf("taskRepo.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("taskRepo.Update: %w", domain.ErrNotFound)
	}

	return nil
}

func (r *TaskRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM tasks WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("taskRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("taskRepo.Delete: %w", domain.ErrNotFound)
	}

	return nil
}

func scanTasks(rows pgx.Rows, caller string) ([]*domain.Task, error) {
	var tasks []*domain.Task
	for rows.Next() {
		var t domain.Task
		if err := rows.Scan(
			&t.ID, &t.TenantID, &t.ProjectID, &t.ADRID, &t.Title, &t.Description,
			&t.Status, &t.Priority, &t.AssignedTo, &t.AgentSessionID,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", caller, err)
		}
		tasks = append(tasks, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", caller, err)
	}

	return tasks, nil
}
