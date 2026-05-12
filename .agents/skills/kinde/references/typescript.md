# Kinde TypeScript / Node reference

For Next.js App Router, see `nextjs.md` — it uses a different
package. This file covers `@kinde-oss/kinde-typescript-sdk`, which
fits any Node 16+ server (Express, Fastify, Hono, raw `http`).

```bash
npm i @kinde-oss/kinde-typescript-sdk
```

## Configure once at boot

```ts
import {
  createKindeServerClient,
  GrantType,
} from "@kinde-oss/kinde-typescript-sdk";

export const kinde = createKindeServerClient(GrantType.AUTHORIZATION_CODE, {
  authDomain:        process.env.KINDE_DOMAIN!,         // https://<tenant>.kinde.com
  clientId:          process.env.KINDE_CLIENT_ID!,
  clientSecret:      process.env.KINDE_CLIENT_SECRET!,
  redirectURL:       process.env.KINDE_REDIRECT_URL!,   // must match dashboard
  logoutRedirectURL: process.env.KINDE_LOGOUT_REDIRECT_URL!,
  // optional:
  // scope:    "openid profile email offline",
  // audience: "api.example.com",
});
```

Grant types:

- `GrantType.AUTHORIZATION_CODE` — server-rendered web with a
  client secret.
- `GrantType.PKCE` — SPAs / mobile, no client secret. (Most browser
  apps use `@kinde-oss/kinde-auth-pkce-js` or
  `@kinde-oss/kinde-auth-react` instead; the server SDK in PKCE
  mode is for niche cases.)
- `GrantType.CLIENT_CREDENTIALS` — M2M.

## SessionManager — the only interface you must implement

Every method is per-request. Back it with whatever you already have:
signed cookies, Redis, a row in Postgres. The shape:

```ts
interface SessionManager {
  getSessionItem(key: string):    Promise<unknown | null>;
  setSessionItem(key: string, v): Promise<void>;
  removeSessionItem(key: string): Promise<void>;
  destroySession():               Promise<void>;
}
```

Express example backed by `express-session`:

```ts
function sessionFor(req: Request, res: Response): SessionManager {
  return {
    async getSessionItem(k)    { return (req.session as any)[k] ?? null; },
    async setSessionItem(k, v) { (req.session as any)[k] = v; },
    async removeSessionItem(k) { delete (req.session as any)[k]; },
    async destroySession()     { await new Promise(r => req.session.destroy(r)); },
  };
}
```

In production: encrypt the session cookie (`express-session` +
`cookie.secure = true`), or store sessions server-side. Plain
in-memory maps are fine for examples only — they don't survive a
restart and break across processes.

## The four routes

```ts
app.get("/login", async (req, res) => {
  const url = await kinde.login(sessionFor(req, res));
  res.redirect(url.toString());
});

app.get("/register", async (req, res) => {
  const url = await kinde.register(sessionFor(req, res));
  res.redirect(url.toString());
});

app.get("/callback", async (req, res) => {
  const url = new URL(`${req.protocol}://${req.get("host")}${req.originalUrl}`);
  await kinde.handleRedirectToApp(sessionFor(req, res), url);
  res.redirect("/");
});

app.get("/logout", async (req, res) => {
  const url = await kinde.logout(sessionFor(req, res));
  res.redirect(url.toString());
});
```

`handleRedirectToApp` does state validation, token exchange, and
stores tokens via your `SessionManager`. After it returns the user
is authenticated for subsequent requests.

## Reading the user

```ts
const session = sessionFor(req, res);

await kinde.isAuthenticated(session);          // boolean
await kinde.getUserProfile(session);           // { id, given_name, family_name, email, picture }
await kinde.getToken(session);                 // raw access token string
await kinde.refreshTokens(session);            // force a refresh
```

## Organizations (multi-tenancy)

```ts
// Log into a specific org
await kinde.login(session,    { org_code: "org_1234" });
await kinde.register(session, { org_code: "org_1234" });

// Create one during signup
const url = await kinde.createOrg(session, { org_name: "Acme Inc" });
res.redirect(url.toString());

// Read current org
await kinde.getOrganization(session);          // { orgCode: "org_1234" }
await kinde.getUserOrganizations(session);     // { orgCodes: ["org_1234", "org_4567"] }
```

## Permissions

```ts
await kinde.getPermission(session, "create:todos");
// → { orgCode: "org_1234", isGranted: true }

await kinde.getPermissions(session);
// → { orgCode: "org_1234", permissions: ["create:todos", "read:todos"] }
```

## Feature flags

```ts
await kinde.getFlag(session, "theme");                            // generic
await kinde.getBooleanFlag(session, "is_dark_mode", false);
await kinde.getStringFlag(session,  "theme",        "black");
await kinde.getIntegerFlag(session, "team_count",   2);
```

Each helper takes an optional default for when the flag isn't set,
which makes them safe to call without first checking existence.

## Claims

If you need raw access to the token claims:

```ts
await kinde.getClaim(session, "email", "id_token");
// → { name: "email", value: "ada@example.com" }

await kinde.getClaimValue(session, "aud");
// just the value
```

The second argument selects which token to read: `"id_token"` or
`"access_token"` (default).

## Verifying tokens on a separate resource server

If the consumer of your access tokens isn't the same Node process
that runs the Kinde SDK, validate JWTs directly:

```ts
import { createRemoteJWKSet, jwtVerify } from "jose";

const JWKS = createRemoteJWKSet(
  new URL(`${process.env.KINDE_DOMAIN}/.well-known/jwks`)
);

export async function requireAuth(req, res, next) {
  const bearer = req.headers.authorization?.replace(/^Bearer\s+/i, "");
  if (!bearer) return res.status(401).end();
  try {
    const { payload } = await jwtVerify(bearer, JWKS, {
      issuer:   process.env.KINDE_DOMAIN!,
      audience: "api.example.com",
    });
    (req as any).claims = payload;
    next();
  } catch {
    res.status(401).end();
  }
}
```

`jose` is the standard pick. Same idea applies to `jsonwebtoken` or
any JWT library — fetch JWKS, verify `iss`, `aud`, `exp`.

## Frontend SPAs

If your TypeScript code runs in a browser, do **not** use this SDK
with a client secret. Use the dedicated PKCE packages:

| Package | When |
|---|---|
| `@kinde-oss/kinde-auth-react` | React SPA |
| `@kinde-oss/kinde-auth-pkce-js` | Plain JS / TS SPA |

Both expose hooks/methods named the same way the server SDK does
(`isAuthenticated`, `getUser`, `getPermission`, `getFlag`, …) but
they store tokens in the browser and use PKCE under the hood.

## Management API client

Separate package — see `management-api.md`.

```bash
npm i @kinde/management-api-js
```

## Pitfalls

- **Reusing a `SessionManager` across requests** mixes users' tokens
  together. Always derive per-request.
- **`scope: "offline_access"`** — Kinde wants `offline`. Refresh
  tokens silently disappear if you send `offline_access`.
- **Mismatched `redirectURL`** — the value you pass when creating
  the client must match an entry in the dashboard *exactly*,
  including scheme and port.
- **In-memory sessions in serverless** — each invocation gets a
  fresh process. Use signed cookies or an external store.
- **CORS on /oauth2/token** — token exchange happens server-side
  with a secret, not from the browser. If you're seeing CORS errors,
  you're probably using the wrong SDK for an SPA.
