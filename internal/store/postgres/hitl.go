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

type HITLRepo struct {
	pool *pgxpool.Pool
}

func NewHITLRepo(pool *pgxpool.Pool) *HITLRepo {
	return &HITLRepo{pool: pool}
}

func (r *HITLRepo) Create(ctx context.Context, q *domain.HITLQuestion) error {
	options, err := json.Marshal(q.Options)
	if err != nil {
		return fmt.Errorf("hitlRepo.Create: marshal options: %w", err)
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO hitl_questions (id, tenant_id, agent_session_id, question, options, messenger_thread_id, messenger_platform, answer, answered_by, status, timeout_at, created_at, answered_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		q.ID, q.TenantID, q.AgentSessionID, q.Question, options,
		q.MessengerThreadID, q.MessengerPlatform, q.Answer, q.AnsweredBy,
		q.Status, q.TimeoutAt, q.CreatedAt, q.AnsweredAt,
	)
	if err != nil {
		return fmt.Errorf("hitlRepo.Create: %w", err)
	}

	return nil
}

func (r *HITLRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.HITLQuestion, error) {
	var q domain.HITLQuestion
	var options []byte

	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, agent_session_id, question, options, messenger_thread_id, messenger_platform,
		        answer, answered_by, status, timeout_at, created_at, answered_at
		 FROM hitl_questions WHERE tenant_id = $1 AND id = $2`,
		tenantID, id,
	).Scan(
		&q.ID, &q.TenantID, &q.AgentSessionID, &q.Question, &options,
		&q.MessengerThreadID, &q.MessengerPlatform, &q.Answer, &q.AnsweredBy,
		&q.Status, &q.TimeoutAt, &q.CreatedAt, &q.AnsweredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("hitlRepo.GetByID: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("hitlRepo.GetByID: %w", err)
	}

	err = json.Unmarshal(options, &q.Options)
	if err != nil {
		return nil, fmt.Errorf("hitlRepo.GetByID: unmarshal options: %w", err)
	}

	return &q, nil
}

func (r *HITLRepo) GetByThreadID(ctx context.Context, tenantID uuid.UUID, platform, threadID string) (*domain.HITLQuestion, error) {
	var q domain.HITLQuestion
	var options []byte

	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, agent_session_id, question, options, messenger_thread_id, messenger_platform,
		        answer, answered_by, status, timeout_at, created_at, answered_at
		 FROM hitl_questions
		 WHERE tenant_id = $1 AND messenger_platform = $2 AND messenger_thread_id = $3`,
		tenantID, platform, threadID,
	).Scan(
		&q.ID, &q.TenantID, &q.AgentSessionID, &q.Question, &options,
		&q.MessengerThreadID, &q.MessengerPlatform, &q.Answer, &q.AnsweredBy,
		&q.Status, &q.TimeoutAt, &q.CreatedAt, &q.AnsweredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("hitlRepo.GetByThreadID: %w", domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("hitlRepo.GetByThreadID: %w", err)
	}

	err = json.Unmarshal(options, &q.Options)
	if err != nil {
		return nil, fmt.Errorf("hitlRepo.GetByThreadID: unmarshal options: %w", err)
	}

	return &q, nil
}

func (r *HITLRepo) Answer(ctx context.Context, tenantID, id uuid.UUID, answer string, answeredBy uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE hitl_questions SET answer = $1, answered_by = $2, answered_at = now(), status = $3
		 WHERE tenant_id = $4 AND id = $5`,
		answer, answeredBy, domain.HITLStatusAnswered, tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("hitlRepo.Answer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("hitlRepo.Answer: %w", domain.ErrNotFound)
	}

	return nil
}

func (r *HITLRepo) ListPending(ctx context.Context, tenantID uuid.UUID) ([]*domain.HITLQuestion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, agent_session_id, question, options, messenger_thread_id, messenger_platform,
		        answer, answered_by, status, timeout_at, created_at, answered_at
		 FROM hitl_questions WHERE tenant_id = $1 AND status = $2
		 ORDER BY created_at
		 LIMIT 500`,
		tenantID, domain.HITLStatusPending,
	)
	if err != nil {
		return nil, fmt.Errorf("hitlRepo.ListPending: %w", err)
	}
	defer rows.Close()

	return scanHITLQuestions(rows, "hitlRepo.ListPending")
}

func (r *HITLRepo) ListExpired(ctx context.Context) ([]*domain.HITLQuestion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, agent_session_id, question, options, messenger_thread_id, messenger_platform,
		        answer, answered_by, status, timeout_at, created_at, answered_at
		 FROM hitl_questions WHERE status = $1 AND timeout_at < now()
		 ORDER BY created_at
		 LIMIT 500`,
		domain.HITLStatusPending,
	)
	if err != nil {
		return nil, fmt.Errorf("hitlRepo.ListExpired: %w", err)
	}
	defer rows.Close()

	return scanHITLQuestions(rows, "hitlRepo.ListExpired")
}

func (r *HITLRepo) Cancel(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE hitl_questions SET status = $1 WHERE tenant_id = $2 AND id = $3`,
		domain.HITLStatusCancelled, tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("hitlRepo.Cancel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("hitlRepo.Cancel: %w", domain.ErrNotFound)
	}

	return nil
}

func scanHITLQuestions(rows pgx.Rows, caller string) ([]*domain.HITLQuestion, error) {
	var questions []*domain.HITLQuestion
	for rows.Next() {
		var q domain.HITLQuestion
		var options []byte

		if err := rows.Scan(
			&q.ID, &q.TenantID, &q.AgentSessionID, &q.Question, &options,
			&q.MessengerThreadID, &q.MessengerPlatform, &q.Answer, &q.AnsweredBy,
			&q.Status, &q.TimeoutAt, &q.CreatedAt, &q.AnsweredAt,
		); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", caller, err)
		}
		if err := json.Unmarshal(options, &q.Options); err != nil {
			return nil, fmt.Errorf("%s: unmarshal options: %w", caller, err)
		}
		questions = append(questions, &q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", caller, err)
	}

	return questions, nil
}
