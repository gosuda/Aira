package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/server/middleware"
)

type CreateTenantInput struct {
	Body struct {
		Name string `json:"name" minLength:"1" maxLength:"255" doc:"Tenant name"`
		Slug string `json:"slug" minLength:"1" maxLength:"63" pattern:"^[a-z0-9]+(?:-[a-z0-9]+)*$" doc:"URL-safe slug (lowercase alphanumeric with hyphens)"`
	}
}

type CreateTenantOutput struct {
	Body *domain.Tenant
}

type ListTenantsInput struct {
	Limit  int `query:"limit" minimum:"1" maximum:"200" default:"50" doc:"Max results"`
	Offset int `query:"offset" minimum:"0" default:"0" doc:"Offset for pagination"`
}

type ListTenantsOutput struct {
	Body []*domain.Tenant
}

func RegisterTenantRoutes(api huma.API, store DataStore) {
	huma.Register(api, huma.Operation{
		OperationID: "create-tenant",
		Method:      http.MethodPost,
		Path:        "/tenants",
		Summary:     "Create a new tenant",
		Tags:        []string{"Tenants"},
	}, func(ctx context.Context, input *CreateTenantInput) (*CreateTenantOutput, error) {
		role, ok := middleware.RoleFromContext(ctx)
		if !ok || role != "admin" {
			return nil, huma.Error403Forbidden("admin role required")
		}

		now := time.Now()
		t := &domain.Tenant{
			ID:        uuid.New(),
			Name:      input.Body.Name,
			Slug:      input.Body.Slug,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := store.Tenants().Create(ctx, t); err != nil {
			return nil, huma.Error500InternalServerError("failed to create tenant", err)
		}

		return &CreateTenantOutput{Body: t}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-tenants",
		Method:      http.MethodGet,
		Path:        "/tenants",
		Summary:     "List all tenants",
		Tags:        []string{"Tenants"},
	}, func(ctx context.Context, input *ListTenantsInput) (*ListTenantsOutput, error) {
		role, ok := middleware.RoleFromContext(ctx)
		if !ok || role != "admin" {
			return nil, huma.Error403Forbidden("admin role required")
		}

		tenants, err := store.Tenants().ListPaginated(ctx, input.Limit, input.Offset)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list tenants", err)
		}

		return &ListTenantsOutput{Body: tenants}, nil
	})
}
