---
name: kinde
description: >-
  Integrate the Kinde auth platform (single sign-on, OAuth 2.0 / OIDC,
  organizations, RBAC, feature flags, M2M, management API) into Go and
  TypeScript applications. ALWAYS use this skill when the user mentions
  Kinde, docs.kinde.com, a kinde.com subdomain, the @kinde-oss or
  @kinde packages, kinde-typescript-sdk, kinde-auth-nextjs,
  kinde-oss/kinde-go, or asks to wire up login/logout/callback,
  validate Kinde JWTs, call the Kinde management API, gate features on
  Kinde flags, or build machine-to-machine auth against Kinde. Covers
  the Authorization Code (+ PKCE) flow, Client Credentials (M2M)
  flow, Device flow, token refresh, JWKS validation, organizations
  and multi-tenancy, roles and permissions, and feature flags. Go and
  TypeScript are first-class; the OAuth-without-an-SDK guidance also
  applies to any language that speaks HTTP and JWT.
version: 1.0.0
tags:
  - kinde
  - auth
  - sso
  - oauth
  - oidc
  - jwt
  - go
  - typescript
  - nextjs
---

# Kinde

Kinde is an Auth0-style identity platform: hosted login pages,
OAuth 2.0 / OIDC under the hood, plus organizations (multi-tenancy),
roles + permissions (RBAC), feature flags, billing, and a REST
management API. This skill covers the integration patterns Claude
needs most: **Go** and **TypeScript** (Node, Next.js App Router,
plain TS server, browser SPA).

## How to pick a path

| You are building... | Load this reference |
|---|---|
| A Go web service or CLI (auth code or device flow) | `references/go.md` |
| A Go service calling the Kinde management API or other M2M | `references/go.md` (client credentials section) |
| A TypeScript / Node backend (Express, Hono, vanilla) | `references/typescript.md` |
| A Next.js 13+ App Router app | `references/nextjs.md` |
| Anything in a language without an SDK, or you need the raw protocol | `references/oauth-without-sdk.md` |
| Calling the Kinde management API (any language) | `references/management-api.md` |

The references are self-contained — load only what the current task
needs. The summary below is enough to scope work and pick the right
flow.

## Concepts in 60 seconds

- **Issuer / domain** — every Kinde tenant has a subdomain like
  `your-tenant.kinde.com`. That URL is the OIDC issuer; everything
  hangs off it (`/oauth2/auth`, `/oauth2/token`, `/logout`,
  `/.well-known/openid-configuration`, `/.well-known/jwks`,
  `/oauth2/user_profile`).
- **Applications** — credentials live on an "application" in the
  Kinde dashboard. Three kinds you'll see:
  - **Back-end web** (confidential, has `client_secret`) — auth code
    flow with a server.
  - **Front-end / SPA / mobile** (public, no secret) — auth code +
    PKCE.
  - **Machine-to-machine (M2M)** — client credentials flow, no user.
- **Grants Kinde supports** — authorization code (with optional
  PKCE), authorization code + PKCE for SPAs, device authorization
  for CLIs and TVs, client credentials for M2M, refresh token.
  Implicit flow is **not supported**.
- **Tokens** — three of them, all JWTs:
  - `id_token` — user identity (sub, email, name). Validate, don't
    send to APIs.
  - `access_token` — what your APIs receive in `Authorization:
    Bearer …`. Validate signature via JWKS, plus `iss`, `aud`, `exp`.
  - `refresh_token` — only issued when the `offline` scope is
    requested. Store httpOnly and server-side.
- **Scopes** — `openid profile email offline` is the typical set.
  Use `offline` (not `offline_access`).
- **Organizations** — Kinde's multi-tenancy primitive. Identified
  by `org_code` like `org_1234`. A user can belong to many; pass
  `org_code=…` on the authorize URL to log them into a specific one.
- **RBAC** — define permissions (e.g. `create:todos`) and roles in
  the dashboard, assign to users per-org. Permissions land on the
  access token as the `permissions` claim and a `roles` claim.
- **Feature flags** — boolean / string / integer / JSON, evaluated
  business → environment → org → user. Read via SDK helpers or the
  `feature_flags` claim on the access token.

## Required configuration

Whatever the language, you need:

| Setting | Where to find it |
|---|---|
| Issuer URL (`https://<tenant>.kinde.com`) | Settings → Applications → your app |
| `client_id` | same |
| `client_secret` | same (back-end / M2M only — never ship to a browser) |
| Allowed callback URL(s) | Settings → Applications → your app → View details. Must match exactly. |
| Allowed logout redirect URL(s) | same |
| Audience (optional) | Set to your API's identifier when you want access tokens scoped to it |

Hardcoding any of these is a smell — load from env (`KINDE_DOMAIN`,
`KINDE_CLIENT_ID`, `KINDE_CLIENT_SECRET`, `KINDE_REDIRECT_URL`,
`KINDE_LOGOUT_REDIRECT_URL` are the names the official SDKs expect).

## Go — the short version

The official Go SDK lives at `github.com/kinde-oss/kinde-go`
(requires Go 1.24+) and ships three packages:

- `oauth2/authorization_code` — browser auth code flow **and**
  device authorization flow (CLIs).
- `oauth2/client_credentials` — M2M.
- `jwt` — parse and validate tokens from headers, strings, sessions,
  or OAuth2 tokens, with JWKS-based signature verification.

Minimal auth code setup:

```go
import (
    "github.com/kinde-oss/kinde-go/oauth2/authorization_code"
    "github.com/kinde-oss/kinde-go/jwt"
)

flow, err := authorization_code.NewAuthorizationCodeFlow(
    issuerURL, clientID, clientSecret, callbackURL,
    authorization_code.WithSessionHooks(sessionStore),
    authorization_code.WithOffline(),
    authorization_code.WithAudience(apiAudience),
    authorization_code.WithTokenValidation(
        true,
        jwt.WillValidateAlgorithm(),
        jwt.WillValidateAudience(apiAudience),
    ),
)
```

Full reference in `references/go.md`: PKCE option, prompt option,
device flow, M2M, JWT validation helpers, middleware patterns
(net/http and Gin).

## TypeScript — the short version

Pick the package that matches the runtime:

| Stack | Package |
|---|---|
| Next.js 13+ App Router | `@kinde-oss/kinde-auth-nextjs` |
| Express / Hono / vanilla Node | `@kinde-oss/kinde-typescript-sdk` |
| React SPA | `@kinde-oss/kinde-auth-react` |
| Browser-only JS | `@kinde-oss/kinde-auth-pkce-js` |
| Management API client | `@kinde/management-api-js` |

Vanilla TS server (Express-style):

```ts
import { createKindeServerClient, GrantType }
    from "@kinde-oss/kinde-typescript-sdk";

const kinde = createKindeServerClient(GrantType.AUTHORIZATION_CODE, {
  authDomain: process.env.KINDE_DOMAIN!,
  clientId: process.env.KINDE_CLIENT_ID!,
  clientSecret: process.env.KINDE_CLIENT_SECRET!,
  redirectURL: process.env.KINDE_REDIRECT_URL!,
  logoutRedirectURL: process.env.KINDE_LOGOUT_REDIRECT_URL!,
});
```

Then expose `/login`, `/register`, `/callback`, `/logout` handlers,
each operating on a per-request `SessionManager`. Full code,
session-manager interface, and helper APIs (`getUserProfile`,
`getPermission`, `getBooleanFlag`, `createOrg`, …) in
`references/typescript.md`. Next.js specifics — route handler,
proxy middleware, server vs. client helpers, `<LoginLink>` /
`<LogoutLink>` components — are in `references/nextjs.md`.

## When in doubt: use the raw protocol

Kinde is standards-compliant OAuth 2.0 / OIDC. If a language has no
SDK, or a use case doesn't fit one, fall back to the protocol:

- Authorize: `GET https://<tenant>.kinde.com/oauth2/auth?…`
- Token: `POST https://<tenant>.kinde.com/oauth2/token`
- User profile: `GET /oauth2/user_profile`
  (Bearer access token)
- JWKS: `GET /.well-known/jwks`
- Logout: `GET /logout?redirect=<url>`
- Discovery: `GET /.well-known/openid-configuration`

Endpoints, parameter tables, PKCE recipe, and security checklist in
`references/oauth-without-sdk.md`.

## Pitfalls Claude should call out

1. **Scope is `offline`, not `offline_access`.** Kinde explicitly
   does not support `offline_access`. If a refresh token is missing,
   that's the first thing to check.
2. **Callback URLs must match exactly.** Trailing slash, scheme,
   port — all part of the match. Errors usually surface as
   `invalid_request` on the authorize endpoint.
3. **`client_secret` is server-only.** SPAs and mobile use PKCE.
   If a user pastes a SPA snippet that includes a secret, flag it.
4. **Validate JWTs, don't just decode them.** Verify signature via
   JWKS, plus `iss`, `aud`, `exp`. Both Go (`jwt.WillValidateWith…`)
   and TS SDKs do this automatically when configured; raw-protocol
   integrations must do it explicitly.
5. **Org context comes from a claim, not the URL.** After login,
   read `org_code` off the access token; trusting a query string is
   the standard tenant-mix-up bug.
6. **Implicit flow is not supported.** Don't try to wire up
   `response_type=token`.
7. **Pages Router vs App Router** in Next.js use *different* SDK
   surface area. The App Router patterns in `references/nextjs.md`
   do not transfer one-for-one to the Pages Router SDK.

## Reference guides

- `references/go.md` — authorization code, device, client
  credentials, JWT validation, middleware. Full APIs.
- `references/typescript.md` — server SDK, session manager,
  organizations, flags, permissions, refresh.
- `references/nextjs.md` — App Router integration: route handler,
  proxy/middleware, server + client helpers, components.
- `references/oauth-without-sdk.md` — endpoints, parameters,
  PKCE, refresh, logout, security checklist.
- `references/management-api.md` — calling the management API
  (users, orgs, roles, permissions, flags) from any language.

## MCP

Use Context7 MCP (`resolve-library-id` then `query-docs`) to pull
the freshest official docs for `kinde-typescript-sdk`,
`kinde-auth-nextjs`, or `kinde-go` when version-specific behaviour
matters.
