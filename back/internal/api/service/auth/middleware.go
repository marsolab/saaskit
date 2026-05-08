package auth

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/marsolab/servekit/errkit"
	"github.com/marsolab/servekit/httpkit"
)

// AuthenticationMiddleware reads the access_token cookie, validates it against
// Kinde's JWKS, resolves the local user, and attaches the user to the request
// context. Protected routes should sit inside a Group that uses this.
//
// Note: no refresh-token rotation yet — an expired token returns 401, and the
// frontend has to kick off a new /kinde/login. Wiring a /refresh endpoint is a
// follow-up; until then, Kinde's hosted session cookies keep the login silent
// on the second visit.
func (s *Service) AuthenticationMiddleware() httpkit.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieAccessToken)
			if err != nil {
				httpkit.ErrorHTTP(w, r, fmt.Errorf("%w: access token cookie missing: %w", errkit.ErrUnauthenticated, err))
				return
			}

			user, err := s.GetUserFromToken(r.Context(), cookie.Value)
			if err != nil {
				// GetUserFromToken already wraps with ErrUnauthenticated on the
				// token-side failure; wrap once for store-side failures too so
				// httpkit.ErrorHTTP maps everything to 401.
				if !errors.Is(err, errkit.ErrUnauthenticated) {
					err = fmt.Errorf("%w: %w", errkit.ErrUnauthenticated, err)
				}

				httpkit.ErrorHTTP(w, r, err)

				return
			}

			ctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
