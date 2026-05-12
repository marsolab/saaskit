package config

import (
	"fmt"

	"github.com/heartwilltell/scotty"
)

// Config represents the configuration for the application.
type Config struct {
	Addr     string `flag:"addr" env:"ADDR" default:":8080" usage:"set HTTP listener address"`
	AddrGRPC string `flag:"addr-grpc" env:"ADDR_GRPC" default:":9090" usage:"set gRPC listener address"`

	LogLevel string `flag:"log-level" env:"LOG_LEVEL" default:"info" usage:"set log level"`
	LogJSON  bool   `flag:"log-json" env:"LOG_JSON" default:"false" usage:"set log format to JSON"`
	LogColor bool   `flag:"log-color" env:"LOG_COLOR" default:"true" usage:"set colorful log output. Ignored if -log-json=true"`

	// Kinde OAuth/OIDC parameters consumed by the authkinde service. Flattened
	// onto Config because scotty does not recurse into nested structs; Kinde()
	// re-groups them for handoff. When KindeDomain is empty the auth routes
	// are skipped so the binary boots without credentials in local dev.
	KindeDomain            string `flag:"kinde-domain" env:"KINDE_DOMAIN" usage:"Kinde issuer domain, e.g. https://acme.kinde.com"`
	KindeClientID          string `flag:"kinde-client-id" env:"KINDE_CLIENT_ID" usage:"Kinde OAuth client id"`
	KindeClientSecret      string `flag:"kinde-client-secret" env:"KINDE_CLIENT_SECRET" usage:"Kinde OAuth client secret"`
	KindeCallbackURL       string `flag:"kinde-callback-url" env:"KINDE_CALLBACK_URL" usage:"OAuth callback URL registered with Kinde"`
	KindeRedirectURL       string `flag:"kinde-redirect-url" env:"KINDE_REDIRECT_URL" usage:"frontend URL the callback redirects to after login"`
	KindeLogoutRedirectURL string `flag:"kinde-logout-redirect-url" env:"KINDE_LOGOUT_REDIRECT_URL" usage:"frontend URL Kinde returns to after logout"`
	KindeCookieDomain      string `flag:"kinde-cookie-domain" env:"KINDE_COOKIE_DOMAIN" usage:"cookie Domain attribute for session cookies; empty emits host-only cookies"`
}

// KindeConfig is the bundle the authkinde package consumes.
type KindeConfig struct {
	Domain            string
	ClientID          string
	ClientSecret      string
	CallbackURL       string
	RedirectURL       string
	LogoutRedirectURL string
	CookieDomain      string
}

// Kinde returns the Kinde configuration in the shape authkinde expects.
func (c *Config) Kinde() KindeConfig {
	return KindeConfig{
		Domain:            c.KindeDomain,
		ClientID:          c.KindeClientID,
		ClientSecret:      c.KindeClientSecret,
		CallbackURL:       c.KindeCallbackURL,
		RedirectURL:       c.KindeRedirectURL,
		LogoutRedirectURL: c.KindeLogoutRedirectURL,
		CookieDomain:      c.KindeCookieDomain,
	}
}

// Bind binds config to a command.
func Bind(cmd *scotty.Command) error {
	var c Config

	if err := cmd.BindConfig(&c); err != nil {
		return fmt.Errorf("bind config to a command %s: %w", cmd.Name, err)
	}

	return nil
}
