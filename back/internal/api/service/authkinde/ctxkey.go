package authkinde

import "context"

// ctxKey is unexported so no other package can collide on the same key.
type ctxKey int

const userKey ctxKey = iota

// withUser returns a new context carrying u. AuthenticationMiddleware uses
// this to attach the resolved user to the request scope.
func withUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// UserFromContext returns the authenticated user attached to ctx, or nil if
// the request did not pass through AuthenticationMiddleware.
func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userKey).(*User) //nolint:errcheck,revive // comma-ok; nil is acceptable
	return u
}
