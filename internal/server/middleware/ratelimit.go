package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

type tenantLimiter struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

type ipLimiter struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// RateLimitByIP applies per-IP rate limiting for unauthenticated endpoints
// (e.g. auth routes). Uses chi's RealIP middleware value via r.RemoteAddr.
// Stale entries are cleaned up every 10 minutes.
func RateLimitByIP(ctx context.Context, requestsPerSecond float64, burst int) func(http.Handler) http.Handler {
	var (
		mu       sync.Mutex
		limiters = make(map[string]*ipLimiter)
	)

	// Background cleanup of stale limiters.
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				cutoff := time.Now().Add(-30 * time.Minute)
				for ip, il := range limiters {
					if il.lastAccess.Before(cutoff) {
						delete(limiters, ip)
					}
				}
				mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	limiterFor := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()

		il, ok := limiters[ip]
		if !ok {
			il = &ipLimiter{
				limiter:    rate.NewLimiter(rate.Limit(requestsPerSecond), burst),
				lastAccess: time.Now(),
			}
			limiters[ip] = il
		} else {
			il.lastAccess = time.Now()
		}
		return il.limiter
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			lim := limiterFor(ip)
			if !lim.Allow() {
				http.Error(w, `{"title":"Too Many Requests","status":429,"detail":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit applies per-tenant rate limiting. Stale limiter entries are cleaned
// up every 10 minutes to prevent unbounded memory growth.
func RateLimit(ctx context.Context, requestsPerSecond float64, burst int) func(http.Handler) http.Handler {
	var (
		mu       sync.Mutex
		limiters = make(map[uuid.UUID]*tenantLimiter)
	)

	// Background cleanup of stale limiters.
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				cutoff := time.Now().Add(-30 * time.Minute)
				for id, tl := range limiters {
					if tl.lastAccess.Before(cutoff) {
						delete(limiters, id)
					}
				}
				mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	limiterFor := func(tenantID uuid.UUID) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()

		tl, ok := limiters[tenantID]
		if !ok {
			tl = &tenantLimiter{
				limiter:    rate.NewLimiter(rate.Limit(requestsPerSecond), burst),
				lastAccess: time.Now(),
			}
			limiters[tenantID] = tl
		} else {
			tl.lastAccess = time.Now()
		}
		return tl.limiter
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
