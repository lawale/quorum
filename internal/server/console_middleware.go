package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/service"
)

type ctxKey string

const operatorIDCtxKey ctxKey = "operator_id"

// consoleJWTMiddleware validates JWT tokens issued by the OperatorService and
// sets the auth identity context so downstream handlers (policies, webhooks, etc.)
// see the operator as the authenticated user.
func consoleJWTMiddleware(operatorService *service.OperatorService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeError(w, http.StatusUnauthorized, "invalid authorization header format")
				return
			}

			claims, err := operatorService.ValidateToken(parts[1])
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			operatorID, err := uuid.Parse(claims.Subject)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid token subject")
				return
			}

			// Set auth identity so existing handlers see the operator's username
			ctx := auth.WithIdentity(r.Context(), &auth.Identity{
				UserID: claims.Username,
			})

			// Store operator UUID for console-specific handlers
			ctx = context.WithValue(ctx, operatorIDCtxKey, operatorID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// operatorIDFromContext extracts the operator UUID set by the JWT middleware.
func operatorIDFromContext(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(operatorIDCtxKey).(uuid.UUID)
	return id
}
