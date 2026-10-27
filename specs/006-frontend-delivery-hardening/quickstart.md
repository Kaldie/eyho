# Quickstart: Frontend & Delivery Hardening

Validation scenarios for spec 006. Run from the repository root.

## Prerequisites

- Docker running. `docker compose down -v && docker compose up` (fresh database with schema
  and demo seed).
- The production web image: `docker compose --profile prod up web` (port 8088, proxies `/api`
  to `backend`).

## S1 — A reload keeps the session (US1, SC-001)

1. Open http://localhost:5173, press **Sign in**, and log in on the mock server: any
   username, with claims
   `{"https://eyho.app/email":"s1@example.nl","https://eyho.app/email_verified":true}`.
2. Open **Account**: the email is shown.
3. Reload the page: the account page is still shown, with no redirect to the mock login.
   DevTools Network shows one `POST /eyho/token` with `grant_type=refresh_token` before
   `GET /api/v1/me`.
4. DevTools → Application → Session Storage → `eyho.session` holds `kind`, `refreshToken`
   and `sub`, and **no** `access_token` or `id_token`. Local Storage has no eyho keys
   (FR-002).

## S2 — Demo then real sign-in; sign-out clears everything (US1, FR-001, SC-002)

1. On `/sign-in`, press a demo account button (for example "Demo buyer"). Open
   **Orders**.
2. Reload: still signed in as the demo buyer. `eyho.session` holds `kind: "demo"`,
   `demoKey`, and no token.
3. Sign in with the mock server as in S1. The account page shows the S1 email, not the demo
   buyer, and `eyho.session.kind` is `"oidc"`.
4. Sign out. Open **Orders**: the sign-in prompt is shown. React Query Devtools (dev build)
   shows an empty cache.

## S3 — Restore failure is a quiet sign-out (Edge Case)

1. Sign in as in S1.
2. Restart the `mock-oidc` container, which forgets its refresh tokens. Reload.
3. Expected: the header shows **Sign in**, no error page appears, and `eyho.session` is
   gone.

## S4 — No example data in the production build (US2, SC-003)

```sh
cd frontend && npm run build
grep -rlE 'mock-listing|mock-order|pi_mock_secret|mockServiceWorker' dist/ && echo FAIL || echo OK
```

Expected: `OK`. `npm run build -- --mode fixtures` still works for Lighthouse and local
use without a backend.

## S5 — Security headers and framing (US3, SC-004)

```sh
curl -sI http://localhost:8088/ | grep -iE 'content-security-policy|x-frame-options|referrer-policy|x-content-type-options'
curl -sI http://localhost:8088/does-not-exist | grep -ic content-security-policy   # 1: also on 404
```

- Open `frontend/tests/e2e/fixtures/frame.html`, which iframes http://localhost:8088/. The
  frame is refused (console: "Refused to frame ... frame-ancestors 'none'").
- `npx playwright test prod-csp-claim` passes the claim and Stripe test-card flow with zero
  CSP violations.

## S6 — No third-party fonts or photos (US3, SC-005)

1. Load http://localhost:8088/ with DevTools → Network, filtered by "Domain".
2. Expected: no `fonts.googleapis.com` or `fonts.gstatic.com`. The font is served from
   `localhost:8088/assets/*.woff2`.
3. With `IMAGE_HOSTS` empty, listings show placeholders and no request goes to any image
   host.

## S7 — Non-root containers and local-only ports (US3, FR-009, FR-012)

```sh
docker compose exec backend id -u        # not 0 (10001)
docker compose --profile prod exec web id -u   # 101
docker compose ps --format '{{.Name}} {{.Ports}}'   # every published port starts with 127.0.0.1:
```

From another machine on the LAN, `nc -vz <dev-ip> 5432` (and 8080, 8081, 5173, 8088) is
refused.

## S8 — One API client (US2, FR-005)

```sh
cd frontend && grep -rn "fetch(" src --include=*.ts --include=*.tsx | grep -v -E 'services/(apiClient|geocoding)\.ts'
```

Expected: no matches. The only direct `fetch` calls are in the API client and the geocoding
service.
