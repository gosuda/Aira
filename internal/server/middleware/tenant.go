package middleware

import (
	"net/http"

	"github.com/google/uuid"
)

func RequireTenant() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tid, ok := TenantIDFromContext(r.Context())
			if !ok || tid == uuid.Nil {
				http.Error(w, `{"title":"Forbidden","status":403,"detail":"valid tenant required"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
