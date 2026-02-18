package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/server/middleware"
)

// setRole injects a role into the request context using the same context key
// that the Auth middleware uses.
func setRole(r *http.Request, role string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ContextKeyUserRole, role)
	return r.WithContext(ctx)
}

// okHandler is a simple handler that writes 200 OK.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestRequireRole_AllowsMatchingRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		allowedRoles []string
		userRole     string
	}{
		{name: "admin allowed for admin-only", allowedRoles: []string{middleware.RoleAdmin}, userRole: middleware.RoleAdmin},
		{name: "member allowed for member-only", allowedRoles: []string{middleware.RoleMember}, userRole: middleware.RoleMember},
		{name: "viewer allowed for viewer-only", allowedRoles: []string{middleware.RoleViewer}, userRole: middleware.RoleViewer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := middleware.RequireRole(tt.allowedRoles...)(okHandler)
			req := setRole(httptest.NewRequest(http.MethodGet, "/", http.NoBody), tt.userRole)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestRequireRole_BlocksNonMatchingRole(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireRole(middleware.RoleAdmin)(okHandler)
	req := setRole(httptest.NewRequest(http.MethodGet, "/", http.NoBody), middleware.RoleMember)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "insufficient permissions")
}

func TestRequireRole_MultipleAllowedRoles(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireRole(middleware.RoleAdmin, middleware.RoleMember)(okHandler)

	tests := []struct {
		name       string
		role       string
		wantStatus int
	}{
		{name: "admin passes", role: middleware.RoleAdmin, wantStatus: http.StatusOK},
		{name: "member passes", role: middleware.RoleMember, wantStatus: http.StatusOK},
		{name: "viewer blocked", role: middleware.RoleViewer, wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := setRole(httptest.NewRequest(http.MethodGet, "/", http.NoBody), tt.role)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestRequireAdmin_ConvenienceWrapper(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireAdmin()(okHandler)

	t.Run("admin passes", func(t *testing.T) {
		t.Parallel()

		req := setRole(httptest.NewRequest(http.MethodGet, "/", http.NoBody), middleware.RoleAdmin)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("member blocked", func(t *testing.T) {
		t.Parallel()

		req := setRole(httptest.NewRequest(http.MethodGet, "/", http.NoBody), middleware.RoleMember)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("viewer blocked", func(t *testing.T) {
		t.Parallel()

		req := setRole(httptest.NewRequest(http.MethodGet, "/", http.NoBody), middleware.RoleViewer)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestRequireRole_NoUserInContext_Returns401(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireRole(middleware.RoleAdmin)(okHandler)

	// Request without any role in context (Auth middleware not applied).
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "authentication required")
}

func TestRequireRole_EmptyRoleInContext_Returns401(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireRole(middleware.RoleAdmin)(okHandler)

	// Request with an empty-string role in context.
	req := setRole(httptest.NewRequest(http.MethodGet, "/", http.NoBody), "")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "authentication required")
}
