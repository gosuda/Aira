package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type HITLStatus string

const (
	HITLStatusPending   HITLStatus = "pending"
	HITLStatusAnswered  HITLStatus = "answered"
	HITLStatusTimeout   HITLStatus = "timeout"
	HITLStatusCancelled HITLStatus = "cancelled"
)

type HITLQuestion struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	AgentSessionID    uuid.UUID
	Question          string
	Options           []string // structured options if any
	MessengerThreadID string
	MessengerPlatform string
	Answer            string
	AnsweredBy        *uuid.UUID
	Status            HITLStatus
	TimeoutAt         *time.Time
	CreatedAt         time.Time
	AnsweredAt        *time.Time
}

type HITLQuestionRepository interface {
	Create(ctx context.Context, q *HITLQuestion) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*HITLQuestion, error)
	GetByThreadID(ctx context.Context, tenantID uuid.UUID, platform, threadID string) (*HITLQuestion, error)
	Answer(ctx context.Context, tenantID, id uuid.UUID, answer string, answeredBy uuid.UUID) error
	ListPending(ctx context.Context, tenantID uuid.UUID) ([]*HITLQuestion, error)
	ListExpired(ctx context.Context) ([]*HITLQuestion, error) // no tenant filter - background job
	Cancel(ctx context.Context, tenantID, id uuid.UUID) error
}
