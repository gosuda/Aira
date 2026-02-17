package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gosuda/aira/internal/domain"
)

type SessionLogRepo struct {
	pool *pgxpool.Pool
}

func NewSessionLogRepo(pool *pgxpool.Pool) *SessionLogRepo {
	return &SessionLogRepo{pool: pool}
}

func (r *SessionLogRepo) Append(ctx context.Context, entry *domain.SessionLogEntry) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO session_log_entries (id, session_id, tenant_id, entry_type, content, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		entry.ID, entry.SessionID, entry.TenantID, entry.EntryType, entry.Content, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("sessionLogRepo.Append: %w", err)
	}

	return nil
}

func (r *SessionLogRepo) ListBySession(ctx context.Context, tenantID, sessionID uuid.UUID, limit, offset int) ([]*domain.SessionLogEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, session_id, tenant_id, entry_type, content, created_at
		 FROM session_log_entries WHERE tenant_id = $1 AND session_id = $2
		 ORDER BY created_at ASC
		 LIMIT $3 OFFSET $4`,
		tenantID, sessionID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("sessionLogRepo.ListBySession: %w", err)
	}
	defer rows.Close()

	var entries []*domain.SessionLogEntry
	for rows.Next() {
		var e domain.SessionLogEntry

		err = rows.Scan(&e.ID, &e.SessionID, &e.TenantID, &e.EntryType, &e.Content, &e.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("sessionLogRepo.ListBySession: scan: %w", err)
		}
		entries = append(entries, &e)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("sessionLogRepo.ListBySession: rows: %w", err)
	}

	return entries, nil
}

func (r *SessionLogRepo) CountBySession(ctx context.Context, tenantID, sessionID uuid.UUID) (int64, error) {
	var count int64

	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM session_log_entries WHERE tenant_id = $1 AND session_id = $2`,
		tenantID, sessionID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sessionLogRepo.CountBySession: %w", err)
	}

	return count, nil
}
