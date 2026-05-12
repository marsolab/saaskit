# Kinde Management API

Use the management API for everything you can't do through the
hosted UI: provisioning users, creating organizations, assigning
roles and permissions, defining feature flags, rotating
applications, listing audit events.

## Base URL

```
https://<tenant>.kinde.com/api/v1
```

OpenAPI spec lives at the docs site (`/kinde-apis/management/`).

## Auth — get an M2M token first

The management API is M2M-only. You authenticate as a Kinde
**M2M application** that has been granted scopes for the management
API.

1. In the dashboard, create an M2M application.
2. Enable the **Kinde Management API** under "Allowed APIs" and
   tick the scopes you need (e.g. `read:users`, `update:users`).
3. Use those credentials to fetch a token:

```
POST https://<tenant>.kinde.com/oauth2/token
Content-Type: application/x-www-form-urlencoded

grant_type=client_credentials
&client_id=<m2m_client_id>
&client_secret=<m2m_client_secret>
&audience=https://<tenant>.kinde.com/api
```

Cache the access token until ~5 minutes before `exp`. It's typically
valid for an hour.

## Calling the API

```
GET  https://<tenant>.kinde.com/api/v1/users
Authorization: Bearer <m2m_access_token>
Accept: application/json
```

Common resource groups:

| Group | What's there |
|---|---|
| `/users` | List, create, update, delete users; password resets |
| `/organizations` | Create orgs, add/remove members, set org metadata |
| `/organizations/{org_code}/users` | Org membership + per-org role assignment |
| `/roles` | Define roles, attach permissions |
| `/permissions` | Define permissions |
| `/feature_flags` | Define flags and per-environment / per-org / per-user overrides |
| `/applications` | List and manage apps and their connections |
| `/connections` | Identity provider connections (Google, Microsoft, custom OIDC, …) |
| `/properties` | Custom user / org properties |
| `/business` | Tenant-level settings |
| `/api_keys` | API key management |
| `/webhooks` | Outbound webhooks |
| `/event_logs` | Audit events |

The exact paths and payloads change occasionally; consult the
OpenAPI reference at `https://docs.kinde.com/kinde-apis/management/`
when in doubt.

## Go

The kinde-go SDK ships a generated management client. Configure the
client credentials flow once and reuse the HTTP client:

```go
import (
    "github.com/kinde-oss/kinde-go/oauth2/client_credentials"
)

m2m, _ := client_credentials.NewClientCredentialsFlow(
    issuer, clientID, clientSecret,
    client_credentials.WithKindeManagementAPI(),  // sets audience + scopes
)

client, _ := m2m.GetClient(ctx)
resp, _ := client.Get(issuer + "/api/v1/users")
```

The `WithKindeManagementAPI()` helper picks the right audience for
you. See `README_MANAGEMENT_API.md` in the kinde-go repo for the
generated client surface.

## TypeScript

Use the dedicated package — it's a generated client that handles
auth + retries:

```bash
npm i @kinde/management-api-js
```

```ts
import { init, Users, Organizations } from "@kinde/management-api-js";

init({
  kindeDomain:  process.env.KINDE_DOMAIN!,
  clientId:     process.env.KINDE_M2M_CLIENT_ID!,
  clientSecret: process.env.KINDE_M2M_CLIENT_SECRET!,
  // audience is set automatically to <kindeDomain>/api
});

const { users } = await Users.getUsers();
const org = await Organizations.createOrganization({
  requestBody: { name: "Acme Inc" },
});
```

Each resource is a namespace; method names mirror operation IDs in
the OpenAPI spec.

## Raw HTTP (any language)

```bash
# 1. Get a token
curl -s -X POST "https://$TENANT.kinde.com/oauth2/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=$CLIENT_ID" \
  -d "client_secret=$CLIENT_SECRET" \
  -d "audience=https://$TENANT.kinde.com/api" \
  | jq -r .access_token > .token

# 2. Use it
curl -s "https://$TENANT.kinde.com/api/v1/users" \
  -H "Authorization: Bearer $(cat .token)"
```

## Pagination

List endpoints return `{ code, message, users: [...], next_token }`.
Pass `next_token` as `next_token` on the next call. Page size is
controlled by `page_size` (default 10, max 500 for most resources).

## Rate limits

The API is rate-limited per tenant. Expect `429 Too Many Requests`
under load — back off with `Retry-After`. The generated SDKs handle
this automatically; raw clients should add a retry policy.

## Common operations

**Create a user:**

```
POST /api/v1/user
{
  "profile":          { "given_name": "Ada", "family_name": "Lovelace" },
  "identities":       [{ "type": "email", "details": { "email": "ada@example.com" } }],
  "organization_code": "org_1234"
}
```

**Assign a role to a user in an org:**

```
POST /api/v1/organizations/{org_code}/users/{user_id}/roles
{ "role_id": "role_xxx" }
```

**Toggle a feature flag for an org:**

```
PATCH /api/v1/organizations/{org_code}/feature_flags/{flag_code}
{ "value": true }
```

**Look up a user by email:**

```
GET /api/v1/users?email=ada@example.com
```

## Pitfalls

- **Wrong audience** — must be `https://<tenant>.kinde.com/api`,
  not `https://<tenant>.kinde.com`.
- **Missing scopes** — the M2M app must be explicitly granted each
  permission it uses; check the "Allowed APIs" tab.
- **Mixing tokens** — user access tokens cannot call the management
  API. Only M2M tokens can.
- **Hardcoding IDs** — `org_code`, role IDs, and flag codes are
  tenant-specific. Discover them at runtime.
