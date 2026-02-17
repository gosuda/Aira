package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
)

// ErrInvalidADRInput is returned when ADR tool input fails validation.
var ErrInvalidADRInput = errors.New("agent: invalid ADR tool input") //nolint:gochecknoglobals // sentinel error

// ADRToolInput represents the structured input an agent emits when invoking the create_adr tool.
type ADRToolInput struct {
	Title        string          `json:"title"`
	Context      string          `json:"context"`      // problem statement
	Decision     string          `json:"decision"`     // what was decided
	Drivers      []string        `json:"drivers"`      // decision drivers
	Options      []string        `json:"options"`      // considered alternatives
	Consequences ADRConsequences `json:"consequences"` // good/bad/neutral
	ProjectID    uuid.UUID       `json:"project_id"`   // target project
	TenantID     uuid.UUID       `json:"tenant_id"`    // owning tenant
	SessionID    uuid.UUID       `json:"session_id"`   // agent session that produced this
}

// ADRConsequences mirrors domain.ADRConsequences for the tool input layer.
type ADRConsequences struct {
	Good    []string `json:"good"`
	Bad     []string `json:"bad"`
	Neutral []string `json:"neutral"`
}

// ValidateADRToolInput checks required fields on an ADRToolInput.
func ValidateADRToolInput(input *ADRToolInput) error {
	if input.Title == "" {
		return fmt.Errorf("%w: title is required", ErrInvalidADRInput)
	}
	if input.Context == "" {
		return fmt.Errorf("%w: context is required", ErrInvalidADRInput)
	}
	if input.Decision == "" {
		return fmt.Errorf("%w: decision is required", ErrInvalidADRInput)
	}
	if input.ProjectID == uuid.Nil {
		return fmt.Errorf("%w: project_id is required", ErrInvalidADRInput)
	}
	if input.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidADRInput)
	}

	return nil
}

// ADRToolDeps contains the repository dependencies needed to process an ADR tool call.
type ADRToolDeps struct {
	ADRs     domain.ADRRepository
	Projects domain.ProjectRepository
}

// ProcessADRToolCall validates the input, allocates a sequence number, creates a domain.ADR,
// and persists it via the ADRRepository. Returns the created ADR or an error.
func ProcessADRToolCall(ctx context.Context, deps ADRToolDeps, input *ADRToolInput) (*domain.ADR, error) {
	if err := ValidateADRToolInput(input); err != nil {
		return nil, fmt.Errorf("agent.ProcessADRToolCall: %w", err)
	}

	// Verify project exists.
	_, err := deps.Projects.GetByID(ctx, input.TenantID, input.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("agent.ProcessADRToolCall: get project: %w", err)
	}

	// Allocate next sequence number for the project.
	seq, err := deps.ADRs.NextSequence(ctx, input.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("agent.ProcessADRToolCall: next sequence: %w", err)
	}

	sessionID := input.SessionID
	now := time.Now()

	adr := &domain.ADR{
		ID:        uuid.New(),
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
		Sequence:  seq,
		Title:     input.Title,
		Status:    domain.ADRStatusProposed,
		Context:   input.Context,
		Decision:  input.Decision,
		Drivers:   input.Drivers,
		Options:   input.Options,
		Consequences: domain.ADRConsequences{
			Good:    input.Consequences.Good,
			Bad:     input.Consequences.Bad,
			Neutral: input.Consequences.Neutral,
		},
		CreatedBy:      nil, // agent-created, no user
		AgentSessionID: &sessionID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	createErr := deps.ADRs.Create(ctx, adr)
	if createErr != nil {
		return nil, fmt.Errorf("agent.ProcessADRToolCall: create ADR: %w", createErr)
	}

	return adr, nil
}
