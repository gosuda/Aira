package middleware

import "net/http"

// Role constants define the supported user roles.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleViewer = "viewer"
)

// RequireRole returns middleware that checks if the authenticated user has one
// of the allowed roles. It must be chained after the Auth middleware, which
// stores the user role in the request context via ContextKeyUserRole.
//
// Returns 401 Unauthorized when no user is found in context (Auth middleware
// not applied or authentication failed). Returns 403 Forbidden when the user
// role does not match any of the allowed roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := RoleFromContext(r.Context())
			if !ok || role == "" {
				http.Error(w, `{"title":"Unauthorized","status":401,"detail":"authentication required"}`, http.StatusUnauthorized)
				return
			}

			if _, match := allowed[role]; !match {
				http.Error(w, `{"title":"Forbidden","status":403,"detail":"insufficient permissions"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin is a convenience wrapper for RequireRole(RoleAdmin).
func RequireAdmin() func(http.Handler) http.Handler {
	return RequireRole(RoleAdmin)
}
