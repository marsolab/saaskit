# Deployment Reference

## Overview

Deploy Astro applications to modern hosting platforms. Astro supports static,
SSR, and hybrid rendering — choose the adapter matching your deployment target.

## Vercel

### Setup

```bash
bunx astro add vercel
```

### Configuration

```typescript
// astro.config.mjs
import { defineConfig } from 'astro/config';
import vercel from '@astrojs/vercel';

export default defineConfig({
  output: 'server', // or 'hybrid' for per-page choice
  adapter: vercel({
    webAnalytics: { enabled: true },
    imageService: true,
    // Edge functions for specific routes
    edgeMiddleware: true,
  }),
});
```

### Deploy

```bash
# CLI deploy
bunx vercel

# Production deploy
bunx vercel --prod
```

Or connect your Git repo in the Vercel dashboard:
- Build command: `bun run build`
- Output directory: `dist`
- Install command: `bun install`

## Netlify

### Setup

```bash
bunx astro add netlify
```

### Configuration

```typescript
// astro.config.mjs
import { defineConfig } from 'astro/config';
import netlify from '@astrojs/netlify';

export default defineConfig({
  output: 'server',
  adapter: netlify({
    edgeMiddleware: true, // Use Netlify Edge Functions
  }),
});
```

### Deploy

```bash
# CLI deploy
bunx netlify deploy

# Production deploy
bunx netlify deploy --prod
```

### Redirects and Headers

```text
# public/_redirects
/old-page  /new-page  301
/api/*     /.netlify/functions/:splat  200
```

```text
# public/_headers
/*
  X-Frame-Options: DENY
  X-Content-Type-Options: nosniff
```

## Cloudflare Pages

### Setup

```bash
bunx astro add cloudflare
```

### Configuration

```typescript
// astro.config.mjs
import { defineConfig } from 'astro/config';
import cloudflare from '@astrojs/cloudflare';

export default defineConfig({
  output: 'server',
  adapter: cloudflare({
    platformProxy: { enabled: true }, // Access KV, D1, R2 bindings
  }),
});
```

### Deploy

```bash
# Install Wrangler
bun add -d wrangler

# Build and deploy
bun run build
bunx wrangler pages deploy dist
```

### Access Cloudflare Bindings

```typescript
// src/pages/api/data.ts
import type { APIRoute } from 'astro';

export const GET: APIRoute = async ({ locals }) => {
  // Access KV, D1, R2 via locals.runtime.env
  const { env } = locals.runtime;
  const value = await env.MY_KV.get('key');
  return Response.json({ value });
};
```

## Docker

### Dockerfile for Astro + Bun

```dockerfile
# Build stage
FROM oven/bun:1 AS build
WORKDIR /app
COPY package.json bun.lockb ./
RUN bun install --frozen-lockfile
COPY . .
RUN bun run build

# Production stage
FROM oven/bun:1-slim
WORKDIR /app
COPY --from=build /app/dist ./dist
COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/package.json ./
ENV HOST=0.0.0.0
ENV PORT=4321
EXPOSE 4321
CMD ["bun", "run", "preview"]
```

```bash
# Build and run
docker build -t my-astro-app .
docker run -p 4321:4321 my-astro-app
```

### Docker Compose (with reverse proxy)

```yaml
# docker-compose.yml
services:
  app:
    build: .
    ports:
      - '4321:4321'
    env_file: .env
    restart: unless-stopped
```

## Static Export (No Adapter)

For purely static sites, no adapter is needed:

```typescript
// astro.config.mjs — default output is 'static'
export default defineConfig({
  site: 'https://example.com',
});
```

```bash
bun run build
# Output in dist/ — deploy to any static host:
# S3 + CloudFront, GitHub Pages, any CDN, Surge, etc.
```

### GitHub Pages

```yaml
# .github/workflows/deploy.yml
name: Deploy to GitHub Pages
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      pages: write
      id-token: write
    steps:
      - uses: actions/checkout@v4
      - uses: oven-sh/setup-bun@v1
      - run: bun install
      - run: bun run build
      - uses: actions/upload-pages-artifact@v3
        with:
          path: dist
      - uses: actions/deploy-pages@v4
```

## Environment Variables

| Prefix | Access | Example |
|--------|--------|---------|
| `PUBLIC_` | Client + server | `PUBLIC_API_URL` |
| (none) | Server only | `DATABASE_URL` |
| `SECRET_` | Server only (Astro convention) | `SECRET_API_KEY` |

- Build-time variables: baked into the output, visible in client code if PUBLIC_
- Runtime variables (SSR): available via `import.meta.env` or `process.env`
- Secrets: never prefix with PUBLIC_, use the platform's secret store

## Pre-Deploy Checklist

1. `bun run build` locally — fix all errors
2. `bun run preview` — test the production build
3. All environment variables set in target platform
4. Redirects and headers configured
5. Custom 404 page works
6. Run Lighthouse audit on preview
7. Check `robots.txt` and `sitemap.xml` generated correctly
8. Verify HTTPS and security headers
