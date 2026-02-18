package v1

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/server/middleware"
)

type CreateTaskInput struct {
	Body struct {
		ProjectID   uuid.UUID  `json:"project_id" doc:"Project ID"`
		Title       string     `json:"title" minLength:"1" maxLength:"500" doc:"Task title"`
		Description string     `json:"description,omitempty" doc:"Task description"`
		ADRID       *uuid.UUID `json:"adr_id,omitempty" doc:"Optional parent ADR ID"`
		Priority    int        `json:"priority,omitempty" doc:"Task priority (0=default)"`
	}
}

type CreateTaskOutput struct {
	Body *domain.Task
}

type ListTasksInput struct {
	ProjectID uuid.UUID `query:"project_id" required:"true" doc:"Project ID"`
	Status    string    `query:"status" doc:"Filter by status"`
}

type ListTasksOutput struct {
	Body []*domain.Task
}

type GetTaskInput struct {
	ID uuid.UUID `path:"id" doc:"Task ID"`
}

type GetTaskOutput struct {
	Body *domain.Task
}

type UpdateTaskInput struct {
	ID   uuid.UUID `path:"id" doc:"Task ID"`
	Body struct {
		Title       string     `json:"title,omitempty" maxLength:"500" doc:"Task title"`
		Description string     `json:"description,omitempty" doc:"Task description"`
		ADRID       *uuid.UUID `json:"adr_id,omitempty" doc:"Parent ADR ID"`
		Priority    *int       `json:"priority,omitempty" doc:"Task priority"`
		AssignedTo  *uuid.UUID `json:"assigned_to,omitempty" doc:"Assigned user ID"`
	}
}

type UpdateTaskOutput struct {
	Body *domain.Task
}

type TransitionTaskStatusInput struct {
	ID   uuid.UUID `path:"id" doc:"Task ID"`
	Body struct {
		Status string `json:"status" minLength:"1" doc:"Target status"`
	}
}

type TransitionTaskStatusOutput struct {
	Body *domain.Task
}

type DeleteTaskInput struct {
	ID uuid.UUID `path:"id" doc:"Task ID"`
}

func RegisterTaskRoutes(api huma.API, store DataStore) {
	huma.Register(api, huma.Operation{
		OperationID: "create-task",
		Method:      http.MethodPost,
		Path:        "/tasks",
		Summary:     "Create a new task",
		Tags:        []string{"Tasks"},
	}, func(ctx context.Context, input *CreateTaskInput) (*CreateTaskOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		if _, err := store.Projects().GetByID(ctx, tenantID, input.Body.ProjectID); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("project not found")
			}
			return nil, huma.Error500InternalServerError("failed to validate project")
		}

		if input.Body.ADRID != nil {
			if _, err := store.ADRs().GetByID(ctx, tenantID, *input.Body.ADRID); err != nil {
				if errors.Is(err, domain.ErrNotFound) {
					return nil, huma.Error404NotFound("ADR not found")
				}
				return nil, huma.Error500InternalServerError("failed to validate ADR")
			}
		}

		now := time.Now()
		t := &domain.Task{
			ID:          uuid.New(),
			TenantID:    tenantID,
			ProjectID:   input.Body.ProjectID,
			ADRID:       input.Body.ADRID,
			Title:       input.Body.Title,
			Description: input.Body.Description,
			Status:      domain.TaskStatusBacklog,
			Priority:    input.Body.Priority,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := store.Tasks().Create(ctx, t); err != nil {
			return nil, huma.Error500InternalServerError("failed to create task", err)
		}

		return &CreateTaskOutput{Body: t}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-tasks",
		Method:      http.MethodGet,
		Path:        "/tasks",
		Summary:     "List tasks for a project",
		Tags:        []string{"Tasks"},
	}, func(ctx context.Context, input *ListTasksInput) (*ListTasksOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		if input.Status != "" {
			status := domain.TaskStatus(input.Status)
			tasks, err := store.Tasks().ListByStatus(ctx, tenantID, input.ProjectID, status)
			if err != nil {
				return nil, huma.Error500InternalServerError("failed to list tasks", err)
			}
			return &ListTasksOutput{Body: tasks}, nil
		}

		tasks, err := store.Tasks().ListByProject(ctx, tenantID, input.ProjectID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list tasks", err)
		}

		return &ListTasksOutput{Body: tasks}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-task",
		Method:      http.MethodGet,
		Path:        "/tasks/{id}",
		Summary:     "Get a task by ID",
		Tags:        []string{"Tasks"},
	}, func(ctx context.Context, input *GetTaskInput) (*GetTaskOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		t, err := store.Tasks().GetByID(ctx, tenantID, input.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("task not found")
			}
			return nil, huma.Error500InternalServerError("failed to get task", err)
		}

		return &GetTaskOutput{Body: t}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-task",
		Method:      http.MethodPut,
		Path:        "/tasks/{id}",
		Summary:     "Update a task",
		Tags:        []string{"Tasks"},
	}, func(ctx context.Context, input *UpdateTaskInput) (*UpdateTaskOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		existing, err := store.Tasks().GetByID(ctx, tenantID, input.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("task not found")
			}
			return nil, huma.Error500InternalServerError("failed to get task", err)
		}

		if input.Body.Title != "" {
			existing.Title = input.Body.Title
		}
		if input.Body.Description != "" {
			existing.Description = input.Body.Description
		}
		if input.Body.ADRID != nil {
			existing.ADRID = input.Body.ADRID
		}
		if input.Body.Priority != nil {
			existing.Priority = *input.Body.Priority
		}
		if input.Body.AssignedTo != nil {
			existing.AssignedTo = input.Body.AssignedTo
		}
		existing.UpdatedAt = time.Now()

		err = store.Tasks().Update(ctx, existing)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to update task", err)
		}

		return &UpdateTaskOutput{Body: existing}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "transition-task-status",
		Method:      http.MethodPatch,
		Path:        "/tasks/{id}/status",
		Summary:     "Transition task status",
		Tags:        []string{"Tasks"},
	}, func(ctx context.Context, input *TransitionTaskStatusInput) (*TransitionTaskStatusOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		existing, err := store.Tasks().GetByID(ctx, tenantID, input.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("task not found")
			}
			return nil, huma.Error500InternalServerError("failed to get task", err)
		}

		target := domain.TaskStatus(input.Body.Status)
		switch target {
		case domain.TaskStatusBacklog, domain.TaskStatusInProgress, domain.TaskStatusReview, domain.TaskStatusDone:
			// valid
		default:
			return nil, huma.Error400BadRequest("unknown task status: " + input.Body.Status)
		}
		if !existing.Status.ValidTransition(target) {
			return nil, huma.Error400BadRequest("invalid status transition from " + string(existing.Status) + " to " + string(target))
		}

		err = store.Tasks().UpdateStatus(ctx, tenantID, input.ID, target)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to update task status", err)
		}

		existing.Status = target
		existing.UpdatedAt = time.Now()

		return &TransitionTaskStatusOutput{Body: existing}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-task",
		Method:      http.MethodDelete,
		Path:        "/tasks/{id}",
		Summary:     "Delete a task",
		Tags:        []string{"Tasks"},
	}, func(ctx context.Context, input *DeleteTaskInput) (*struct{}, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		if err := store.Tasks().Delete(ctx, tenantID, input.ID); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("task not found")
			}
			return nil, huma.Error500InternalServerError("failed to delete task", err)
		}

		return nil, nil
	})
}
