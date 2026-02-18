package v1

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/server/middleware"
)

type ListADRsInput struct {
	ProjectID uuid.UUID `query:"project_id" required:"true" doc:"Project ID"`
}

type ListADRsOutput struct {
	Body []*domain.ADR
}

type GetADRInput struct {
	ID uuid.UUID `path:"id" doc:"ADR ID"`
}

type GetADROutput struct {
	Body *domain.ADR
}

type UpdateADRStatusInput struct {
	ID   uuid.UUID `path:"id" doc:"ADR ID"`
	Body struct {
		Status string `json:"status" minLength:"1" doc:"New ADR status"`
	}
}

type UpdateADRStatusOutput struct {
	Body *domain.ADR
}

func adrTransitions() map[domain.ADRStatus][]domain.ADRStatus {
	return map[domain.ADRStatus][]domain.ADRStatus{
		domain.ADRStatusDraft:    {domain.ADRStatusProposed},
		domain.ADRStatusProposed: {domain.ADRStatusAccepted, domain.ADRStatusRejected},
		domain.ADRStatusAccepted: {domain.ADRStatusDeprecated},
	}
}

func isValidADRTransition(from, to domain.ADRStatus) bool {
	allowed, ok := adrTransitions()[from]
	if !ok {
		return false
	}
	return slices.Contains(allowed, to)
}

func RegisterADRRoutes(api huma.API, store DataStore) {
	huma.Register(api, huma.Operation{
		OperationID: "list-adrs",
		Method:      http.MethodGet,
		Path:        "/adrs",
		Summary:     "List ADRs for a project",
		Tags:        []string{"ADRs"},
	}, func(ctx context.Context, input *ListADRsInput) (*ListADRsOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		adrs, err := store.ADRs().ListByProject(ctx, tenantID, input.ProjectID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list ADRs", err)
		}

		return &ListADRsOutput{Body: adrs}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-adr",
		Method:      http.MethodGet,
		Path:        "/adrs/{id}",
		Summary:     "Get an ADR by ID",
		Tags:        []string{"ADRs"},
	}, func(ctx context.Context, input *GetADRInput) (*GetADROutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		adr, err := store.ADRs().GetByID(ctx, tenantID, input.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("ADR not found")
			}
			return nil, huma.Error500InternalServerError("failed to get ADR", err)
		}

		return &GetADROutput{Body: adr}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-adr-status",
		Method:      http.MethodPatch,
		Path:        "/adrs/{id}/status",
		Summary:     "Update ADR status",
		Tags:        []string{"ADRs"},
	}, func(ctx context.Context, input *UpdateADRStatusInput) (*UpdateADRStatusOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		existing, err := store.ADRs().GetByID(ctx, tenantID, input.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("ADR not found")
			}
			return nil, huma.Error500InternalServerError("failed to get ADR", err)
		}

		target := domain.ADRStatus(input.Body.Status)
		switch target {
		case domain.ADRStatusDraft, domain.ADRStatusProposed, domain.ADRStatusAccepted, domain.ADRStatusRejected, domain.ADRStatusDeprecated:
			// valid
		default:
			return nil, huma.Error400BadRequest("unknown ADR status: " + input.Body.Status)
		}
		if !isValidADRTransition(existing.Status, target) {
			return nil, huma.Error400BadRequest("invalid status transition from " + string(existing.Status) + " to " + string(target))
		}

		err = store.ADRs().UpdateStatus(ctx, tenantID, input.ID, target)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to update ADR status", err)
		}

		existing.Status = target

		return &UpdateADRStatusOutput{Body: existing}, nil
	})
}
