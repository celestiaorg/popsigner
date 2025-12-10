package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Bidon15/banhbaoring/control-plane/internal/database"
	apierrors "github.com/Bidon15/banhbaoring/control-plane/internal/pkg/errors"
	"github.com/Bidon15/banhbaoring/control-plane/internal/pkg/response"
)

// RateLimitConfig defines rate limiting parameters.
type RateLimitConfig struct {
	RequestsPerMinute int
	BurstSize         int
}

// DefaultRateLimitConfig returns default rate limiting configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         10,
	}
}

// RateLimit returns a rate limiting middleware using Redis.
func RateLimit(redis *database.Redis, cfg RateLimitConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client identifier (IP or API key)
			clientID := getClientID(r)
			key := fmt.Sprintf("ratelimit:%s", clientID)

			ctx := r.Context()
			windowDuration := time.Minute

			// Increment counter and get current value
			count, err := redis.IncrWithExpire(ctx, key, windowDuration)
			if err != nil {
				// On Redis error, allow the request but log the error
				next.ServeHTTP(w, r)
				return
			}

			limit := cfg.RequestsPerMinute
			remaining := limit - int(count)
			if remaining < 0 {
				remaining = 0
			}

			// Get TTL for reset time
			resetTime := time.Now().Add(windowDuration).Unix()

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))

			// Check if rate limit exceeded
			if int(count) > limit+cfg.BurstSize {
				w.Header().Set("Retry-After", strconv.Itoa(60))
				response.Error(w, apierrors.ErrRateLimited)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientID extracts a unique identifier for the client.
func getClientID(r *http.Request) string {
	// Check for API key first
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		// Use a hash prefix of the API key
		if len(apiKey) > 20 {
			return "apikey:" + apiKey[:20]
		}
		return "apikey:" + apiKey
	}

	// Fall back to IP address
	return "ip:" + getRealIP(r)
}

// getRealIP extracts the real client IP, considering proxies.
func getRealIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	// Check X-Real-IP header
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}

	return r.RemoteAddr
}

// RateLimitByKey returns a rate limiter that uses a custom key extractor.
func RateLimitByKey(redis *database.Redis, cfg RateLimitConfig, keyFunc func(*http.Request) string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := keyFunc(r)
			if clientID == "" {
				clientID = getClientID(r)
			}

			key := fmt.Sprintf("ratelimit:%s", clientID)
			ctx := r.Context()
			windowDuration := time.Minute

			count, err := redis.IncrWithExpire(ctx, key, windowDuration)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			limit := cfg.RequestsPerMinute
			remaining := limit - int(count)
			if remaining < 0 {
				remaining = 0
			}

			resetTime := time.Now().Add(windowDuration).Unix()

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))

			if int(count) > limit+cfg.BurstSize {
				w.Header().Set("Retry-After", strconv.Itoa(60))
				response.Error(w, apierrors.ErrRateLimited)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// OrgIDKey is the context key for organization ID.
	OrgIDKey contextKey = "org_id"
	// UserIDKey is the context key for user ID.
	UserIDKey contextKey = "user_id"
	// APIKeyIDKey is the context key for API key ID.
	APIKeyIDKey contextKey = "api_key_id"
)

// GetOrgID retrieves the organization ID from context.
func GetOrgID(ctx context.Context) string {
	if v := ctx.Value(OrgIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetUserID retrieves the user ID from context.
func GetUserID(ctx context.Context) string {
	if v := ctx.Value(UserIDKey); v != nil {
		return v.(string)
	}
	return ""
}

