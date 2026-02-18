package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/server/middleware"
)

type GetBoardInput struct {
	ProjectID uuid.UUID `path:"projectID" doc:"Project ID"`
}

type BoardColumn struct {
	Backlog    []*domain.Task `json:"backlog"`
	InProgress []*domain.Task `json:"in_progress"`
	Review     []*domain.Task `json:"review"`
	Done       []*domain.Task `json:"done"`
}

type GetBoardOutput struct {
	Body *BoardColumn
}

func RegisterBoardRoutes(api huma.API, store DataStore) {
	huma.Register(api, huma.Operation{
		OperationID: "get-board",
		Method:      http.MethodGet,
		Path:        "/boards/{projectID}",
		Summary:     "Get kanban board for a project",
		Tags:        []string{"Boards"},
	}, func(ctx context.Context, input *GetBoardInput) (*GetBoardOutput, error) {
		tenantID, ok := middleware.TenantIDFromContext(ctx)
		if !ok {
			return nil, huma.Error403Forbidden("missing tenant context")
		}

		tasks, err := store.Tasks().ListByProject(ctx, tenantID, input.ProjectID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list tasks for board", err)
		}

		board := &BoardColumn{
			Backlog:    make([]*domain.Task, 0),
			InProgress: make([]*domain.Task, 0),
			Review:     make([]*domain.Task, 0),
			Done:       make([]*domain.Task, 0),
		}

		for _, t := range tasks {
			switch t.Status {
			case domain.TaskStatusBacklog:
				board.Backlog = append(board.Backlog, t)
			case domain.TaskStatusInProgress:
				board.InProgress = append(board.InProgress, t)
			case domain.TaskStatusReview:
				board.Review = append(board.Review, t)
			case domain.TaskStatusDone:
				board.Done = append(board.Done, t)
			}
		}

		return &GetBoardOutput{Body: board}, nil
	})
}
