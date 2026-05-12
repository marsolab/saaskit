package authkinde

import (
	"errors"
	"net/http"

	"github.com/marsolab/servekit/httpkit"
)

// AuthenticationMiddleware reads the access_token cookie, validates it
// against Kinde's JWKS via Service.GetUserFromToken, and attaches the
// resolved User to the request context. Routes that sit inside a Group using
// this middleware can read the user via UserFromContext.
//
// While the service skeleton returns ErrNotImplemented, the middleware
// responds with 401 — that is the same status the eventual implementation
// will use for an invalid token, so callers can wire against it now.
func AuthenticationMiddleware(service *Service) httpkit.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieAccessToken)
			if err != nil {
				httpkit.Status(w, r, http.StatusUnauthorized)
				return
			}

			user, err := service.GetUserFromToken(r.Context(), cookie.Value)
			if err != nil {
				if !errors.Is(err, ErrNotImplemented) {
					httpkit.Status(w, r, http.StatusUnauthorized)
					return
				}

				httpkit.Status(w, r, http.StatusNotImplemented)

				return
			}

			next.ServeHTTP(w, r.WithContext(withUser(r.Context(), user)))
		})
	}
}
