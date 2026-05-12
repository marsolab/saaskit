package authkinde

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/marsolab/servekit/httpkit"
)

// Cookie names used across login/callback/logout/middleware. Kept exported so
// tests and reverse proxies can reference them.
const (
	CookieAccessToken   = "access_token"
	CookieRefreshToken  = "refresh_token"
	CookieAuthenticated = "authenticated"
)

// TransportOptions configures cookie behavior. CookieDomain defaults to ""
// which emits host-only cookies. Set it to a registrable suffix (e.g.
// ".example.com") when the frontend and API live on different subdomains.
type TransportOptions struct {
	CookieDomain string
}

// TransportHTTP exposes the Kinde OAuth HTTP endpoints and the authenticated
// /me handler. Mount it under the API's auth prefix.
type TransportHTTP struct {
	service *Service
	router  chi.Router
	logger  *slog.Logger
	opts    TransportOptions
}

// NewTransportHTTP constructs a TransportHTTP and wires its routes.
func NewTransportHTTP(service *Service, logger *slog.Logger, opts TransportOptions) *TransportHTTP {
	t := &TransportHTTP{
		service: service,
		router:  chi.NewRouter(),
		logger:  logger,
		opts:    opts,
	}

	t.router.Route("/kinde", func(kinde chi.Router) {
		kinde.Get("/login", t.loginHandler)
		kinde.Get("/signup", t.signupHandler)
		kinde.Get("/callback", t.callbackHandler)
		kinde.Get("/logout", t.logoutHandler)
		kinde.Post("/refresh", t.refreshHandler)
	})

	t.router.Group(func(gr chi.Router) {
		gr.Use(AuthenticationMiddleware(service))
		gr.Get("/me", t.meHandler)
	})

	return t
}

// ServeHTTP satisfies http.Handler so the transport can be mounted directly
// by httpkit.ListenerHTTP.Mount.
func (t *TransportHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.router.ServeHTTP(w, r)
}

func (t *TransportHTTP) loginHandler(w http.ResponseWriter, r *http.Request) {
	t.notImplemented(w, r, "login")
}

func (t *TransportHTTP) signupHandler(w http.ResponseWriter, r *http.Request) {
	t.notImplemented(w, r, "signup")
}

func (t *TransportHTTP) callbackHandler(w http.ResponseWriter, r *http.Request) {
	t.notImplemented(w, r, "callback")
}

func (t *TransportHTTP) logoutHandler(w http.ResponseWriter, r *http.Request) {
	t.notImplemented(w, r, "logout")
}

func (t *TransportHTTP) refreshHandler(w http.ResponseWriter, r *http.Request) {
	t.notImplemented(w, r, "refresh")
}

func (t *TransportHTTP) meHandler(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		httpkit.Status(w, r, http.StatusUnauthorized)
		return
	}

	httpkit.JSON(w, r, meResponse{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	})
}

func (t *TransportHTTP) notImplemented(w http.ResponseWriter, r *http.Request, handler string) {
	t.logger.WarnContext(r.Context(), "authkinde: handler not implemented", slog.String("handler", handler))
	httpkit.Status(w, r, http.StatusNotImplemented)
}

type meResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}
