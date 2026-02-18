package domain

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"
)

type ADRStatus string

const (
	ADRStatusDraft      ADRStatus = "draft"
	ADRStatusProposed   ADRStatus = "proposed"
	ADRStatusAccepted   ADRStatus = "accepted"
	ADRStatusRejected   ADRStatus = "rejected"
	ADRStatusDeprecated ADRStatus = "deprecated"
)

type ADRConsequences struct {
	Good    []string `json:"good"`
	Bad     []string `json:"bad"`
	Neutral []string `json:"neutral"`
}

type ADR struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	ProjectID      uuid.UUID
	Sequence       int // monotonic per project
	Title          string
	Status         ADRStatus
	Context        string // problem statement
	Decision       string
	Drivers        []string
	Options        []string
	Consequences   ADRConsequences
	CreatedBy      *uuid.UUID // nil if agent-created
	AgentSessionID *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ADRRepository interface {
	Create(ctx context.Context, adr *ADR) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*ADR, error)
	ListByProject(ctx context.Context, tenantID, projectID uuid.UUID) ([]*ADR, error)
	ListByProjectPaginated(ctx context.Context, tenantID, projectID uuid.UUID, limit, offset int) ([]*ADR, error)
	UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status ADRStatus) error
}

// ValidADRStatuses is the canonical set of known ADR statuses.
var ValidADRStatuses = []ADRStatus{ //nolint:gochecknoglobals // canonical enum list
	ADRStatusDraft,
	ADRStatusProposed,
	ADRStatusAccepted,
	ADRStatusRejected,
	ADRStatusDeprecated,
}

// ValidateStatus returns true if the given status is a known ADR status.
func ValidateStatus(s ADRStatus) bool {
	return slices.Contains(ValidADRStatuses, s)
}
