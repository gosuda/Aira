package middleware

import (
	"net/http"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

func RateLimit(requestsPerSecond float64, burst int) func(http.Handler) http.Handler {
	var (
		mu       sync.Mutex
		limiters = make(map[uuid.UUID]*rate.Limiter)
	)

	limiterFor := func(tenantID uuid.UUID) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()

		lim, ok := limiters[tenantID]
		if !ok {
			lim = rate.NewLimiter(rate.Limit(requestsPerSecond), burst)
			limiters[tenantID] = lim
		}
		return lim
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, ok := TenantIDFromContext(r.Context())
			if !ok {
				// No tenant in context; skip rate limiting.
				next.ServeHTTP(w, r)
				return
			}

			lim := limiterFor(tenantID)
			if !lim.Allow() {
				http.Error(w, `{"title":"Too Many Requests","status":429,"detail":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
