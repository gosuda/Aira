package middleware

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const (
	ContextKeyTenantID contextKey = "tenant_id"
	ContextKeyUserID   contextKey = "user_id"
	ContextKeyUserRole contextKey = "role"
)

func TenantIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ContextKeyTenantID).(uuid.UUID)
	return v, ok
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ContextKeyUserID).(uuid.UUID)
	return v, ok
}

func RoleFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeyUserRole).(string)
	return v, ok
}
