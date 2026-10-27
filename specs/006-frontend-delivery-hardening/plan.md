# Implementation Plan: Frontend & Delivery Hardening

**Branch**: `006-frontend-delivery-hardening` | **Date**: 2026-10-26 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/006-frontend-delivery-hardening/spec.md`

## Summary

Three changes to the web app and its delivery, none of which adds product behaviour.

1. **One browser session that survives a reload** (US1). The two session sources become one
   session store owned by `AuthProvider`: react-oidc-context's user and the demo token in
   `sessionStorage`. Only a small *session hint* is persisted, never an access token: which
   kind of session the tab has, and for OIDC the rotating refresh token. On load the store
   restores the session (an OIDC refresh-token grant, or re-minting the demo token from its
   public key). The module-level `setAccessTokenGetter` is replaced by an API client provided
   through context. Every sign-in, sign-out or account switch calls `queryClient.clear()`.
2. **One API client, no example data in the production build** (US2). `apiFetch` plus a single
   `ApiError` become the only way to call the server (`ClaimApiError` is merged into it).
   Example data moves to `src/dev/fixtures/`, which is used by the tests and by an opt-in
   fixtures mode for local development and Lighthouse. Production builds do not contain it
   (enforced by a build check). PDOK address search moves into `services/geocoding.ts`.
3. **Locked-down delivery** (US3).
   - A new production web image: multi-stage, Node 24 build, served by unprivileged nginx. It
     sets a strict CSP and the other security headers, proxies `/api` to the backend, and
     serves a runtime `app-config.json` rendered from environment variables, so one image
     serves every environment.
   - The backend image runs as a non-root user.
   - Compose binds every port to `127.0.0.1`.
   - The Bricolage Grotesque font is self-hosted.
   - Listing photos render only from the configured image hosts.

## Technical Context

**Language/Version**:
- TypeScript 7 / React 19 (frontend).
- Go 1.27 (backend; only its Dockerfile changes).
- nginx config and shell for the web image.

**Primary Dependencies**:
- Existing: React Router 7, TanStack Query 5, oidc-client-ts 3.5 + react-oidc-context 3.3,
  Stripe.js 10 / react-stripe-js 7, Tailwind CSS 4, Vite 8.
- New runtime dependency: `@fontsource-variable/bricolage-grotesque` (self-hosted font,
  bundled by Vite).
- New dev dependency: `msw` 3 (fixtures in unit tests).
- New image: `nginxinc/nginx-unprivileged` (stable-alpine, pinned by digest during
  implementation).

**Storage**:
- No server-side storage.
- Browser `sessionStorage` holds only the session hint: session kind, demo key, OIDC
  refresh token. It never holds an access token or ID token.

**Testing**:
- Vitest 5 + Testing Library + MSW (unit and component).
- Playwright 1.64 e2e against the compose stack, for reload, sign-out and account switch, and
  for the claim flow under the production CSP.
- `curl` header checks against the production image.
- A build check that greps `dist/` for fixture markers.

**Target Platform**:
- Evergreen browsers.
- Linux containers: compose locally; the demo deployment (004) for the production image.

**Project Type**: Web application (existing `frontend/` + `backend/`).

**Performance Goals**:
- A reload restores the session in at most one network round-trip (one token request)
  before the first authenticated query.
- The Lighthouse mobile ≥ 90 targets from feature 003 still hold, measured on the fixtures
  build.

**Constraints**:
- No access token or ID token in web storage (FR-002).
- No third-party request on page load other than the identity provider, Stripe and PDOK, and
  only when they are used (SC-005).
- CSP without `'unsafe-inline'` for scripts.
- The Vite dev server stays the local development server; the production image is an
  additional compose target.

**Scale/Scope**:
- About 15 frontend files touched: `auth/*`, `services/*`, `ClaimFlow`, `AddressSearch`,
  `ImageWithFallback`, the pages that use queries, `main.tsx`, `index.html`.
- Two Dockerfiles, one new nginx config template, `docker-compose.yml`.
- No backend code changes.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Crowd-Sourced Verification & Trust**: PASS. Verification badges and ratings are
  unchanged. Only the data source moves from fixtures to the server.
- **II. Waste-Reduction Impact Tracking**: N/A. No impact data is touched.
- **III. Community Collaboration**: PASS. Local discovery is unchanged. PDOK address search
  keeps working and only moves into the service layer.
- **IV. Gamified Engagement**: N/A.
- **V. Low Price, Low Friction**: PASS, and improved. A reload no longer signs the buyer out
  mid-claim (001's "resume the claim" journey). The session restore adds at most one token
  request.
- **Product Constraints (privacy)**: PASS, and improved:
  - Visitor IPs are no longer sent to Google Fonts or to arbitrary photo hosts.
  - Pickup addresses keep their 002 visibility rules, because the server stays the only
    source.
- **Local Development Constraint**: PASS.
  - `docker compose up` still runs everything, and the Vite dev server stays the default
    `frontend` service.
  - The production image is added as a compose service under a `prod` profile (`web`), so it
    is runnable locally as the constitution requires for every service.
  - Binding ports to `127.0.0.1` changes nothing for the developer on their own machine.

No violations. Complexity Tracking not required.

**Post-design re-check (after Phase 1)**: PASS.
- Keeping the refresh token in `sessionStorage` is a deliberate trade-off, recorded in
  research.md R1. The spec only forbids *access* tokens in storage.
- The decision is flagged for the maintainer in the Summary of research.md, because it is the
  one security-relevant choice in this plan.

## Project Structure

### Documentation (this feature)

```text
specs/006-frontend-delivery-hardening/
├── plan.md              # This file
├── research.md          # Phase 0: decisions R1–R9
├── data-model.md        # Phase 1: client-side session, config and error shapes
├── quickstart.md        # Phase 1: validation scenarios
├── contracts/
│   ├── security-headers.md   # Response headers + CSP of the production web image
│   ├── runtime-config.md     # app-config.json served to the browser
│   └── api-client.md         # Frontend API client and error contract
└── tasks.md             # Phase 2 (/speckit-tasks — not created here)
```

### Source Code (repository root)

```text
frontend/
├── Dockerfile                    # Dev server image (unchanged role); stays the compose `frontend`
├── Dockerfile.prod               # NEW: multi-stage build -> nginx-unprivileged, non-root
├── nginx/
│   ├── default.conf.template     # NEW: SPA fallback, /api proxy, security headers (envsubst)
│   └── app-config.json.template  # NEW: runtime config rendered from env at container start
├── index.html                    # Google Fonts links removed
├── src/
│   ├── main.tsx                  # Loads runtime config before render; imports self-hosted font
│   ├── config/
│   │   └── runtimeConfig.ts      # NEW: loads /app-config.json (prod) or import.meta.env (dev)
│   ├── auth/
│   │   ├── AuthProvider.tsx      # Rewritten: one session store, restore on load, clear cache
│   │   ├── sessionStore.ts       # NEW: session hint persistence + token holder (no tokens in storage)
│   │   ├── session.ts            # Session type gains `kind` ('oidc' | 'demo')
│   │   ├── config.ts             # Reads runtime config instead of VITE_* directly
│   │   └── returnTo.ts           # Unchanged (hardening of safeReturnTo tracked separately)
│   ├── services/
│   │   ├── apiClient.ts          # NEW: createApiClient(tokenSource), ApiError, ApiClientContext, useApi()
│   │   ├── api.ts                # Listing/claim/order calls via the client; fixtures removed
│   │   ├── accounts.ts           # Uses the client; expectOk moves to apiClient.ts
│   │   └── geocoding.ts          # NEW: PDOK suggest/lookup (moved from AddressSearch)
│   ├── dev/
│   │   └── fixtures/             # NEW: example listings/orders + fixture fetch handler (dev/test only)
│   ├── components/
│   │   ├── AddressSearch.tsx     # Uses services/geocoding
│   │   ├── ClaimFlow.tsx         # Uses ApiError; publishable key from runtime config
│   │   └── ui/ImageWithFallback.tsx  # Placeholder unless the photo host is allowed
│   └── styles/tokens.css         # --font-sans unchanged (family name stays)
└── tests/
    ├── unit/setup.ts             # Starts MSW with src/dev/fixtures handlers
    └── e2e/
        ├── session-reload.spec.ts        # NEW: reload keeps session, demo->oidc precedence, sign-out clears
        └── prod-csp-claim.spec.ts        # NEW: claim + Stripe flow against the prod image (CSP)

backend/
└── Dockerfile                    # Runtime stage runs as a non-root user

docker-compose.yml                # Ports on 127.0.0.1; `web` service (profile prod) for Dockerfile.prod
```

**Structure Decision**: the existing web-application layout (`frontend/` + `backend/`) is
kept. New code goes into `frontend/src/{config,dev}/`, `frontend/src/services/apiClient.ts`,
`frontend/src/services/geocoding.ts` and `frontend/src/auth/sessionStore.ts`, plus
`frontend/nginx/` for the production image. No new top-level project.

## Complexity Tracking

*No violations — section not applicable.*
