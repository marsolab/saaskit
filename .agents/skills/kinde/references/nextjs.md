# Kinde + Next.js (App Router)

Package: `@kinde-oss/kinde-auth-nextjs`. Requires Node 20+ and
Next.js 13+ with App Router. For Pages Router, use the dedicated
Pages Router SDK — the APIs differ.

```bash
npm i @kinde-oss/kinde-auth-nextjs
```

## Environment

```
KINDE_CLIENT_ID=...
KINDE_CLIENT_SECRET=...
KINDE_DOMAIN=https://<tenant>.kinde.com
KINDE_REDIRECT_URL=http://localhost:3000/api/auth/kinde_callback
KINDE_LOGOUT_REDIRECT_URL=http://localhost:3000
# optional
KINDE_AUTH_API_PATH=/api/auth                 # default
KINDE_POST_LOGIN_REDIRECT_URL=/dashboard      # static post-login redirect
```

The default callback path is `/api/auth/kinde_callback`. If you
override the base path with `KINDE_AUTH_API_PATH=/foo`, set
`KINDE_REDIRECT_URL` to `…/foo/kinde_callback` and update the
dashboard.

## The single auth route

The SDK ships one route handler that fans out to login, register,
callback, logout, refresh, etc., based on the URL segment.

```ts
// app/api/auth/[kindeAuth]/route.ts
import { handleAuth } from "@kinde-oss/kinde-auth-nextjs/server";

export const GET = handleAuth();
```

URLs that result:

| URL | Action |
|---|---|
| `/api/auth/login` | Redirect to Kinde sign-in |
| `/api/auth/register` | Redirect to Kinde sign-up |
| `/api/auth/logout` | Sign out |
| `/api/auth/kinde_callback` | Token exchange + redirect |

## Server components and route handlers

```ts
import { getKindeServerSession } from "@kinde-oss/kinde-auth-nextjs/server";

export default async function Dashboard() {
  const { isAuthenticated, getUser } = getKindeServerSession();

  if (!(await isAuthenticated())) {
    return <a href="/api/auth/login">Sign in</a>;
  }

  const user = await getUser();
  return <p>Hi {user?.given_name}</p>;
}
```

All helpers on `getKindeServerSession()`:

| Helper | Returns |
|---|---|
| `isAuthenticated()` | `boolean` |
| `getUser()` | `{ id, email, given_name, family_name, picture }` |
| `getOrganization()` | `{ orgCode, orgName }` |
| `getUserOrganizations()` | `{ orgCodes: string[] }` |
| `getPermission(code)` | `{ isGranted, orgCode }` |
| `getPermissions()` | `{ permissions: string[], orgCode }` |
| `getAccessToken()` / `getIdToken()` | Decoded claims object |
| `getClaim(name, "id_token" \| "access_token")` | `{ name, value }` |
| `getFlag(code, default?, type?)` | Flag value with default fallback |
| `getBooleanFlag` / `getIntegerFlag` / `getStringFlag` | Typed variants |

## Client components

```tsx
"use client";
import { useKindeBrowserClient } from "@kinde-oss/kinde-auth-nextjs";

export default function Profile() {
  const { user, isAuthenticated, isLoading, error } = useKindeBrowserClient();
  if (isLoading) return null;
  return isAuthenticated
    ? <span>{user?.given_name}</span>
    : <a href="/api/auth/login">Sign in</a>;
}
```

`useKindeBrowserClient` exposes the same helpers as the server
session plus:

- `isLoading: boolean`
- `error: string | null`
- `refreshData(): Promise<void>` — force a token refresh

## Components

Pre-built link components that point at the right auth endpoint:

```tsx
import {
  LoginLink, RegisterLink, LogoutLink,
} from "@kinde-oss/kinde-auth-nextjs/components";

<LoginLink    postLoginRedirectURL="/dashboard">Sign in</LoginLink>
<RegisterLink postLoginRedirectURL="/welcome">Create account</RegisterLink>
<LogoutLink>Sign out</LogoutLink>
```

These render as anchors to `/api/auth/login?post_login_redirect_url=…`
etc., so they work without JS.

## Protecting routes via middleware / proxy

Next.js 13–15 uses `middleware.ts`. Next.js 16 renamed it to
`proxy.ts`. The Kinde import is the same.

```ts
// middleware.ts  (or proxy.ts on Next 16+)
import { withAuth } from "@kinde-oss/kinde-auth-nextjs/middleware";

export default withAuth(
  async function middleware(req) {
    // optional: extra logic
  },
  {
    publicPaths: ["/", "/blog", "/api/public"],
    isReturnToCurrentPage: true,
    loginPage: "/api/auth/login",
  },
);

export const config = {
  matcher: [
    "/((?!_next|[^?]*\\.(?:html?|css|js|png|svg|ico)).*)",
  ],
};
```

`publicPaths` is the allow-list; everything else redirects to
`loginPage`. With `isReturnToCurrentPage: true` the user lands back
on the path they were trying to reach.

## Optional KindeProvider for client context

Most apps don't need it (`useKindeBrowserClient` works without).
Wrap the layout if you have providers that need session-derived
context:

```tsx
// app/AuthProvider.tsx
"use client";
import { KindeProvider } from "@kinde-oss/kinde-auth-nextjs";
export const AuthProvider = ({ children }: { children: React.ReactNode }) =>
  <KindeProvider>{children}</KindeProvider>;
```

```tsx
// app/layout.tsx
import { AuthProvider } from "./AuthProvider";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return <html><body><AuthProvider>{children}</AuthProvider></body></html>;
}
```

## Patterns

**Server component requires auth + redirect:**

```ts
import { redirect } from "next/navigation";
import { getKindeServerSession } from "@kinde-oss/kinde-auth-nextjs/server";

export default async function Page() {
  const { isAuthenticated } = getKindeServerSession();
  if (!(await isAuthenticated())) redirect("/api/auth/login");
  // ...
}
```

**Role/permission gate:**

```ts
const { getPermission } = getKindeServerSession();
const { isGranted } = await getPermission("delete:posts");
if (!isGranted) redirect("/forbidden");
```

**Org switcher:**

```ts
const { getUserOrganizations } = getKindeServerSession();
const { orgCodes } = await getUserOrganizations();
// Render links to `/api/auth/login?org_code=org_xxx`
```

## Server-side fetch to your own API

The access token isn't exposed by default — read it explicitly:

```ts
import { getKindeServerSession } from "@kinde-oss/kinde-auth-nextjs/server";

const { getAccessTokenRaw } = getKindeServerSession();
const token = await getAccessTokenRaw();

await fetch("https://api.example.com/me", {
  headers: { Authorization: `Bearer ${token}` },
});
```

## Pitfalls

- **Pages Router code doesn't work here.** Different package,
  different APIs. Check imports if examples look wrong.
- **Edge runtime + cookies.** The Kinde middleware reads cookies on
  the edge; make sure your matcher excludes static assets, or you
  trigger token refresh on every PNG request.
- **`KINDE_REDIRECT_URL` mismatch.** If `KINDE_AUTH_API_PATH` is
  changed, the redirect URL must be updated in **both** the env and
  the dashboard.
- **Multiple Kinde domains.** The SDK reads env vars at module
  load. Don't try to swap tenants per request — run separate apps.
