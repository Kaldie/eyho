# Phase 0 Research: Accounts & Sign-in

## Identity provider: Auth0 (free plan)

**Decision**: Auth0 Universal Login with two connections: Google (social) and a Username-Password
database connection. Apple is added later as a third connection with no code change.

**Rationale**: chosen by the maintainer. The free plan covers pilot scale; Actions (used below)
are available on it; and tokens for Auth0's own Management API do not count towards the
1,000-per-month machine-to-machine token quota, so the backend can call it for deletion and
verification emails at no cost.

**To confirm before go-live**: tenant region (pick an EU region for GDPR), current free-plan MAU
limit and branding limits on the free plan's Universal Login.

## Frontend sign-in library: generic OIDC, not the Auth0 SDK

**Decision**: `oidc-client-ts` with `react-oidc-context` (Authorization Code flow with PKCE),
configured from the issuer's `/.well-known/openid-configuration`.

**Rationale**: the same code must talk to Auth0 (demo) and to a mock OIDC server (local, FR-015).
Auth0's own SPA SDK hard-codes Auth0 endpoint paths and would not work against a generic mock.
A standards-based client keeps local and demo identical except for configuration.

**Alternatives considered**: `@auth0/auth0-react` (rejected: Auth0-only); a hand-rolled redirect
flow (rejected: PKCE, silent renew and state handling are easy to get wrong).

**Session**: access tokens kept in memory; refresh via refresh-token rotation (Auth0 "rotating
refresh tokens", absolute lifetime 30 days). Nothing stored in `localStorage` except the
library's transient sign-in state.

**Resume after sign-in (FR-005)**: the claim intent (`listingId`, `quantity`) is passed in the
OIDC `state` and restored on the callback route, which reopens the claim dialog.

## Backend token verification

**Decision**: `github.com/coreos/go-oidc/v3` verifies RS256 access tokens against the issuer's
JWKS (cached), checking `iss`, `aud` (the eyho API identifier) and `exp`. An Auth0 post-login
Action adds namespaced claims `https://eyho.app/email` and `https://eyho.app/email_verified` to
access tokens, because Auth0 access tokens don't carry email by default. The mock server is
configured to emit the same claims.

**Alternatives considered**: calling the provider's userinfo endpoint per request (rejected:
latency and rate limits); opaque tokens (rejected: need introspection).

## Accounts and account linking (FR-007)

**Decision**: link in our own database, not in Auth0. `accounts` holds one row per person with a
unique email; `account_identities` maps each `(issuer, subject)` to an account. On first sign-in
with a new identity whose email is verified and matches an existing account's verified email,
the identity is attached to that account. Unverified emails never link.

**Rationale**: provider-independent (works the same with the mock server and later with Apple),
no Management API calls on the login path, and easy to test.

**Alternatives considered**: Auth0 account-linking Action plus Management API (rejected: Auth0-only
and harder to test locally).

## Email verification and resend (FR-006)

**Decision**: Auth0 sends verification emails for database-connection sign-ups. "Resend" calls
`POST /api/v1/me/verification-email`, which uses the Management API job endpoint. Google
identities arrive with `email_verified = true`. The backend reads verification from the token
claim and caches it on the account; the frontend refreshes the token after the user returns from
the verification link.

## Account deletion (FR-013)

**Decision**: one database transaction anonymises the account (email, display name, phone,
addresses set to null or "Former cook"; identities deleted; `deleted_at` set), then the backend
deletes the user in Auth0 via the Management API. If the Auth0 call fails, a retry job keeps
trying; the local identity rows are already gone, so the old login can't reach the old account
either way.

**Blocking rule**: deletion returns `409` while the user has claims with
`pickup_status = pending` on paid claims, or published listings that have such claims.

## Local development: mock OIDC server (FR-015)

**Decision**: `ghcr.io/navikt/mock-oauth2-server` as a `docker compose` service, configured with
an interactive login page and claim mappings that mirror the Auth0 Action's namespaced claims.

**Rationale**: standards-compliant issuer with discovery and JWKS, so frontend and backend run
their production code paths; no account or internet needed (constitution, Local Development
Constraint).

## Demo one-tap accounts (FR-016, FR-017)

**Decision**: a small demo issuer inside the Go backend, enabled only when `APP_ENV` is `demo` or
`local`. It signs short-lived tokens for the seeded demo accounts
(`POST /api/v1/demo/sign-in`) and publishes its own JWKS. In those environments the backend
trusts two issuers: the real one (Auth0 or the mock) and the demo issuer. The frontend's
sign-in screen shows one-tap buttons only when `/api/v1/demo/accounts` responds.

**Rationale**: the demo runs on a single free backend instance (feature 004), so no extra
service; Auth0's password grant for seeded users is discouraged and off by default.

**Safeguard**: startup fails if the demo issuer, or a non-Auth0 issuer, is configured while
`APP_ENV` is anything other than `demo` or `local`.

## Turning the pickup address into coordinates

**Decision**: cook onboarding uses the PDOK Locatieserver (the Dutch government's free address
search, no API key) for address autocomplete; the selected suggestion supplies both the address
text and its coordinates. Called from the browser; nothing stored at PDOK.

**Rationale**: discovery needs a point for radius queries (002 FR-011); typing coordinates is not
an option; PDOK covers every Dutch address and costs nothing.

**Alternatives considered**: Google Places / Mapbox (keys, billing, consent banner); manual pin on
a map (slower onboarding, SC-002).

## Checking business registration (FR-018 to FR-023)

**Decision**: check the cook's KvK number with the KVK Handelsregister **Zoeken API v2**
(`GET https://api.kvk.nl/api/v2/zoeken?kvkNummer=...`) from the backend, behind a small
`registry.BusinessRegister` interface with three implementations:
- `kvk` — production, paid API key (about €6.20 per month; Zoeken queries are free, Basisprofiel
  is €0.02 per query and is not needed).
- `kvk-test` — demo, KVK's public test environment (`https://api.kvk.nl/test/...`) with KVK's
  published test key; fictitious companies only, €0.
- `fake` — local, an in-memory list of fixed test numbers (active and inactive); no internet.

Zoeken leaves out inactive registrations unless `inclusiefInactieveRegistraties=true` is passed.
The check sends the number twice in the worst case: first without the flag (a hit means
**active**), then, on no hit, with the flag (a hit means **inactive**, no hit means **not found**).
The trade name comes from the first result's `naam`.

Re-checks (FR-021): a background job re-checks profiles whose `registration_checked_at` is older
than 30 days, and retries `pending` profiles with backoff. A transport error never changes an
`active` status; only a definite "inactive" or "not found" answer does.

**NVWA**: no public lookup of registered (non-accredited) food businesses was found, so FR-019 is a
declaration, stored with time and wording version. NVWA registration now runs through MijnNVWA
and requires a KvK registration, which is why the KvK check is the meaningful automatic filter.

**Rationale**: free per query, instant (SC-008), no human review needed (maintainer decision
2026-10-25), and the interface keeps local development offline (Local Development Constraint).

**Alternatives considered**: upload of NVWA proof reviewed by an admin (needs an admin role and
slows onboarding; deferred); Basisprofiel for richer data (costs per query, not needed);
matching pickup addresses to KvK branches via Vestigingsprofiel (rejected by the maintainer:
any pickup address is allowed).

**To confirm in implementation**: which numbers in KVK's test set are active, for the demo seed
and the fake register.

## Outcome

No NEEDS CLARIFICATION remain. Open confirmations (Auth0 region and current free-plan limits)
are listed above and in quickstart.md prerequisites.
