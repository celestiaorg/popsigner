package middleware

import (
	"context"
	"net/http"
	"strings"

	apierrors "github.com/Bidon15/banhbaoring/control-plane/internal/pkg/errors"
	"github.com/Bidon15/banhbaoring/control-plane/internal/pkg/response"
)

// AuthConfig holds authentication middleware configuration.
type AuthConfig struct {
	// JWTSecret is the secret used to verify JWT tokens.
	JWTSecret string
	// SkipPaths are paths that don't require authentication.
	SkipPaths []string
}

// APIKeyValidator is a function that validates an API key and returns the org ID.
type APIKeyValidator func(ctx context.Context, apiKey string) (orgID string, scopes []string, err error)

// JWTValidator is a function that validates a JWT token and returns user info.
type JWTValidator func(ctx context.Context, token string) (userID, orgID string, err error)

// Auth returns an authentication middleware.
// This is a placeholder that will be fully implemented by the Auth agents.
func Auth(cfg AuthConfig, apiKeyValidator APIKeyValidator, jwtValidator JWTValidator) func(next http.Handler) http.Handler {
	skipPaths := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipPaths[path] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for certain paths
			if skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Check for API key authentication
			if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
				if apiKeyValidator == nil {
					response.Error(w, apierrors.ErrUnauthorized)
					return
				}

				orgID, scopes, err := apiKeyValidator(r.Context(), apiKey)
				if err != nil {
					response.Error(w, apierrors.ErrUnauthorized)
					return
				}

				// Add org ID and scopes to context
				ctx := context.WithValue(r.Context(), OrgIDKey, orgID)
				ctx = context.WithValue(ctx, contextKey("scopes"), scopes)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Check for Bearer token authentication
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")

				if jwtValidator == nil {
					response.Error(w, apierrors.ErrUnauthorized)
					return
				}

				userID, orgID, err := jwtValidator(r.Context(), token)
				if err != nil {
					response.Error(w, apierrors.ErrUnauthorized)
					return
				}

				// Add user and org to context
				ctx := context.WithValue(r.Context(), UserIDKey, userID)
				ctx = context.WithValue(ctx, OrgIDKey, orgID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// No authentication provided
			response.Error(w, apierrors.ErrUnauthorized)
		})
	}
}

// RequireScope returns a middleware that checks for required scopes.
func RequireScope(requiredScopes ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scopes, ok := r.Context().Value(contextKey("scopes")).([]string)
			if !ok {
				// No scopes means user auth (full access within org)
				next.ServeHTTP(w, r)
				return
			}

			// Check if any required scope is present
			scopeSet := make(map[string]bool)
			for _, s := range scopes {
				scopeSet[s] = true
			}

			for _, required := range requiredScopes {
				if scopeSet[required] {
					next.ServeHTTP(w, r)
					return
				}
			}

			response.Error(w, apierrors.ErrForbidden)
		})
	}
}

// OptionalAuth returns a middleware that attempts authentication but doesn't require it.
func OptionalAuth(cfg AuthConfig, apiKeyValidator APIKeyValidator, jwtValidator JWTValidator) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try API key authentication
			if apiKey := r.Header.Get("X-API-Key"); apiKey != "" && apiKeyValidator != nil {
				orgID, scopes, err := apiKeyValidator(r.Context(), apiKey)
				if err == nil {
					ctx := context.WithValue(r.Context(), OrgIDKey, orgID)
					ctx = context.WithValue(ctx, contextKey("scopes"), scopes)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// Try Bearer token authentication
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") && jwtValidator != nil {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				userID, orgID, err := jwtValidator(r.Context(), token)
				if err == nil {
					ctx := context.WithValue(r.Context(), UserIDKey, userID)
					ctx = context.WithValue(ctx, OrgIDKey, orgID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// Continue without authentication
			next.ServeHTTP(w, r)
		})
	}
}

