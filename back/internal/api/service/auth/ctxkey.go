// Package auth provides the Kinde-backed authentication stack for the tracker
// service. It wraps servekit/authkit/oauthkit/kinde for OIDC discovery and
// JWKS verification, uses golang.org/x/oauth2 for the PKCE code-exchange
// dance, and stores session tokens in HttpOnly cookies. Local user records
// live in store.Store; the cookie's access token carries the Kinde subject
// which the middleware resolves to the local user ID on every request.
package auth

import (
	"context"

	"github.com/proydov/sprints/back/internal/tracker"
)

// ctxKey is an unexported type so no other package can collide.
type ctxKey int

const (
	userKey ctxKey = iota
)

// WithUser returns a new context carrying u. Used by AuthenticationMiddleware.
func WithUser(ctx context.Context, u *tracker.User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// User returns the authenticated user attached to ctx, or nil if none.
// Handlers should use UserID unless they need email/name too.
func User(ctx context.Context) *tracker.User {
	u, _ := ctx.Value(userKey).(*tracker.User) //nolint:errcheck,revive // comma-ok; nil is acceptable
	return u
}

// UserID returns the authenticated user's local ULID, or "" if the context
// has no user (e.g. a bug where a handler runs outside AuthenticationMiddleware).
// Store calls pass this value as the userID argument; an empty string lets
// stores reject the call with ErrUnauthenticated before touching SQL.
func UserID(ctx context.Context) string {
	if u := User(ctx); u != nil {
		return u.ID
	}

	return ""
}
