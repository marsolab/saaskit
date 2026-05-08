package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	neturl "net/url"
	"strings"
	"time"

	"github.com/marsolab/servekit/authkit/jwtkit"
	"github.com/marsolab/servekit/authkit/oauthkit/kinde"
	"github.com/marsolab/servekit/errkit"
	"github.com/marsolab/servekit/idkit"
	"github.com/proydov/sprints/back/internal/store"
	"github.com/proydov/sprints/back/internal/tracker"
	"golang.org/x/oauth2"
)

// Provider identifies the external identity provider we link local users to.
// Keep stable: the string is persisted in `external_identities.provider`.
const Provider = "kinde"

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
// redirect URL plus the CSRF-and-PKCE secrets that the callback must echo back.
type ExchangeURL struct {
	URL          string
	State        string
	CodeVerifier string
}

// Service glues together the Kinde OAuth2 config, the JWKS verifier, and the
// local user store. It has no HTTP concerns — transport.go owns those.
type Service struct {
	oauth             *oauth2.Config
	jwks              *jwtkit.JWKSProvider
	store             store.Store
	domain            string // issuer domain used to build the Kinde /logout URL.
	redirectURL       string // frontend URL the callback ultimately redirects to.
	logoutRedirectURL string // frontend URL Kinde returns to after ending the upstream session.
}

// NewService wires a Service. The Kinde client is built here to perform OIDC
// discovery once at startup (it blocks on an HTTP round-trip).
func NewService(st store.Store, domain, clientID, clientSecret, callbackURL, redirectURL, logoutRedirectURL string) (*Service, error) {
	if domain == "" {
		return nil, errors.New("auth: kinde domain is required")
	}

	if clientID == "" || clientSecret == "" {
		return nil, errors.New("auth: kinde client id/secret are required")
	}

	if callbackURL == "" || redirectURL == "" {
		return nil, errors.New("auth: kinde callback/redirect URLs are required")
	}

	lru := logoutRedirectURL
	if lru == "" {
		// Fall back to the same redirect we use after login; Kinde will bounce
		// the browser there after ending the SSO session. The dashboard still
		// has to whitelist it.
		lru = redirectURL
	}

	client, err := kinde.NewClient(domain, clientID, clientSecret)
	if err != nil {
		return nil, fmt.Errorf("auth: build kinde client: %w", err)
	}

	oidc := client.GetOIDCConfig()

	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  callbackURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  oidc.AuthorizationEndpoint,
			TokenURL: oidc.TokenEndpoint,
		},
		Scopes: []string{"openid", "profile", "email", "offline"},
	}

	return &Service{
		oauth:             cfg,
		jwks:              client.TokenVerifier(),
		store:             st,
		domain:            strings.TrimRight(domain, "/"),
		redirectURL:       redirectURL,
		logoutRedirectURL: lru,
	}, nil
}

// OAuthExchangeURL builds the Kinde authorize URL plus the short-lived state
// and PKCE verifier the transport layer must stash in cookies before redirecting.
func (s *Service) OAuthExchangeURL(kind ExchangeKind) (ExchangeURL, error) {
	state := idkit.ULID()

	verifier, err := newCodeVerifier()
	if err != nil {
		return ExchangeURL{}, fmt.Errorf("auth: generate code verifier: %w", err)
	}

	challenge := codeChallenge(verifier)

	url := s.oauth.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", string(kind)),
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	return ExchangeURL{URL: url, State: state, CodeVerifier: verifier}, nil
}

// ExchangeOAuthToken exchanges the authorization code for a signed access +
// refresh + id token bundle. The access token is verified against Kinde's JWKS
// before returning so a forged /oauth2/token response cannot slip through.
func (s *Service) ExchangeOAuthToken(ctx context.Context, code, codeVerifier string) (Session, error) {
	tok, err := s.oauth.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return Session{}, fmt.Errorf("auth: exchange oauth token: %w", err)
	}

	if err := s.jwks.Verify(tok.AccessToken); err != nil {
		return Session{}, fmt.Errorf("auth: verify access token: %w", err)
	}

	idToken, _ := tok.Extra("id_token").(string) //nolint:errcheck,revive // comma-ok; empty string is acceptable

	return Session{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		IDToken:      idToken,
		ExpiresAt:    tok.Expiry,
	}, nil
}

// RefreshSession redeems the refresh token for a fresh access (and possibly
// rotated refresh) token. Kinde rotates refresh tokens by default, so the
// caller must persist whatever `Session.RefreshToken` comes back with — if the
// upstream did not rotate, oauth2 preserves the prior value.
//
// Returned errors wrap `errkit.ErrUnauthenticated` when the refresh token is
// rejected (expired / revoked / malformed) so the transport layer can map to
// 401 and clear cookies uniformly.
func (s *Service) RefreshSession(ctx context.Context, refreshToken string) (Session, error) {
	if refreshToken == "" {
		return Session{}, fmt.Errorf("%w: no refresh token", errkit.ErrUnauthenticated)
	}

	// TokenSource does a single-shot refresh when given a token with only a
	// RefreshToken populated: it issues the RFC6749 refresh_token grant and
	// returns the response. We don't cache the TokenSource — the caller owns
	// token lifetime via cookies.
	src := s.oauth.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})

	tok, err := src.Token()
	if err != nil {
		return Session{}, fmt.Errorf("%w: refresh oauth token: %w", errkit.ErrUnauthenticated, err)
	}

	if err := s.jwks.Verify(tok.AccessToken); err != nil {
		return Session{}, fmt.Errorf("%w: verify refreshed token: %w", errkit.ErrUnauthenticated, err)
	}

	// Keep the old refresh token if Kinde did not rotate — oauth2 already
	// preserves it, but guard against driver-level inconsistencies.
	rt := tok.RefreshToken
	if rt == "" {
		rt = refreshToken
	}

	idToken, _ := tok.Extra("id_token").(string) //nolint:errcheck,revive // comma-ok; empty string is acceptable

	return Session{
		AccessToken:  tok.AccessToken,
		RefreshToken: rt,
		IDToken:      idToken,
		ExpiresAt:    tok.Expiry,
	}, nil
}

// GetUserFromToken validates the access token and returns the local user.
// Returns ErrUnauthenticated if the token is invalid / expired, or if no local
// user has been provisioned for the token subject yet.
func (s *Service) GetUserFromToken(ctx context.Context, accessToken string) (*tracker.User, error) {
	var claims kinde.AccessTokenClaims
	if err := s.jwks.ParseVerifyClaims(accessToken, &claims); err != nil {
		return nil, fmt.Errorf("%w: parse access token: %w", errkit.ErrUnauthenticated, err)
	}

	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: token has no subject", errkit.ErrUnauthenticated)
	}

	user, err := s.store.GetUserByExternalIdentity(ctx, Provider, claims.Subject)
	if err != nil {
		if errors.Is(err, errkit.ErrNotFound) {
			return nil, fmt.Errorf("%w: local user not provisioned", errkit.ErrUnauthenticated)
		}

		return nil, fmt.Errorf("auth: lookup user: %w", err)
	}

	return &user, nil
}

// ProvisionUser is the canonical "find-or-create" used by the callback and by
// Kinde user-created webhooks. Callers pass in the identity profile claimed in
// the ID token. The store's CreateUserWithIdentity owns the transactional
// semantics (link if email matches, reject if identity already linked).
func (s *Service) ProvisionUser(ctx context.Context, profile tracker.ExternalIdentityProfile) (tracker.User, error) {
	// Fast path: already linked, return existing.
	if existing, err := s.store.GetUserByExternalIdentity(ctx, profile.Provider, profile.ProviderSub); err == nil {
		return existing, nil
	} else if !errors.Is(err, errkit.ErrNotFound) {
		return tracker.User{}, fmt.Errorf("auth: provision: lookup identity: %w", err)
	}

	// Link-or-create path.
	created, err := s.store.CreateUserWithIdentity(ctx, profile)
	if err != nil {
		if errors.Is(err, errkit.ErrAlreadyExists) {
			// Raced against a parallel provision for the same identity — the
			// link is now present; re-read and return it.
			if existing, lookupErr := s.store.GetUserByExternalIdentity(ctx, profile.Provider, profile.ProviderSub); lookupErr == nil {
				return existing, nil
			}
		}

		return tracker.User{}, fmt.Errorf("auth: provision: create user: %w", err)
	}

	return created, nil
}

// RedirectURL returns the frontend URL the callback redirects to after setting
// session cookies.
func (s *Service) RedirectURL() string { return s.redirectURL }

// LogoutURL returns the Kinde-hosted logout endpoint with our configured
// post-logout redirect preloaded. Sending the browser here ends the SSO
// session server-side: the upstream session cookie at kinde.com gets cleared,
// so the next /kinde/login cannot silently mint a fresh token without a
// password prompt. The post-logout redirect must be whitelisted in the Kinde
// dashboard, or Kinde will reject the redirect param and land the user on
// its own page.
func (s *Service) LogoutURL() string { return s.LogoutURLFor(s.logoutRedirectURL) }

// LogoutURLFor is LogoutURL with an explicit post-logout redirect, used by
// the transport when a caller overrides the destination via ?redirect_url=.
// The redirect must still be on the dashboard's whitelist; we don't validate
// that here because the failure mode is visible (Kinde rejects the param).
func (s *Service) LogoutURLFor(redirect string) string {
	q := neturl.Values{}
	q.Set("redirect", redirect)

	return s.domain + "/logout?" + q.Encode()
}

// ParseIDTokenClaims pulls the Kinde ID-token claims (email, given_name, ...)
// out of an unverified payload. The access token has already been verified by
// the time this runs — the id_token is issued alongside it by Kinde as part of
// the same exchange, so re-verification would be belt-and-braces. It is still
// JWKS-verified here for defense-in-depth when a webhook supplies the token.
func (s *Service) ParseIDTokenClaims(idToken string) (*kinde.IDTokenClaims, error) {
	var claims kinde.IDTokenClaims
	if err := s.jwks.ParseVerifyClaims(idToken, &claims); err != nil {
		return nil, fmt.Errorf("auth: parse id token: %w", err)
	}

	return &claims, nil
}

// --- PKCE helpers ---.

// codeVerifierByteLength is the number of random bytes to encode as a code verifier.
// When base64url-encoded, 48 bytes yields a 64-character string, which sits
// comfortably inside RFC 7636 §4.1's 43–128-char range.
const codeVerifierByteLength = 48

// newCodeVerifier returns a base64url-encoded random string suitable for PKCE.
func newCodeVerifier() (string, error) {
	b := make([]byte, codeVerifierByteLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate code verifier: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// codeChallenge computes the S256 challenge for a given verifier.
func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
