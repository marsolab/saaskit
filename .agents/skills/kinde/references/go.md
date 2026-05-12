# Kinde Go reference

Official SDK: `github.com/kinde-oss/kinde-go` (Go 1.24+).

```bash
go get github.com/kinde-oss/kinde-go
```

Three packages you'll use:

| Package | Use for |
|---|---|
| `github.com/kinde-oss/kinde-go/oauth2/authorization_code` | Browser auth code flow (with optional PKCE) and device authorization flow |
| `github.com/kinde-oss/kinde-go/oauth2/client_credentials` | Machine-to-machine (M2M) |
| `github.com/kinde-oss/kinde-go/jwt` | Parse and validate JWTs from headers, strings, sessions, or OAuth2 tokens |

There is a `cli` subpackage under `client_credentials` for
file-backed session storage in CLI tools:
`github.com/kinde-oss/kinde-go/oauth2/client_credentials/cli`.

## Session hooks (ISessionHooks)

All flows persist tokens through an `ISessionHooks` you implement.
Conceptually:

```go
type ISessionHooks interface {
    Save(ctx context.Context, key string, value string) error
    Load(ctx context.Context, key string) (string, error)
    Delete(ctx context.Context, key string) error
}
```

For a web app, back this with encrypted cookies or a server-side
session store. For a CLI, the bundled `cli.NewCliSession("app-name")`
writes to the user's config dir.

## Authorization code flow (server-rendered web)

### Configure

```go
import (
    "github.com/kinde-oss/kinde-go/oauth2/authorization_code"
    "github.com/kinde-oss/kinde-go/jwt"
)

flow, err := authorization_code.NewAuthorizationCodeFlow(
    issuerURL,          // e.g. "https://your-tenant.kinde.com"
    clientID,
    clientSecret,
    callbackURL,        // must match an allowed callback in dashboard
    authorization_code.WithSessionHooks(sessions),
    authorization_code.WithOffline(),                 // ask for refresh token
    authorization_code.WithAudience("api.example.com"),
    authorization_code.WithTokenValidation(
        true,                                          // validate signature
        jwt.WillValidateAlgorithm(),                   // pin algs
        jwt.WillValidateAudience("api.example.com"),
        jwt.WillValidateIssuer(issuerURL),
    ),
)
```

Available options:

- `WithSessionHooks(hooks)` — required for stateful flows.
- `WithOffline()` — requests the `offline` scope so a refresh token
  is issued.
- `WithAudience(aud)` — sets the JWT `aud` claim on access tokens.
- `WithPKCE()` — adds PKCE on top of the auth-code flow. Recommended
  even for confidential clients.
- `WithPrompt("login" | "create" | "none")` — forces the hosted
  login behaviour.
- `WithTokenValidation(verifySignature bool, opts ...jwt.Option)` —
  see the JWT section.

### Wire up the HTTP routes

```go
// GET /login
http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
    url, err := flow.GetAuthURL(r.Context())
    if err != nil { http.Error(w, err.Error(), 500); return }
    http.Redirect(w, r, url, http.StatusFound)
})

// GET /callback
http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
    code  := r.URL.Query().Get("code")
    state := r.URL.Query().Get("state")
    if err := flow.ExchangeCode(r.Context(), code, state); err != nil {
        http.Error(w, err.Error(), 401); return
    }
    http.Redirect(w, r, "/", http.StatusFound)
})
```

`ExchangeCode` does CSRF state validation, token exchange, JWT
validation (if enabled), and stores tokens via the session hook.

### Make authenticated calls

`GetClient` returns an `*http.Client` that injects the access token
and refreshes it automatically when `WithOffline()` is set.

```go
client, err := flow.GetClient(r.Context())
resp, err := client.Get("https://api.example.com/me")
```

To read the user's profile via the OIDC userinfo endpoint:

```go
resp, _ := client.Get(issuerURL + "/oauth2/user_profile")
```

### PKCE

For confidential clients PKCE is optional but recommended; for
public clients it's required. Add `WithPKCE()` to the options — the
SDK handles verifier/challenge generation and storage via the
session hooks.

## Device authorization flow (CLI)

For tools without a browser handy. The user is shown a short URL
and a code; your CLI polls until they confirm.

```go
deviceFlow, err := authorization_code.NewDeviceAuthorizationFlow(
    issuerURL,
    authorization_code.WithClientID(clientID),
    authorization_code.WithClientSecret(clientSecret), // optional for public clients
    authorization_code.WithSessionHooks(cli.NewCliSession("myapp")),
    authorization_code.WithOffline(),
    authorization_code.WithTokenValidation(true, jwt.WillValidateAlgorithm()),
)

da, err := deviceFlow.StartDeviceAuth(ctx)
// Print: "Visit", da.VerificationURI, "and enter", da.UserCode
if err := deviceFlow.ExchangeDeviceAccessToken(ctx, da); err != nil {
    log.Fatal(err)
}

// Subsequent calls reuse the persisted token:
tok, _ := deviceFlow.GetToken(ctx)
```

`InjectTokenMiddleware()` wraps an outbound `http.RoundTripper` to
add the access token automatically.

## Client credentials flow (M2M)

```go
import (
    "github.com/kinde-oss/kinde-go/oauth2/client_credentials"
    "github.com/kinde-oss/kinde-go/jwt"
)

m2m, err := client_credentials.NewClientCredentialsFlow(
    issuerURL,
    clientID, clientSecret,
    client_credentials.WithAudience("https://your-tenant.kinde.com/api"),
    client_credentials.WithScopes(), // pass scopes if your audience requires them
    client_credentials.WithSessionHooks(sessions),
    client_credentials.WithTokenValidation(
        true,
        jwt.WillValidateAlgorithm(),
        jwt.WillValidateAudience("https://your-tenant.kinde.com/api"),
    ),
)

client, _ := m2m.GetClient(ctx)        // *http.Client with bearer token
resp, _   := client.Get("https://your-tenant.kinde.com/api/v1/users")

// or grab the token directly:
tok, _ := m2m.GetToken(ctx)
```

Extra option: `WithKindeManagementAPI()` configures audience and
scopes for the Kinde management API in one call.

## JWT validation (server side)

The `jwt` package parses and validates Kinde tokens. You'll use it
to authenticate incoming requests on a resource server, and the SDK
itself uses it inside `WithTokenValidation`.

### Parsing

```go
import "github.com/kinde-oss/kinde-go/jwt"

// From an incoming request header
token, err := jwt.ParseFromAuthorizationHeader(r,
    jwt.WillValidateWithJWKSUrl(issuerURL + "/.well-known/jwks"),
    jwt.WillValidateAlgorithm("RS256"),
    jwt.WillValidateIssuer(issuerURL),
    jwt.WillValidateAudience("api.example.com"),
)

// From a raw string
token, err = jwt.ParseFromString(raw, opts...)

// From an oauth2.Token
token, err = jwt.ParseOAuth2Token(oauth2Token, opts...)

// From a session-stored JSON blob
token, err = jwt.ParseFromSessionStorage(blob, opts...)
```

### Validation options

| Option | Purpose |
|---|---|
| `WillValidateWithJWKSUrl(url)` | Fetch JWKS, verify signature |
| `WillValidateWithPublicKey(fn)` | Provide your own `*rsa.PublicKey` |
| `WillValidateWithKeyFunc(fn)` | Drop down to `jwt.Keyfunc` |
| `WillValidateAlgorithm("RS256", …)` | Pin allowed algorithms |
| `WillValidateAudience("...")` | Match `aud` |
| `WillValidateIssuer("...")` | Match `iss` |
| `WillValidateClaims(fn)` | Arbitrary claim predicate |
| `WillValidateWithClockSkew(d)` | Tolerate skew on `exp` / `nbf` |
| `WillValidateWithTimeFunc(fn)` | Inject time for tests |

### Reading claims

```go
if !token.IsValid() { ... }

sub := token.GetSubject()
iss := token.GetIssuer()
aud := token.GetAudience()
claims := token.GetClaims()           // jwt.MapClaims

if orgCode, ok := claims["org_code"].(string); ok { ... }
if perms, ok := claims["permissions"].([]interface{}); ok { ... }

at, _ := token.GetAccessToken()
it, _ := token.GetIdToken()
rt, _ := token.GetRefreshToken()
```

### Error handling

`ParseFromString` (and siblings) return both a token and an error
when validation fails partway — the token can still expose
diagnostic details:

```go
token, err := jwt.ParseFromString(raw, opts...)
if err != nil {
    for _, ve := range token.GetValidationErrors() {
        log.Printf("validation: %v", ve)
    }
    return
}
```

## Middleware patterns

### net/http

```go
func RequireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tok, err := jwt.ParseFromAuthorizationHeader(r,
            jwt.WillValidateWithJWKSUrl(issuerURL+"/.well-known/jwks"),
            jwt.WillValidateAlgorithm("RS256"),
            jwt.WillValidateIssuer(issuerURL),
            jwt.WillValidateAudience(apiAudience),
        )
        if err != nil || !tok.IsValid() {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        ctx := context.WithValue(r.Context(), ctxKeyClaims{}, tok.GetClaims())
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Gin

The repo ships an example at `examples/gin-chat` using a
`gin_kinde.UseKindeAuth()` middleware. Same idea as above — parse
the bearer, set claims into the context.

## Permission and feature-flag checks

For SDK-issued tokens, permissions and flags ride on the access
token. Read them from the claims map:

```go
claims := token.GetClaims()

// Permissions
perms, _ := claims["permissions"].([]interface{})
for _, p := range perms {
    if p == "create:todos" { ... }
}

// Feature flags
if ff, ok := claims["feature_flags"].(map[string]interface{}); ok {
    if dark, ok := ff["is_dark_mode"].(map[string]interface{}); ok {
        v, _ := dark["v"].(bool)   // value
        _ = dark["t"]              // type, "b"|"s"|"i"|"j"
        _ = v
    }
}
```

The `feature_flags` claim is keyed by flag code; each entry is
`{ t: <type>, v: <value> }`.

## Things to avoid

- Decoding tokens with `encoding/base64` and trusting the payload.
  Always go through `jwt.Parse…` with a JWKS validator.
- Sharing one `*http.Client` from `GetClient` across users. Each
  user's session hooks store their own tokens; the client closes
  over that session, so derive one per request.
- Forgetting `WithOffline()` and then asking why refresh fails.
- Logging the full access token. Log the `sub` and a token hash if
  you need correlation.
