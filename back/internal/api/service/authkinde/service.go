// Package authkinde provides the Kinde-backed authentication stack: HTTP
// transport for login/callback/logout/refresh, middleware that resolves the
// access-token cookie into a User on each request, and a Service that owns
// the OAuth2 + JWKS plumbing.
//
// The package currently ships as a skeleton: handlers and methods are wired
// but return ErrNotImplemented until the storage layer for local users lands.
package authkinde

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/marsolab/servekit/authkit/jwtkit"
	"github.com/marsolab/servekit/authkit/oauthkit/kinde"
	"golang.org/x/oauth2"

	"github.com/marsolab/saaskit/back/internal/config"
)

// Provider identifies the external identity provider that issues the tokens
// we validate. Persisted alongside the provider subject when a local user
// record is linked to a Kinde identity.
const Provider = "kinde"

// ErrNotImplemented is returned by skeleton methods that have not been wired
// to a storage backend yet.
var ErrNotImplemented = errors.New("authkinde: not implemented")

// ExchangeKind signals whether the authorize URL should push Kinde's login or
// signup screen. It maps to the `prompt` parameter Kinde interprets.
type ExchangeKind string

const (
	// ExchangeLogin prompts the Kinde login screen.
	ExchangeLogin ExchangeKind = "login"

	// ExchangeSignup prompts the Kinde signup screen.
	ExchangeSignup ExchangeKind = "create"
)

// Session is the token bundle returned by a successful code exchange.
type Session struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	ExpiresAt    time.Time
}

// ExchangeURL is the payload the transport layer hands off to the browser: a
// redirect URL plus the CSRF-and-PKCE secrets the callback must echo back.
type ExchangeURL struct {
	URL          string
	State        string
	CodeVerifier string
}

// User is the minimal local user the middleware attaches to the request
// context. The skeleton keeps the type local so the package compiles
// standalone; a real user model can replace this once storage lands.
type User struct {
	ID        string
	Email     string
	FirstName string
	LastName  string
}

// Service wires the Kinde OAuth2 config and JWKS verifier together. Handlers
// in transport.go go through Service for every operation that touches Kinde.
type Service struct {
	oauth             *oauth2.Config
	jwks              *jwtkit.JWKSProvider
	domain            string
	redirectURL       string
	logoutRedirectURL string
}

// Config holds the parameters NewService needs. config.Config.Kinde()
// produces an instance of this type from CLI flags / env vars.
type Config = config.KindeConfig

// NewService wires a Service. The Kinde client is built here so OIDC
// discovery (an HTTP round-trip) runs once at startup.
func NewService(cfg Config) (*Service, error) {
	if cfg.Domain == "" {
		return nil, errors.New("authkinde: domain is required")
	}

	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("authkinde: client id/secret are required")
	}

	if cfg.CallbackURL == "" || cfg.RedirectURL == "" {
		return nil, errors.New("authkinde: callback/redirect URLs are required")
	}

	logoutRedirect := cfg.LogoutRedirectURL
	if logoutRedirect == "" {
		logoutRedirect = cfg.RedirectURL
	}

	client, err := kinde.NewClient(cfg.Domain, cfg.ClientID, cfg.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("authkinde: build kinde client: %w", err)
	}

	oidc := client.GetOIDCConfig()

	return &Service{
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.CallbackURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  oidc.AuthorizationEndpoint,
				TokenURL: oidc.TokenEndpoint,
			},
			Scopes: []string{"openid", "profile", "email", "offline"},
		},
		jwks:              client.TokenVerifier(),
		domain:            cfg.Domain,
		redirectURL:       cfg.RedirectURL,
		logoutRedirectURL: logoutRedirect,
	}, nil
}

// OAuthExchangeURL will build the Kinde authorize URL plus the state and PKCE
// verifier the transport must stash in cookies before redirecting.
func (s *Service) OAuthExchangeURL(_ ExchangeKind) (ExchangeURL, error) {
	return ExchangeURL{}, ErrNotImplemented
}

// ExchangeOAuthToken will exchange the authorization code for an access +
// refresh + id token bundle, verifying the access token against Kinde's JWKS.
func (s *Service) ExchangeOAuthToken(_ context.Context, _, _ string) (Session, error) {
	return Session{}, ErrNotImplemented
}

// RefreshSession will redeem the refresh token for a fresh access token.
func (s *Service) RefreshSession(_ context.Context, _ string) (Session, error) {
	return Session{}, ErrNotImplemented
}

// GetUserFromToken will validate the access token and return the local user.
func (s *Service) GetUserFromToken(_ context.Context, _ string) (*User, error) {
	return nil, ErrNotImplemented
}

// RedirectURL returns the frontend URL the callback redirects to after
// setting session cookies.
func (s *Service) RedirectURL() string { return s.redirectURL }

// LogoutURL returns the Kinde-hosted logout endpoint preloaded with the
// configured post-logout redirect.
func (s *Service) LogoutURL() string { return s.logoutRedirectURL }
