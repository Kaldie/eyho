# Phase 0 Research: Frontend & Delivery Hardening

**Decision for the maintainer to confirm**: R1 keeps the OIDC *refresh* token in the tab's
`sessionStorage` so the session survives a reload. It is the one security trade-off in this
plan; the alternatives and why they lose are below.

## R1. A session that survives a reload without access tokens in web storage

**Today**:
- `AuthProvider.tsx` keeps the whole oidc-client-ts `User` in an `InMemoryWebStorage`, so a
  reload loses it.
- `offline_access` and `automaticSilentRenew` are set, but only help while the page stays
  open.
- The demo token is stored in `sessionStorage` (`eyho.demo-session`).

**Decision**:
- Persist a *session hint* in `sessionStorage` (key `eyho.session`, shape in data-model.md):
  - `{ kind: 'oidc', refreshToken }`, or
  - `{ kind: 'demo', demoKey }`.
- Access and ID tokens stay in memory only.
- On load, `AuthProvider` restores the session before rendering authenticated queries:
  - **OIDC**: a refresh-token grant (`UserManager.signinSilent()` with the restored refresh
    token). The rotated refresh token replaces the stored one.
  - **Demo**: `POST /api/v1/demo/sign-in {key}` re-mints a token. The demo key is a public,
    seeded account name, not a secret.
- If restore fails (`invalid_grant`, expired refresh token, demo key gone), the hint is
  deleted and the person is shown as signed out. This is the spec's "renewal fails" edge
  case: no error page.
- Implementation: a custom oidc-client-ts `userStore` that persists only `refresh_token`,
  `profile.sub`, `scope` and `expires_at`, stripping `access_token` and `id_token` before
  writing. Restore then needs only a token request, not a redirect.
- Auth0 settings (documented in `docs/auth0-setup.md`):
  - Refresh Token Rotation on, with reuse detection.
  - Inactivity lifetime 12 h, absolute lifetime 7 days.
  - Allowed Web Origins set to the app origin.
  - `revokeTokensOnSignout: true` on the client, so sign-out revokes the refresh token.

**Rationale**:
- Works the same against Auth0 and the local mock server. Verified on 2026-10-26 against
  mock-oauth2-server 6.1.3: the `refresh_token` grant returns a new access token that keeps
  `aud: eyho-api` and the `https://eyho.app/email*` claims.
- Keeps 001's research decision ("refresh via refresh-token rotation").
- Satisfies FR-002 as written: no *access* token in storage.
- `sessionStorage` is tab-scoped and cleared when the tab closes. Rotation plus reuse
  detection makes a stolen refresh token single-use, and the US3 CSP is the main XSS
  defence.

**Alternatives considered**:
- *Silent authentication in a hidden iframe (`prompt=none`)*: no token at all in storage.
  - Rejected for now. mock-oauth2-server 6.1.3 ignores `prompt=none` and keeps no login
    session (tested: it always returns its login form), so local reloads would still sign
    out.
  - Against Auth0 it needs a custom domain on the app's site, because Safari and Firefox
    block the third-party cookie otherwise.
  - Worth revisiting when 004 sets up a custom domain. The session store hides the
    mechanism, so switching later is local to `sessionStore.ts`.
- *Top-level `prompt=none` redirect on every load*: one full redirect per reload, and it
  shows the mock's login form locally. Rejected.
- *Backend-for-frontend (tokens in an httpOnly cookie held by the Go backend)*: strongest
  against XSS. Rejected for this feature: it is a redesign of 001's auth model (sessions,
  CSRF, a cookie domain shared with the API) and conflicts with "local and demo identical
  except config". Revisit if the app ever stores payment-sensitive state client-side.
- *DPoP-bound tokens*: oidc-client-ts supports DPoP, but Auth0 support and the mock's
  support are not confirmed. Recorded as a follow-up.
- *Whole `User` in `sessionStorage`*: puts the access token in storage. Violates FR-002.

**Known limitation**: "Duplicate tab" copies `sessionStorage`. Two tabs then share one
refresh token, and rotation-reuse detection signs both out on the second refresh. This is
accepted, and handled as the "renewal fails, shown as signed out" path. Tabs opened from
links start with their own empty `sessionStorage` and simply sign in again silently, so they
are unaffected.

## R2. One session state and demo/OIDC precedence

**Decision**:
- `AuthProvider` owns one `SessionStore` with `status` (`loading | signedOut | signedIn`),
  `kind` (`oidc | demo | null`) and the in-memory access token.
- Completing an OIDC sign-in clears any demo hint and token. Demo sign-in calls
  `removeUser()` on the OIDC manager and stores the demo hint. FR-001: the last sign-in wins;
  today the demo token silently wins.
- Each of these clears the query cache with `queryClient.clear()` (FR-003, SC-002): sign-in,
  sign-out, a change of `kind`, and a change of the token subject (`sub`) during renewal.
  Today only `['me']` and `['orders']` are removed.

**Rationale**: one owner, one rule, and the cache is keyed to the person by being emptied
whenever the person changes.

**Alternatives considered**: putting the subject in every query key. Rejected: easy to forget
on a new query; clearing is total.

## R3. Getting the token to the API layer without a render-time global

**Today**: `SessionProvider` calls `setAccessTokenGetter(...)` during render (module-level
global in `services/api.ts`).

**Decision**:
- `services/apiClient.ts` exports `createApiClient(getToken)` and an `ApiClientContext`.
- `AuthProvider` creates the client once, with a getter that reads the session store's
  current token, and provides it.
- Data functions take the client as their first argument (`listListings(api, params)`).
  Hooks get it through `useApi()`.
- Queries that need a signed-in person are `enabled: status === 'signedIn'`. So no query runs
  while `status === 'loading'`, and there is no race on first render.

**Alternatives considered**:
- Passing the token into every hook. Rejected: noisy, and leaks auth into pages.
- Keeping the global but setting it in an effect. Rejected: first-render queries would run
  without a token.

## R4. One API client and one error shape

**Decision**:
- `apiFetch` (moved to `apiClient.ts`) is the only path to the eyho server.
- It throws `ApiError(status, body)` for non-2xx responses, using `expectOk` (moved from
  `accounts.ts`).
- It parses the server's single error shape from feature 005: `{ "error": code, ...details }`.
- `ClaimApiError` is deleted. `ClaimFlow` checks `err instanceof ApiError && err.status >= 400
  && err.status < 500` for the "no longer available" path. A network failure stays a
  non-`ApiError` `Error`, keeping 003's distinct "connection" message.
- All `fetch(...)` TODO bodies in `services/api.ts` are rewritten through the client. They no
  longer call bare `fetch` without the bearer token.

**Overlap with feature 002**: 002's T021 ("replace mocks in services/api.ts") and T012
(dashboard listings) do the same wiring for the endpoints 002 implements.
- Whichever lands first introduces `apiClient.ts`; the other only adds calls through it.
- If 006 lands first, its US2 calls the still-stubbed endpoints. Locally they answer 501, so
  local development uses fixtures mode (R5) until 002 ships.

## R5. Example data out of the production build

**Decision**:
- Move `MOCK_LISTINGS`, `MOCK_ORDERS`, the mock claim behaviour (`network-error-listing`,
  `expired-listing`) and the cook dashboard's `MOCK_DASHBOARD_LISTINGS` to
  `src/dev/fixtures/`.
- The fixtures are exposed as MSW request handlers for the eyho API routes.
- **Unit and component tests**: `tests/unit/setup.ts` starts an MSW node server with these
  handlers (MSW 3).
- **Local development without a backend, and Lighthouse CI**: build mode `fixtures`
  (`vite --mode fixtures`, `vite build --mode fixtures`).
  - `.env.fixtures` sets `VITE_API_FIXTURES=true`.
  - `main.tsx` does
    `if (import.meta.env.VITE_API_FIXTURES === 'true') await import('./dev/fixtures/browser')`,
    which starts the MSW service worker.
  - Vite replaces the condition statically, so in a production build the dynamic import is
    dead code and no fixture chunk is emitted.
  - MSW's `mockServiceWorker.js` lives in `frontend/dev-public/` (served via `publicDir` only
    in fixtures mode), not in `public/`.
- `lighthouserc.json` builds with `--mode fixtures` (it currently points at
  `mock-listing-001`).
- **Build check** (in 005's pipeline):
  `vite build && ! grep -rqE 'mock-listing|mock-order|pi_mock_secret|mockServiceWorker' dist/`
  (SC-003).

**Rationale**:
- One set of fixtures for tests, local and Lighthouse.
- Production builds provably do not contain it.
- MSW intercepts at the network layer, so the real client code (headers, errors) is
  exercised.

**Alternatives considered**:
- Keep mocks inside the data functions behind `if (import.meta.env.DEV)`. Rejected: mixes
  fixtures into production modules, and tests then skip the real client.
- A separate fixture backend container. Rejected: more moving parts than MSW.

## R6. Address search in the service layer

**Decision**:
- Create `services/geocoding.ts` with `suggestAddresses(query, signal)` and
  `lookupAddress(id)`.
- These hold the PDOK base URL (`https://api.pdok.nl/bzk/locatieserver/search/v3_1`, from
  runtime config `geocodingBaseUrl`) and the WKT centroid parsing now in `AddressSearch.tsx`.
- `AddressSearch` keeps its UI and the `baseUrl` prop for tests, and calls the service.
- PDOK is listed in CSP `connect-src`.
- PDOK calls do not carry the eyho bearer token. They use plain `fetch` inside the service,
  which is the only allowed non-eyho call site.

## R7. Production web image, headers and runtime configuration

**Decision**:
- `frontend/Dockerfile.prod`:
  1. `node:24-alpine` runs `npm ci && npm run build` (production mode).
  2. `nginxinc/nginx-unprivileged:stable-alpine` (UID 101, listens on 8080; pinned by digest
     when implemented) serves `dist/`.
- `nginx/default.conf.template`, rendered by the image's built-in envsubst step at start:
  - SPA fallback: `try_files $uri /index.html`.
  - Long-cache `assets/` (hashed), and `no-cache` for `index.html` and `app-config.json`.
  - `location /api/ { proxy_pass ${BACKEND_ORIGIN}; }`, so the browser only talks to its own
    origin.
  - The headers in [contracts/security-headers.md](./contracts/security-headers.md), with
    `always` so they are also on error responses.
- `nginx/app-config.json.template` → `/app-config.json`, rendered from env:
  `OIDC_AUTHORITY`, `OIDC_CLIENT_ID`, `OIDC_AUDIENCE`, `STRIPE_PUBLISHABLE_KEY`,
  `IMAGE_HOSTS`, `GEOCODING_BASE_URL`, `DEMO_SIGN_IN`.
- `main.tsx` fetches `/app-config.json` before rendering in production builds. In dev mode,
  `runtimeConfig.ts` reads `import.meta.env.VITE_*` as today.
- One image serves local `prod` profile, demo and production; the identity provider origin
  and image hosts reach both the CSP and the app from the same environment variables.

**Rationale**:
- Build once, configure per environment.
- The CSP must name the identity provider origin, so it has to be set at deploy time, not
  baked in at build time.
- nginx-unprivileged runs as non-root out of the box (FR-008).

**Alternatives considered**:
- Caddy: equally capable, but nginx's envsubst-templates entrypoint does the runtime
  rendering without custom scripts.
- Build-time `VITE_*` config per environment: needs a rebuild per environment, and the CSP
  header could drift from the baked-in config.
- An inline `<script>window.__CONFIG__</script>`: would need a CSP hash or nonce.

## R8. Backend image runs as non-root; local ports

**Decision**:
- `backend/Dockerfile` runtime stage on `alpine:3.22`:
  `RUN adduser -D -H -u 10001 eyho` and `USER 10001:10001`.
- Alpine stays, because compose's healthcheck runs `wget` inside the container.
- `docker-compose.yml` publishes every port as `127.0.0.1:<host>:<container>`: db 5432,
  mock-oidc 8081, backend 8080, frontend 5173, and the new `web` 8088.
- The `e2e` service shares mock-oidc's network namespace and is unaffected.

**Alternatives considered**: distroless `static:nonroot`. Rejected for now: it has no shell
or `wget` for the healthcheck. It would need a `-healthcheck` mode in the Go binary;
revisit with 004.

## R9. Self-hosted font and allowed photo hosts

**Decision (font)**:
- `@fontsource-variable/bricolage-grotesque` (5.3.0, OFL), imported in `main.tsx`. This is
  the family `index.html` now loads from Google Fonts with `opsz 12..96`, `wght 400..800`.
- Vite bundles the woff2 files into `assets/`, so `font-src 'self'` covers them.
- Remove the `fonts.googleapis.com` and `fonts.gstatic.com` links from `index.html`.
- `--font-sans` in `tokens.css` keeps the family name (the variable package registers it as
  `Bricolage Grotesque Variable`, so the token gains that name first).
- **To confirm in implementation**: that the package's default import includes the `opsz`
  axis; otherwise import its full-axes stylesheet.

**Decision (photos)**:
- `ImageWithFallback` renders the placeholder unless `src` is an `https:` URL whose origin is
  in runtime config `imageHosts` (empty by default, so every listing shows the placeholder
  until 002/004 pick a host).
- CSP `img-src` lists the same origins.
- Rejecting other hosts on save is 002's listing validation (spec FR-011 note).

**Rationale**: SC-005 means no third-party request on load. The CSP enforces this even if a
component forgets the check.
