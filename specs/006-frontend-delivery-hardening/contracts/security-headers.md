# Contract: Production Web Image Response Headers

Served by `frontend/Dockerfile.prod` (nginx-unprivileged, see research.md R7). Every
response carries these headers, including 404 and 5xx responses (`add_header ... always`).
`${...}` values are rendered from container environment variables at start-up.

## Headers

| Header | Value |
|---|---|
| `Content-Security-Policy` | see below |
| `X-Frame-Options` | `DENY` (legacy browsers; CSP `frame-ancestors` is authoritative) |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `X-Content-Type-Options` | `nosniff` |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=(self), payment=(self "https://js.stripe.com")` |
| `Cross-Origin-Opener-Policy` | `same-origin-allow-popups` (Stripe 3-D Secure may open a popup) |
| `Strict-Transport-Security` | `max-age=31536000` only when `HSTS=true` (TLS terminated in front; off for local `prod` profile) |

## Content-Security-Policy

```text
default-src 'self';
script-src 'self' https://js.stripe.com https://*.js.stripe.com;
style-src 'self';
font-src 'self';
img-src 'self' data: https://*.stripe.com ${IMAGE_HOSTS};
connect-src 'self' https://api.stripe.com ${OIDC_ORIGIN} ${GEOCODING_ORIGIN};
frame-src https://js.stripe.com https://*.js.stripe.com https://hooks.stripe.com;
form-action 'self' ${OIDC_ORIGIN};
frame-ancestors 'none';
base-uri 'self';
object-src 'none'
```

- `${OIDC_ORIGIN}`: scheme + host + port of `OIDC_AUTHORITY` (e.g. `https://eyho.eu.auth0.com`,
  locally `http://localhost:8081`). Needed in `connect-src` for discovery, the token and
  refresh calls, and revocation. Sign-in and sign-out are top-level navigations, which CSP
  does not restrict.
- `${GEOCODING_ORIGIN}`: `https://api.pdok.nl`.
- `${IMAGE_HOSTS}`: space-separated origins from `IMAGE_HOSTS`; empty by default.
- `style-src 'self'` holds: Tailwind compiles to a file, React applies `style={}` through the
  CSSOM (not blocked by CSP), and Stripe Elements styles live inside Stripe's iframe.
- No `'unsafe-inline'` or `'unsafe-eval'` anywhere. `index.html` has no inline script; the
  runtime config is fetched from `/app-config.json`, not inlined.

## Verification

- `curl -sI http://localhost:8088/ | grep -i -E 'content-security|x-frame|referrer|nosniff'`
  shows all four (quickstart S5).
- The Playwright `prod-csp-claim.spec.ts` runs the claim and Stripe test-card flow against
  the `web` service and fails on any `securitypolicyviolation` event.
