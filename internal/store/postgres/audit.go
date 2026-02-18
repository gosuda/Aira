package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gosuda/aira/internal/domain"
)

type AuditRepo struct {
	pool *pgxpool.Pool
}

func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

func (r *AuditRepo) Record(ctx context.Context, entry *domain.AuditEntry) error {
	details, err := json.Marshal(entry.Details)
	if err != nil {
		return fmt.Errorf("auditRepo.Record: marshal details: %w", err)
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO audit_log (id, tenant_id, actor_type, actor_id, action, resource, resource_id, details, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		entry.ID, entry.TenantID, entry.ActorType, entry.ActorID,
		entry.Action, entry.Resource, entry.ResourceID,
		details, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("auditRepo.Record: %w", err)
	}

	return nil
}

func (r *AuditRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*domain.AuditEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, actor_type, actor_id, action, resource, resource_id, details, created_at
		 FROM audit_log WHERE tenant_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		tenantID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("auditRepo.ListByTenant: %w", err)
	}
	defer rows.Close()

	return scanAuditEntries(rows, "auditRepo.ListByTenant")
}

func (r *AuditRepo) ListByResource(ctx context.Context, tenantID uuid.UUID, resource string, resourceID uuid.UUID) ([]*domain.AuditEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, actor_type, actor_id, action, resource, resource_id, details, created_at
		 FROM audit_log WHERE tenant_id = $1 AND resource = $2 AND resource_id = $3
		 ORDER BY created_at DESC`,
		tenantID, resource, resourceID,
	)
	if err != nil {
		return nil, fmt.Errorf("auditRepo.ListByResource: %w", err)
	}
	defer rows.Close()

	return scanAuditEntries(rows, "auditRepo.ListByResource")
}

func scanAuditEntries(rows pgx.Rows, caller string) ([]*domain.AuditEntry, error) {
	var entries []*domain.AuditEntry
	for rows.Next() {
		var e domain.AuditEntry
		var details []byte

		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.ActorType, &e.ActorID, &e.Action,
			&e.Resource, &e.ResourceID, &details, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", caller, err)
		}
		if err := json.Unmarshal(details, &e.Details); err != nil {
			return nil, fmt.Errorf("%s: unmarshal details: %w", caller, err)
		}
		entries = append(entries, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", caller, err)
	}

	return entries, nil
}
