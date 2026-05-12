# Kinde without an SDK (raw OAuth 2.0 / OIDC)

Kinde is a standards-compliant OAuth 2.0 + OIDC provider. If there
is no SDK for your language, or a use case the SDK doesn't fit, you
can talk to it directly with `http` and a JWT library.

## Endpoints

All hang off `https://<tenant>.kinde.com`:

| Endpoint | Purpose |
|---|---|
| `/.well-known/openid-configuration` | Discovery — every other URL listed here |
| `/.well-known/jwks` | Public keys for JWT signature verification |
| `/oauth2/auth` | Authorize endpoint — redirect users here |
| `/oauth2/token` | Token exchange + refresh |
| `/oauth2/user_profile` | OIDC userinfo (Bearer token) |
| `/logout` | End the Kinde session |

Always fetch `/.well-known/openid-configuration` once at boot
rather than hardcoding the others — issuer config can move.

## Grant types Kinde supports

- **Authorization Code** — server-rendered web with a client secret.
- **Authorization Code + PKCE** — SPAs, mobile, CLIs without a
  secret. Recommended even for confidential clients.
- **Device Authorization** — limited-input devices (CLIs, smart TVs).
- **Client Credentials** — machine-to-machine.
- **Refresh Token** — exchanging a refresh for a new access token.

**Implicit flow is not supported.** Do not attempt
`response_type=token`.

## Authorization code flow

### Step 1 — redirect to the authorize endpoint

```
GET https://<tenant>.kinde.com/oauth2/auth?
    response_type=code
   &client_id=<client_id>
   &redirect_uri=<callback_url>            (must match the dashboard exactly)
   &scope=openid+profile+email+offline
   &state=<random>                         (CSRF token; you check it on return)
   &nonce=<random>                         (replay protection; check inside id_token)
```

Optional parameters worth knowing:

| Parameter | Purpose |
|---|---|
| `audience` | Sets the JWT `aud` for access tokens |
| `prompt` | `login`, `create`, or `none` |
| `login_hint` | Pre-fill the email field |
| `org_code` | Log into a specific organization |
| `is_create_org` + `org_name` | Create an org during signup |
| `connection_id` | Force a specific identity provider |
| `is_use_auth_success_page` | Show a success page (mobile/desktop) |
| `lang` | UI language |
| `code_challenge` + `code_challenge_method=S256` | PKCE |
| `workflow_deployment_id` | Test a workflow deployment |

### Step 2 — handle the callback

```
GET <callback_url>?code=<auth_code>&state=<state>
```

Verify `state` matches what you stored. Then exchange:

### Step 3 — exchange code for tokens

```
POST https://<tenant>.kinde.com/oauth2/token
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code
&code=<auth_code>
&client_id=<client_id>
&client_secret=<client_secret>          (omit for public PKCE clients)
&redirect_uri=<callback_url>
&code_verifier=<verifier>               (PKCE only)
```

Response:

```json
{
  "access_token":  "eyJ…",
  "id_token":      "eyJ…",
  "refresh_token": "…",
  "token_type":    "Bearer",
  "expires_in":    3600
}
```

`refresh_token` is only present when `offline` was in the scope.
**Important**: use `offline`, not `offline_access` — Kinde doesn't
support the latter.

## PKCE

For public clients, generate:

```
code_verifier  = 43–128 random URL-safe chars
code_challenge = BASE64URL(SHA256(code_verifier))
```

Send `code_challenge` + `code_challenge_method=S256` on the
authorize URL, and `code_verifier` on the token exchange.

## Refreshing a token

```
POST https://<tenant>.kinde.com/oauth2/token
Content-Type: application/x-www-form-urlencoded

grant_type=refresh_token
&refresh_token=<refresh_token>
&client_id=<client_id>
&client_secret=<client_secret>          (omit for public clients)
```

Refresh tokens are rotated — store the new one if returned.

## Client credentials (M2M)

```
POST https://<tenant>.kinde.com/oauth2/token
Content-Type: application/x-www-form-urlencoded

grant_type=client_credentials
&client_id=<m2m_client_id>
&client_secret=<m2m_client_secret>
&audience=<your_api_audience>
&scope=<optional scopes>
```

For the Kinde management API, the audience is
`https://<tenant>.kinde.com/api`.

## Userinfo

```
GET https://<tenant>.kinde.com/oauth2/user_profile
Authorization: Bearer <access_token>
```

Returns the user's id, email, and profile data.

## Logout

```
GET https://<tenant>.kinde.com/logout?redirect=<post_logout_url>
```

`<post_logout_url>` must be on the allowed-logout-URLs list. This
clears the Kinde session; clear your own session cookie at the same
time.

## Validating tokens

Access tokens (and id tokens) are RS256-signed JWTs. To validate:

1. `GET /.well-known/jwks` and cache the keys (rotate periodically).
2. Verify the signature against the key whose `kid` matches the
   token header's `kid`.
3. Check `iss == https://<tenant>.kinde.com`.
4. Check `aud` matches your API audience (for access tokens) or
   your `client_id` (for id tokens).
5. Check `exp` is in the future and `nbf` (if present) is in the
   past. Allow a few seconds of clock skew.
6. For id tokens, check the `nonce` matches the one you sent.

Useful libraries:

| Language | Library |
|---|---|
| Go | `github.com/kinde-oss/kinde-go/jwt` or `github.com/golang-jwt/jwt/v5` + `github.com/MicahParks/keyfunc` |
| Node / TS | `jose` (`createRemoteJWKSet` + `jwtVerify`) |
| Python | `python-jose` or `authlib` |
| Rust | `jsonwebtoken` |

## Useful claims

| Claim | Meaning |
|---|---|
| `sub` | User id |
| `iss` | Issuer (your Kinde domain) |
| `aud` | Audience |
| `exp` / `iat` / `nbf` | Standard time claims |
| `scope` | Space-separated granted scopes |
| `permissions` | Array of permission strings (RBAC) |
| `roles` | Array of role objects |
| `org_code` / `org_codes` | Tenant context |
| `feature_flags` | Map of flag code → `{ t, v }` |

## Security checklist

- HTTPS for every URL.
- Never put `client_secret` in front-end code.
- Always use `state`, and validate it.
- Always use `nonce` if you read id-token claims, and validate it.
- Store refresh tokens server-side or in httpOnly cookies — never
  in `localStorage`.
- Validate JWT signatures via JWKS, not just `base64` decode.
- Pin the algorithm to `RS256` (don't accept `none` or `HS256`).
- Match `iss` and `aud` explicitly.
- Don't log full tokens — log a hash and the `sub`.

## Minimal Go example (no SDK)

```go
// 1. Authorize URL
q := url.Values{}
q.Set("response_type", "code")
q.Set("client_id", clientID)
q.Set("redirect_uri", callback)
q.Set("scope", "openid profile email offline")
q.Set("state", state)
authURL := issuer + "/oauth2/auth?" + q.Encode()

// 2. Token exchange in /callback
form := url.Values{}
form.Set("grant_type",    "authorization_code")
form.Set("code",          r.URL.Query().Get("code"))
form.Set("client_id",     clientID)
form.Set("client_secret", clientSecret)
form.Set("redirect_uri",  callback)

resp, err := http.PostForm(issuer+"/oauth2/token", form)
```

## Minimal Node example (no SDK)

```ts
// Token exchange
const body = new URLSearchParams({
  grant_type:    "authorization_code",
  code:          req.query.code as string,
  client_id:     CLIENT_ID,
  client_secret: CLIENT_SECRET,
  redirect_uri:  CALLBACK,
});
const r = await fetch(`${ISSUER}/oauth2/token`, {
  method: "POST",
  headers: { "Content-Type": "application/x-www-form-urlencoded" },
  body,
});
const tokens = await r.json();
```
