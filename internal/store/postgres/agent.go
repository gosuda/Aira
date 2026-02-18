package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gosuda/aira/internal/domain"
)

type AgentSessionRepo struct {
	pool *pgxpool.Pool
}

func NewAgentSessionRepo(pool *pgxpool.Pool) *AgentSessionRepo {
	return &AgentSessionRepo{pool: pool}
}

func (r *AgentSessionRepo) Create(ctx context.Context, s *domain.AgentSession) error {
	metadata, err := json.Marshal(s.Metadata)
	if err != nil {
		return fmt.Errorf("agentSessionRepo.Create: marshal metadata: %w", err)
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO agent_sessions (id, tenant_id, project_id, task_id, agent_type, status, container_id, branch_name, started_at, completed_at, error, metadata, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		s.ID, s.TenantID, s.ProjectID, s.TaskID, s.AgentType, s.Status,
		s.ContainerID, s.BranchName, s.StartedAt, s.CompletedAt,
		s.Error, metadata, s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("agentSessionRepo.Create: %w", err)
	}

	return nil
}

func (r *AgentSessionRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.AgentSession, error) {
	var s domain.AgentSession
	var metadata []byte

	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, project_id, task_id, agent_type, status, container_id, branch_name,
		        started_at, completed_at, error, metadata, created_at
		 FROM agent_sessions WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	).Scan(
		&s.ID, &s.TenantID, &s.ProjectID, &s.TaskID, &s.AgentType, &s.Status,
		&s.ContainerID, &s.BranchName, &s.StartedAt, &s.CompletedAt,
		&s.Error, &metadata, &s.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("agentSessionRepo.GetByID: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("agentSessionRepo.GetByID: %w", err)
	}

	err = json.Unmarshal(metadata, &s.Metadata)
	if err != nil {
		return nil, fmt.Errorf("agentSessionRepo.GetByID: unmarshal metadata: %w", err)
	}

	return &s, nil
}

func (r *AgentSessionRepo) UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status domain.AgentSessionStatus) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE agent_sessions SET status = $1 WHERE tenant_id = $2 AND id = $3`,
		status, tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("agentSessionRepo.UpdateStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agentSessionRepo.UpdateStatus: %w", domain.ErrNotFound)
	}

	return nil
}

func (r *AgentSessionRepo) UpdateContainer(ctx context.Context, tenantID, id uuid.UUID, containerID, branchName string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE agent_sessions SET container_id = $1, branch_name = $2 WHERE tenant_id = $3 AND id = $4`,
		containerID, branchName, tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("agentSessionRepo.UpdateContainer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agentSessionRepo.UpdateContainer: %w", domain.ErrNotFound)
	}

	return nil
}

func (r *AgentSessionRepo) SetCompleted(ctx context.Context, tenantID, id uuid.UUID, errMsg string) error {
	status := domain.AgentStatusCompleted
	if errMsg != "" {
		status = domain.AgentStatusFailed
	}

	tag, err := r.pool.Exec(ctx,
		`UPDATE agent_sessions SET status = $1, completed_at = now(), error = $2 WHERE tenant_id = $3 AND id = $4`,
		status, errMsg, tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("agentSessionRepo.SetCompleted: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agentSessionRepo.SetCompleted: %w", domain.ErrNotFound)
	}

	return nil
}

func (r *AgentSessionRepo) ListByTask(ctx context.Context, tenantID, taskID uuid.UUID) ([]*domain.AgentSession, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, project_id, task_id, agent_type, status, container_id, branch_name,
		        started_at, completed_at, error, metadata, created_at
		 FROM agent_sessions WHERE tenant_id = $1 AND task_id = $2
		 ORDER BY created_at DESC`,
		tenantID, taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("agentSessionRepo.ListByTask: %w", err)
	}
	defer rows.Close()

	return scanAgentSessions(rows, "agentSessionRepo.ListByTask")
}

func (r *AgentSessionRepo) ListByProject(ctx context.Context, tenantID, projectID uuid.UUID) ([]*domain.AgentSession, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, project_id, task_id, agent_type, status, container_id, branch_name,
		        started_at, completed_at, error, metadata, created_at
		 FROM agent_sessions WHERE tenant_id = $1 AND project_id = $2
		 ORDER BY created_at DESC`,
		tenantID, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("agentSessionRepo.ListByProject: %w", err)
	}
	defer rows.Close()

	return scanAgentSessions(rows, "agentSessionRepo.ListByProject")
}

func (r *AgentSessionRepo) ListByProjectPaginated(ctx context.Context, tenantID, projectID uuid.UUID, limit, offset int) ([]*domain.AgentSession, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, project_id, task_id, agent_type, status, container_id, branch_name,
		        started_at, completed_at, error, metadata, created_at
		 FROM agent_sessions WHERE tenant_id = $1 AND project_id = $2
		 ORDER BY created_at DESC
		 LIMIT $3 OFFSET $4`,
		tenantID, projectID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("agentSessionRepo.ListByProjectPaginated: %w", err)
	}
	defer rows.Close()

	return scanAgentSessions(rows, "agentSessionRepo.ListByProjectPaginated")
}

func (r *AgentSessionRepo) CountByProject(ctx context.Context, tenantID, projectID uuid.UUID) (int64, error) {
	var count int64

	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_sessions WHERE tenant_id = $1 AND project_id = $2`,
		tenantID, projectID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("agentSessionRepo.CountByProject: %w", err)
	}

	return count, nil
}

func scanAgentSessions(rows pgx.Rows, caller string) ([]*domain.AgentSession, error) {
	var sessions []*domain.AgentSession
	for rows.Next() {
		var s domain.AgentSession
		var metadata []byte

		if err := rows.Scan(
			&s.ID, &s.TenantID, &s.ProjectID, &s.TaskID, &s.AgentType, &s.Status,
			&s.ContainerID, &s.BranchName, &s.StartedAt, &s.CompletedAt,
			&s.Error, &metadata, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", caller, err)
		}
		if err := json.Unmarshal(metadata, &s.Metadata); err != nil {
			return nil, fmt.Errorf("%s: unmarshal metadata: %w", caller, err)
		}
		sessions = append(sessions, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", caller, err)
	}

	return sessions, nil
}
