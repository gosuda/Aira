package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/server/middleware"
	"github.com/gosuda/aira/internal/store/postgres"
)

type CreateProjectInput struct {
	Body struct {
		Name     string          `json:"name" minLength:"1" maxLength:"255" doc:"Project name"`
		RepoURL  string          `json:"repo_url" minLength:"1" doc:"Git clone URL"`
		Branch   string          `json:"branch,omitempty" doc:"Default branch"`
		Settings json.RawMessage `json:"settings,omitempty" doc:"Project settings"`
	}
}

type CreateProjectOutput struct {
	Body *domain.Project
}

type ListProjectsInput struct{}

type ListProjectsOutput struct {
	Body []*domain.Project
}

type GetProjectInput struct {
	ID uuid.UUID `path:"id" doc:"Project ID"`
}

type GetProjectOutput struct {
	Body *domain.Project
}

type UpdateProjectInput struct {
	ID   uuid.UUID `path:"id" doc:"Project ID"`
	Body struct {
		Name     string          `json:"name,omitempty" maxLength:"255" doc:"Project name"`
		RepoURL  string          `json:"repo_url,omitempty" doc:"Git clone URL"`
		Branch   string          `json:"branch,omitempty" doc:"Default branch"`
		Settings json.RawMessage `json:"settings,omitempty" doc:"Project settings"`
	}
}

type UpdateProjectOutput struct {
	Body *domain.Project
}

type DeleteProjectInput struct {
	ID uuid.UUID `path:"id" doc:"Project ID"`
}

func RegisterProjectRoutes(api huma.API, store *postgres.Store) {
	huma.Register(api, huma.Operation{
		OperationID: "create-project",
		Method:      http.MethodPost,
		Path:        "/projects",
		Summary:     "Create a new project",
		Tags:        []string{"Projects"},
	}, func(ctx context.Context, input *CreateProjectInput) (*CreateProjectOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		if !strings.HasPrefix(input.Body.RepoURL, "https://") && !strings.HasPrefix(input.Body.RepoURL, "git@") {
			return nil, huma.Error400BadRequest("repo_url must use https:// or git@ scheme")
		}

		p, err := domain.NewProject(tenantID, input.Body.Name, input.Body.RepoURL, input.Body.Branch, input.Body.Settings)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}

		if createErr := store.Projects().Create(ctx, p); createErr != nil {
			return nil, huma.Error500InternalServerError("failed to create project", createErr)
		}

		return &CreateProjectOutput{Body: p}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-projects",
		Method:      http.MethodGet,
		Path:        "/projects",
		Summary:     "List projects in current tenant",
		Tags:        []string{"Projects"},
	}, func(ctx context.Context, _ *ListProjectsInput) (*ListProjectsOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		projects, err := store.Projects().List(ctx, tenantID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list projects", err)
		}

		return &ListProjectsOutput{Body: projects}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-project",
		Method:      http.MethodGet,
		Path:        "/projects/{id}",
		Summary:     "Get a project by ID",
		Tags:        []string{"Projects"},
	}, func(ctx context.Context, input *GetProjectInput) (*GetProjectOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		p, err := store.Projects().GetByID(ctx, tenantID, input.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("project not found")
			}
			return nil, huma.Error500InternalServerError("failed to get project", err)
		}

		return &GetProjectOutput{Body: p}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-project",
		Method:      http.MethodPut,
		Path:        "/projects/{id}",
		Summary:     "Update a project",
		Tags:        []string{"Projects"},
	}, func(ctx context.Context, input *UpdateProjectInput) (*UpdateProjectOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		existing, err := store.Projects().GetByID(ctx, tenantID, input.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("project not found")
			}
			return nil, huma.Error500InternalServerError("failed to get project", err)
		}

		if input.Body.Name != "" {
			existing.Name = input.Body.Name
		}
		if input.Body.RepoURL != "" {
			existing.RepoURL = input.Body.RepoURL
		}
		if input.Body.Branch != "" {
			existing.Branch = input.Body.Branch
		}
		if input.Body.Settings != nil {
			existing.Settings = input.Body.Settings
		}

		err = store.Projects().Update(ctx, existing)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to update project", err)
		}

		return &UpdateProjectOutput{Body: existing}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-project",
		Method:      http.MethodDelete,
		Path:        "/projects/{id}",
		Summary:     "Delete a project",
		Tags:        []string{"Projects"},
	}, func(ctx context.Context, input *DeleteProjectInput) (*struct{}, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		if err := store.Projects().Delete(ctx, tenantID, input.ID); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("project not found")
			}
			return nil, huma.Error500InternalServerError("failed to delete project", err)
		}

		return nil, nil
	})
}
