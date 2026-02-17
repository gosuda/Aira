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

type ADRRepo struct {
	pool *pgxpool.Pool
}

func NewADRRepo(pool *pgxpool.Pool) *ADRRepo {
	return &ADRRepo{pool: pool}
}

func (r *ADRRepo) Create(ctx context.Context, adr *domain.ADR) error {
	drivers, err := json.Marshal(adr.Drivers)
	if err != nil {
		return fmt.Errorf("adrRepo.Create: marshal drivers: %w", err)
	}

	options, err := json.Marshal(adr.Options)
	if err != nil {
		return fmt.Errorf("adrRepo.Create: marshal options: %w", err)
	}

	consequences, err := json.Marshal(adr.Consequences)
	if err != nil {
		return fmt.Errorf("adrRepo.Create: marshal consequences: %w", err)
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO adrs (id, tenant_id, project_id, sequence, title, status, context, decision, drivers, options, consequences, created_by, agent_session_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		adr.ID, adr.TenantID, adr.ProjectID, adr.Sequence, adr.Title, adr.Status,
		adr.Context, adr.Decision, drivers, options, consequences,
		adr.CreatedBy, adr.AgentSessionID, adr.CreatedAt, adr.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("adrRepo.Create: %w", err)
	}

	return nil
}

func (r *ADRRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.ADR, error) {
	var adr domain.ADR
	var drivers, options, consequences []byte

	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, project_id, sequence, title, status, context, decision,
		        drivers, options, consequences, created_by, agent_session_id, created_at, updated_at
		 FROM adrs WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	).Scan(
		&adr.ID, &adr.TenantID, &adr.ProjectID, &adr.Sequence, &adr.Title, &adr.Status,
		&adr.Context, &adr.Decision, &drivers, &options, &consequences,
		&adr.CreatedBy, &adr.AgentSessionID, &adr.CreatedAt, &adr.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("adrRepo.GetByID: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("adrRepo.GetByID: %w", err)
	}

	err = unmarshalADRJSON(&adr, drivers, options, consequences)
	if err != nil {
		return nil, fmt.Errorf("adrRepo.GetByID: %w", err)
	}

	return &adr, nil
}

func (r *ADRRepo) ListByProject(ctx context.Context, tenantID, projectID uuid.UUID) ([]*domain.ADR, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, project_id, sequence, title, status, context, decision,
		        drivers, options, consequences, created_by, agent_session_id, created_at, updated_at
		 FROM adrs WHERE tenant_id = $1 AND project_id = $2
		 ORDER BY sequence DESC`,
		tenantID, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("adrRepo.ListByProject: %w", err)
	}
	defer rows.Close()

	var adrs []*domain.ADR
	for rows.Next() {
		var adr domain.ADR
		var drivers, options, consequences []byte

		err = rows.Scan(
			&adr.ID, &adr.TenantID, &adr.ProjectID, &adr.Sequence, &adr.Title, &adr.Status,
			&adr.Context, &adr.Decision, &drivers, &options, &consequences,
			&adr.CreatedBy, &adr.AgentSessionID, &adr.CreatedAt, &adr.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("adrRepo.ListByProject: scan: %w", err)
		}
		err = unmarshalADRJSON(&adr, drivers, options, consequences)
		if err != nil {
			return nil, fmt.Errorf("adrRepo.ListByProject: %w", err)
		}
		adrs = append(adrs, &adr)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("adrRepo.ListByProject: rows: %w", err)
	}

	return adrs, nil
}

func (r *ADRRepo) UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status domain.ADRStatus) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE adrs SET status = $1, updated_at = now() WHERE tenant_id = $2 AND id = $3`,
		status, tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("adrRepo.UpdateStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("adrRepo.UpdateStatus: %w", domain.ErrNotFound)
	}

	return nil
}

func (r *ADRRepo) NextSequence(ctx context.Context, projectID uuid.UUID) (int, error) {
	var seq int

	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(sequence), 0) + 1 FROM adrs WHERE project_id = $1`,
		projectID,
	).Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("adrRepo.NextSequence: %w", err)
	}

	return seq, nil
}

func unmarshalADRJSON(adr *domain.ADR, drivers, options, consequences []byte) error {
	if err := json.Unmarshal(drivers, &adr.Drivers); err != nil {
		return fmt.Errorf("unmarshal drivers: %w", err)
	}
	if err := json.Unmarshal(options, &adr.Options); err != nil {
		return fmt.Errorf("unmarshal options: %w", err)
	}
	if err := json.Unmarshal(consequences, &adr.Consequences); err != nil {
		return fmt.Errorf("unmarshal consequences: %w", err)
	}

	return nil
}
