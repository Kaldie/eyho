# Quickstart: Accounts & Sign-in

Validation guide for feature 004. Runs against the Docker Compose stack (constitution, Local
Development Constraint) unless a step says "demo".

## Prerequisites

- `docker compose up` with no provider credentials: includes the `mock-oidc` service.
- For demo checks: an Auth0 tenant (EU region) with Google and Username-Password connections,
  the post-login Action adding `https://eyho.app/email` and `https://eyho.app/email_verified`,
  and a Management API application with `delete:users` and `update:users` scopes.

## Scenario 1: Browse signed out, sign in to claim (US1, FR-003, FR-005)

1. Open the app signed out; discovery and a listing detail load with no sign-in prompt.
2. Press Claim, choose 2 portions. Expect the sign-in page.
3. Sign in on the mock server as a new user with a verified email.
4. Expect to land back on the same listing with the claim dialog open at 2 portions.

## Scenario 2: Signed-out access to protected pages (US1)

1. Signed out, open Orders and the cook area. Expect an explanation and a "Sign in" action,
   no data.
2. `GET /api/v1/claims` without a token → `401 unauthenticated`.

## Scenario 3: Email verification gate (US2, FR-006)

1. Sign in as a mock user with `email_verified = false`.
2. Try to claim → `403 email_not_verified`; UI shows a resend action.
3. `POST /api/v1/me/verification-email` → `202`.
4. Sign in again as the same user with `email_verified = true`; claim succeeds.

## Scenario 4: Account linking (US2, FR-007)

1. Sign in with identity A (`sub=a`, email `x@example.nl`, verified).
2. Sign in with identity B (`sub=b`, same email, verified).
3. `GET /api/v1/me` returns the same account `id` both times.
4. Repeat with B unverified → a separate account is created; no linking.

## Scenario 5: Cook onboarding and own-listing guard (US3, FR-009, FR-011)

1. As a verified buyer, `POST /api/v1/me/cook-profile` missing `phone` → `422 missing_field`.
2. Submit complete onboarding → `201`, `verification_status: "unverified"`.
3. Publish a listing without a pickup address → it uses the default pickup address.
4. Claim that listing from the same account → `403 own_listing`.
5. Claim another cook's listing from the same account → `201`.

## Scenario 6: Account deletion (US4, FR-013)

1. As a cook with a paid claim awaiting pickup on one of their listings, `DELETE /api/v1/me` →
   `409 active_listings`.
2. Mark the claim collected, then `DELETE /api/v1/me` → `204`.
3. Verify: account email/name null, identities gone, cook profile shows "Former cook" with
   phone and addresses null, all `verification_history` rows still present.
4. Sign in again with the same identity → a new, empty account.

## Scenario 7: Demo one-tap sign-in and safeguard (US5, FR-016, FR-017)

1. Locally, `GET /api/v1/demo/accounts` lists three accounts; one-tap as "Demo cook (verified)"
   and publish a listing; one-tap as "Demo buyer" and claim it.
2. Start the backend with `APP_ENV=production` and the demo issuer enabled → the process exits
   with an error naming the setting.
3. Start with `APP_ENV=production` and no demo settings → `/api/v1/demo/*` returns `404`.

## Out of scope

Apple sign-in, SMS verification, two-factor authentication, admin accounts.
