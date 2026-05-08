package auth

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/marsolab/servekit/errkit"
	"github.com/marsolab/servekit/httpkit"

	"github.com/proydov/sprints/back/internal/tracker"
)

// Cookie names used across login/callback/logout/middleware.
const (
	CookieAccessToken   = "access_token"
	CookieRefreshToken  = "refresh_token"
	CookieAuthenticated = "authenticated"
	cookieOAuthState    = "oauth_state"
	cookieOAuthVerifier = "oauth_code_verifier"

	oauthCookieTTL = 10 * time.Minute // round-trip to Kinde is seconds; 10m is ample.
)

// TransportOptions configures the cookie behavior. CookieDomain defaults to
// "" which emits host-only cookies. When the frontend and API live on
// different registrable domains (or subdomains of one), set CookieDomain to
// the registrable suffix (e.g. ".sdvg.io") so the browser attaches cookies on
// the frontend host.
type TransportOptions struct {
	CookieDomain string
}

// Transport exposes the Kinde OAuth HTTP endpoints and the authenticated
// /me handler. It is mounted by api.BuildRoutes under /v1/auth.
type Transport struct {
	service *Service
	logger  *slog.Logger
	opts    TransportOptions
}

// NewTransport constructs a Transport.
func NewTransport(service *Service, logger *slog.Logger, opts TransportOptions) *Transport {
	return &Transport{service: service, logger: logger, opts: opts}
}

// Mount registers the /kinde/* public routes and the protected /me route on
// the provided router. Protected routes require the caller to run this under
// a group that installs AuthenticationMiddleware; we keep Mount passive so the
// caller can compose public vs protected segments as it sees fit.
//
// /kinde/refresh is public by design: the refresh-token cookie *is* the
// credential, and the caller is the browser whose access token just expired.
// Requiring AuthMiddleware here would deadlock: the expired access token
// would be rejected before we ever read the refresh cookie.
func (t *Transport) Mount(r chi.Router) {
	r.Route("/kinde", func(kinde chi.Router) {
		kinde.Get("/login", t.loginHandler)
		kinde.Get("/signup", t.signupHandler)
		kinde.Get("/callback", t.callbackHandler)
		kinde.Get("/logout", t.logoutHandler)
		kinde.Post("/refresh", t.refreshHandler)
	})

	// /me is mounted inside the authenticated group by the caller.
	r.Group(func(gr chi.Router) {
		gr.Use(t.service.AuthenticationMiddleware())
		gr.Get("/me", t.meHandler)
	})
}

// loginHandler sends the browser to Kinde's hosted login page.
func (t *Transport) loginHandler(w http.ResponseWriter, r *http.Request) {
	t.initOAuthFlow(w, r, ExchangeLogin)
}

// signupHandler sends the browser to Kinde's hosted sign-up page.
func (t *Transport) signupHandler(w http.ResponseWriter, r *http.Request) {
	t.initOAuthFlow(w, r, ExchangeSignup)
}

func (t *Transport) initOAuthFlow(w http.ResponseWriter, r *http.Request, kind ExchangeKind) {
	ex, err := t.service.OAuthExchangeURL(kind)
	if err != nil {
		httpkit.ErrorHTTP(w, r, err)
		return
	}

	t.setShortCookie(w, r, cookieOAuthState, ex.State, oauthCookieTTL)
	t.setShortCookie(w, r, cookieOAuthVerifier, ex.CodeVerifier, oauthCookieTTL)

	http.Redirect(w, r, ex.URL, http.StatusTemporaryRedirect)
}

// callbackHandler is the Kinde redirect target. It verifies state, exchanges
// the authorization code, provisions the local user from ID-token claims,
// then sets the session cookies and redirects to the frontend.
func (t *Transport) callbackHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	if errCode := q.Get("error"); errCode != "" {
		t.logger.WarnContext(r.Context(), "kinde callback error",
			slog.String("error", errCode),
			slog.String("description", q.Get("error_description")),
		)
		httpkit.ErrorHTTP(w, r, fmt.Errorf("%w: kinde: %s", errkit.ErrUnauthenticated, errCode))

		return
	}

	code := q.Get("code")
	if code == "" {
		httpkit.ErrorHTTP(w, r, fmt.Errorf("%w: missing code", errkit.ErrInvalidArgument))
		return
	}

	// State check. Kinde sometimes substitutes its own base64-JSON state (e.g.
	// multi-org flows); only enforce CSRF match when the state looks like the
	// ULID we handed out.
	queryState := q.Get("state")
	if !looksLikeKindeState(queryState) {
		stateCookie, err := r.Cookie(cookieOAuthState)
		if err != nil || stateCookie.Value == "" {
			httpkit.ErrorHTTP(w, r, fmt.Errorf("%w: missing state cookie", errkit.ErrUnauthenticated))
			return
		}

		if stateCookie.Value != queryState {
			httpkit.ErrorHTTP(w, r, fmt.Errorf("%w: state mismatch", errkit.ErrUnauthenticated))
			return
		}
	}

	t.clearCookie(w, cookieOAuthState)

	verifierCookie, err := r.Cookie(cookieOAuthVerifier)
	if err != nil || verifierCookie.Value == "" {
		httpkit.ErrorHTTP(w, r, fmt.Errorf("%w: missing code_verifier cookie", errkit.ErrUnauthenticated))
		return
	}

	t.clearCookie(w, cookieOAuthVerifier)

	session, err := t.service.ExchangeOAuthToken(r.Context(), code, verifierCookie.Value)
	if err != nil {
		httpkit.ErrorHTTP(w, r, fmt.Errorf("%w: %w", errkit.ErrUnauthenticated, err))
		return
	}

	// Provision (or link) the local user from the ID token claims. We do this
	// here so the first protected request always finds a local row — the
	// webhook path (if wired later) is defense-in-depth.
	if session.IDToken != "" {
		claims, parseErr := t.service.ParseIDTokenClaims(session.IDToken)
		if parseErr != nil {
			t.logger.WarnContext(r.Context(), "parse id token", slog.String("err", parseErr.Error()))
		} else {
			profile := tracker.ExternalIdentityProfile{
				Provider:    Provider,
				ProviderSub: claims.Subject,
				Email:       claims.Email,
				FirstName:   claims.GivenName,
				LastName:    claims.FamilyName,
			}
			if _, provErr := t.service.ProvisionUser(r.Context(), profile); provErr != nil {
				t.logger.ErrorContext(r.Context(), "provision user", slog.String("err", provErr.Error()))
				httpkit.ErrorHTTP(w, r, fmt.Errorf("auth: provision user: %w", provErr))

				return
			}
		}
	}

	t.setSessionCookies(w, r, session)

	http.Redirect(w, r, t.service.RedirectURL(), http.StatusTemporaryRedirect)
}

// logoutHandler clears the local session cookies and bounces the browser to
// Kinde's hosted /logout endpoint so the upstream SSO session is destroyed
// too. Without the Kinde hop, a logged-out user hitting /kinde/login again
// would silently re-authenticate from the leftover Kinde cookie — which is
// the bug "a leaked access token stays valid until exp" turns into from the
// user's perspective.
//
// After Kinde clears its session it redirects to the logoutRedirectURL we
// registered with the dashboard. The `redirect_url` query param on *our*
// endpoint is still honored: callers can ask to land somewhere other than
// the configured default (useful for e2e tests or post-logout flash states).
func (t *Transport) logoutHandler(w http.ResponseWriter, r *http.Request) {
	t.clearCookie(w, CookieAccessToken)
	t.clearCookie(w, CookieRefreshToken)
	t.clearCookie(w, CookieAuthenticated)

	target := t.service.LogoutURL()

	if override := r.URL.Query().Get("redirect_url"); override != "" {
		// Only honor overrides that resolve to a valid URL. If the param is
		// malformed we silently fall back to the configured Kinde logout URL.
		if u, err := neturl.Parse(override); err == nil && u.Scheme != "" && u.Host != "" {
			target = t.service.LogoutURLFor(override)
		}
	}

	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}

// refreshHandler redeems the refresh_token cookie for a new access token
// and rotates both cookies in place. Returns 204 on success, 401 on failure
// (and clears session cookies so the frontend falls back to anonymous).
//
// Semantics: `POST /v1/auth/kinde/refresh`, no body. The browser is already
// carrying `refresh_token` in a HttpOnly cookie; JS never sees it, and the
// request must include `credentials: 'include'`.
func (t *Transport) refreshHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(CookieRefreshToken)
	if err != nil || cookie.Value == "" {
		t.clearCookie(w, CookieAccessToken)
		t.clearCookie(w, CookieRefreshToken)
		t.clearCookie(w, CookieAuthenticated)
		httpkit.ErrorHTTP(w, r, fmt.Errorf("%w: no refresh token cookie", errkit.ErrUnauthenticated))

		return
	}

	session, err := t.service.RefreshSession(r.Context(), cookie.Value)
	if err != nil {
		// Refresh failed — refresh token revoked, expired, or Kinde outage.
		// Either way the browser's session is over. Clear cookies so the
		// frontend stops pretending to be logged in.
		t.clearCookie(w, CookieAccessToken)
		t.clearCookie(w, CookieRefreshToken)
		t.clearCookie(w, CookieAuthenticated)
		t.logger.WarnContext(r.Context(), "refresh session failed", slog.String("err", err.Error()))
		httpkit.ErrorHTTP(w, r, err)

		return
	}

	t.setSessionCookies(w, r, session)
	httpkit.Status(w, r, http.StatusNoContent)
}

// meHandler returns the authenticated user attached by AuthenticationMiddleware.
func (*Transport) meHandler(w http.ResponseWriter, r *http.Request) {
	user := User(r.Context())
	if user == nil {
		httpkit.ErrorHTTP(w, r, errkit.ErrUnauthenticated)
		return
	}

	httpkit.JSON(w, r, meResponse{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	})
}

type meResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// --- cookie helpers ---.

func (t *Transport) setShortCookie(w http.ResponseWriter, r *http.Request, name, value string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   t.opts.CookieDomain,
		HttpOnly: true,
		Secure:   isSecure(r),
		SameSite: sameSiteFor(r),
		Expires:  time.Now().Add(ttl),
	})
}

func (t *Transport) setSessionCookies(w http.ResponseWriter, r *http.Request, session Session) {
	secure := isSecure(r)
	sameSite := sameSiteFor(r)

	http.SetCookie(w, &http.Cookie{
		Name:     CookieAccessToken,
		Value:    session.AccessToken,
		Path:     "/",
		Domain:   t.opts.CookieDomain,
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})

	if session.RefreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     CookieRefreshToken,
			Value:    session.RefreshToken,
			Path:     "/",
			Domain:   t.opts.CookieDomain,
			HttpOnly: true,
			Secure:   secure,
			SameSite: sameSite,
		})
	}

	// authenticated is a non-sensitive hint for the frontend's Astro middleware
	// to make the "am I logged in?" decision without touching /me on each page
	// load. It is NOT used for authorization — only redirect gating.
	http.SetCookie(w, &http.Cookie{
		Name:     CookieAuthenticated,
		Value:    "true",
		Path:     "/",
		Domain:   t.opts.CookieDomain,
		Expires:  session.ExpiresAt,
		HttpOnly: false,
		Secure:   secure,
		SameSite: sameSite,
	})
}

func (t *Transport) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:   name,
		Value:  "",
		Path:   "/",
		Domain: t.opts.CookieDomain,
		MaxAge: -1,
	})
}

// isSecure returns true when the client reached us via HTTPS (directly or
// through a TLS-terminating proxy). We trust X-Forwarded-Proto because the
// Kinde callback path is only reached after a top-level redirect the browser
// initiated — an attacker cannot forge the header on behalf of the user.
func isSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}

	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// sameSiteFor returns SameSiteNoneMode when the request is HTTPS (so cookies
// can ride cross-origin fetches from the frontend), otherwise Lax. Modern
// browsers reject `SameSite=None` without `Secure`, and localhost dev is
// typically same-origin via a vite proxy, so Lax is the right default there.
func sameSiteFor(r *http.Request) http.SameSite {
	if isSecure(r) {
		return http.SameSiteNoneMode
	}

	return http.SameSiteLaxMode
}

// looksLikeKindeState detects when Kinde substituted our state with its own
// base64-encoded JSON payload (e.g. in org-selection flows). If so, we skip
// the CSRF check and lean on PKCE's code_verifier — Kinde cannot have
// manufactured a code exchange without our verifier.
func looksLikeKindeState(s string) bool {
	if s == "" {
		return false
	}

	decoded, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return false
	}

	return len(decoded) > 0 && decoded[0] == '{'
}
