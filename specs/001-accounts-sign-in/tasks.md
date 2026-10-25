---
description: "Task list for Accounts & Sign-in"
---

# Tasks: Accounts & Sign-in

**Input**: Design documents from `/specs/001-accounts-sign-in/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/accounts-api.md, quickstart.md

**Tests**: included where the spec's success criteria are security-relevant (SC-003, SC-004) and
for the scenarios in quickstart.md.

**Organization**: grouped by user story (US1–US3 = P1, US4–US5 = P2).

**Roadmap position**: first feature; depends on nothing later. Features 002–004 build on this
one's Phase 2 (accounts, middleware, mock sign-in, test token issuer).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1–US5
- Paths are relative to the repository root

---

## Phase 1: Setup

- [ ] T001 Add `mock-oidc` service (`ghcr.io/navikt/mock-oauth2-server`, pinned version) to `docker-compose.yml` with `infra/mock-oidc/config.json`: interactive login, issuer `eyho`, and claim mappings for `https://eyho.app/email` and `https://eyho.app/email_verified`; wire `OIDC_ISSUER`/`OIDC_AUDIENCE` into the backend service and `VITE_OIDC_*` into the frontend service
- [ ] T002 [P] Add `github.com/coreos/go-oidc/v3` and `github.com/go-jose/go-jose/v4` to `backend/go.mod`; create `backend/src/config/config.go` loading `APP_ENV`, `OIDC_ISSUER`, `OIDC_AUDIENCE`, `DEMO_AUTH_ENABLED`, `AUTH0_MGMT_*`, `KVK_MODE` (`kvk` | `kvk-test` | `fake`), `KVK_API_KEY`; update `backend/.env.example`
- [ ] T003 [P] Add `oidc-client-ts` and `react-oidc-context` to `frontend/package.json`; add `VITE_OIDC_AUTHORITY`, `VITE_OIDC_CLIENT_ID`, `VITE_OIDC_AUDIENCE` to `frontend/.env.example`
- [ ] T004 [P] Write `docs/auth0-setup.md`: EU-region tenant, Google + Username-Password connections, Universal Login password reset, post-login Action adding the namespaced email claims, API identifier, Management API app with `delete:users` and `update:users`, callback/logout URLs for local and the demo address

---

## Phase 2: Foundational (blocks US1–US5)

- [ ] T005 Create `backend/src/db/migrations/000002_accounts.up.sql` / `.down.sql` per data-model.md: `accounts`, `account_identities`, `idp_deletion_requests`; `account_id` on `buyer_profiles` (not null, unique) and `cook_profiles` (nullable, unique); cook `phone`, `default_pickup_address_text`, `default_pickup_location`, `anonymised_at`, `kvk_number`, `kvk_trade_name`, `registration_status` (enum `pending`/`active`/`inactive`), `registration_checked_at`, `nvwa_declared_at`, `nvwa_declaration_version`, with the required-while-not-anonymised check and the 8-digit KvK format check
- [ ] T006 Create `backend/src/db/queries/accounts.sql` (GetAccountByIdentity, GetAccountByVerifiedEmail, CreateAccount, CreateIdentity, UpdateAccountEmailVerified, CreateBuyerProfile, GetCookProfileByAccount) and add it to `backend/src/db/sqlc.yaml`; run `sqlc generate` into `backend/src/db/generated/`
- [ ] T007 [P] Implement `backend/src/auth/verifier.go`: verifies access tokens for a list of trusted issuers via go-oidc (JWKS cached), checks `aud`, extracts `sub`, `iss`, namespaced email and email_verified into a `Claims` struct
- [ ] T008 [P] Implement `backend/src/auth/safeguard.go`: refuse startup when `DEMO_AUTH_ENABLED` is true, `KVK_MODE` isn't `kvk`, or the issuer isn't the configured Auth0 tenant while `APP_ENV` ∉ {`demo`, `local`} (FR-017); unit tests in `backend/src/auth/safeguard_test.go`
- [ ] T009 Implement `backend/src/services/account_service.go` `ResolveAccount(claims)`: existing identity → account; else verified-email match → attach identity (FR-007); else create account + identity + buyer profile; refresh `email_verified`; reject deleted accounts
- [ ] T010 Implement `backend/src/api/middleware/auth.go`: `Authenticate` (optional, sets account in context), `RequireAccount`, `RequireVerifiedEmail`, `RequireCook`, `AccountFromContext`; error bodies per contracts/accounts-api.md
- [ ] T011 Wire config, verifier, safeguard and middleware into `backend/main.go` and `backend/src/api/router.go` (`Authenticate` on all `/api/v1` routes)
- [ ] T012 [P] Create `backend/tests/testutil/tokens.go`: in-process test issuer that mints tokens with chosen `sub`, email and email_verified, for integration tests in both features
- [ ] T013 [P] Implement `frontend/src/auth/AuthProvider.tsx` (react-oidc-context from env config, in-memory tokens, refresh rotation), `/auth/callback` route in `frontend/src/App.tsx`, and bearer-token injection in `frontend/src/services/api.ts`
- [ ] T014 Integration test `backend/tests/integration/auth_test.go`: protected route without token → 401, wrong audience → 401, expired → 401 (SC-003)

**Checkpoint**: backend knows who is calling. Feature 002 can start in parallel from here.

---

## Phase 3: User Story 1 — Browse freely, sign in only to claim (P1) 🎯 MVP

**Goal**: public browsing, sign-in at Claim, return to the same claim.

**Independent Test**: quickstart.md Scenarios 1–2.

- [ ] T015 [P] [US1] Implement `GET /api/v1/me` in `backend/src/api/handlers/me.go`
- [ ] T016 [P] [US1] Create `frontend/src/components/AccountMenu.tsx` (signed out: "Sign in"; signed in: name, Account, Sign out) and use it in the header in `frontend/src/App.tsx` (FR-014)
- [ ] T017 [P] [US1] Create `frontend/src/auth/RequireSignIn.tsx` (short explanation + Sign in) and wrap the Orders and cook routes
- [ ] T018 [US1] In `frontend/src/components/ClaimFlow.tsx`, when signed out, start sign-in with OIDC `state` `{returnTo, listingId, quantity}`; in the callback route, navigate back and reopen the claim dialog with that quantity (FR-005)
- [ ] T019 [US1] Playwright `frontend/tests/e2e/sign-in-to-claim.spec.ts` against the mock server: Scenario 1 and 2

---

## Phase 4: User Story 2 — Sign up and verify email (P1)

**Independent Test**: quickstart.md Scenarios 3–4.

- [ ] T020 [P] [US2] Implement `backend/src/idp/auth0.go`: Management API client with cached token, `SendVerificationEmail(subject)`, `DeleteUser(subject)`; no-op implementation for mock/demo issuers
- [ ] T021 [US2] Implement `POST /api/v1/me/verification-email` in `backend/src/api/handlers/me.go` with the one-per-minute limit
- [ ] T022 [P] [US2] Create `frontend/src/components/VerifyEmailBanner.tsx`: shown when signed in and unverified, with Resend; ClaimFlow and the cook area show it instead of proceeding (FR-006)
- [ ] T023 [US2] Integration test `backend/tests/integration/account_linking_test.go`: Scenario 4, including the unverified no-link case

---

## Phase 5: User Story 3 — Buyer becomes a cook (P1)

**Independent Test**: quickstart.md Scenario 5.

- [ ] T024 [US3] Implement `POST /api/v1/me/cook-profile` and `PATCH /api/v1/me/cook-profile` (handler `backend/src/api/handlers/me.go`, logic in `account_service.go`, queries in `accounts.sql`), validation per contract, new profile `unverified` (FR-009, FR-010); require the NVWA declaration and run the KvK check before saving, returning `201`/`202`/`422` per contract (FR-018, FR-019); a changed `kvk_number` is re-checked
- [ ] T025 [P] [US3] Create `frontend/src/components/AddressSearch.tsx` using the PDOK Locatieserver suggest/lookup API, returning address text and lat/lng
- [ ] T026 [US3] Create `frontend/src/pages/CookOnboardingPage.tsx` (display name, non-profit/professional, phone, AddressSearch, KvK number, NVWA declaration checkbox with versioned wording, clear messages for `kvk_not_found`/`kvk_inactive`/pending); show it in the cook area when the account has no cook profile; replace the mock cook data in `frontend/src/pages/CookDashboardPage.tsx` with `GET /me`
- [ ] T027 [US3] Add `AccountOwnsCookProfile(accountID, cookProfileID)` to `backend/src/services/account_service.go` with unit tests; feature 002 uses it for the own-listing guard (FR-011) when it builds claim creation
- [ ] T038 [P] [US3] Implement `backend/src/registry/`: `BusinessRegister` interface (`Check(kvkNumber) → active | inactive | not_found, tradeName`, transport errors separate) with `kvk.go` (Zoeken v2, two-step inactive lookup per research.md, used for both `kvk` and `kvk-test` base URLs) and `fake.go` (fixed active and inactive test numbers matching the demo seed); unit tests with `httptest` in `backend/src/registry/kvk_test.go`
- [ ] T039 [US3] Implement `backend/src/registry/recheck_job.go`: retries `pending` profiles with backoff and re-checks profiles not checked for 30 days; transport errors never downgrade an `active` profile (FR-021); start it from `backend/main.go`
- [ ] T040 [US3] Add `CookCanPublish(cookProfileID)` to `backend/src/services/account_service.go` (`registration_status = active` and declaration on record) and a `cook_not_registered` error body helper in `backend/src/api/middleware/auth.go`; feature 002's listing service calls it before publishing (FR-020); unit tests
- [ ] T041 [P] [US3] Add a registration status card to the cook area in `frontend/src/pages/CookDashboardPage.tsx` ("Registered" with trade name / "Checking your registration" / "Registration inactive" with what to do), visually distinct from the verified badge (FR-022)
- [ ] T042 [US3] Integration test `backend/tests/integration/registration_gate_test.go`: quickstart Scenario 5 steps 2–4 and Scenario 8, using the fake register (SC-007)

---

## Phase 6: User Story 4 — Delete account (P2)

**Independent Test**: quickstart.md Scenario 6.

- [ ] T028 [US4] Implement deletion in `backend/src/services/account_service.go`: blocking checks (`active_claims`, `active_listings`), one transaction that nulls personal fields (including `kvk_number` and `kvk_trade_name`), sets "Former cook" and `anonymised_at`, deletes identities, sets `deleted_at`, enqueues `idp_deletion_requests`; `DELETE /api/v1/me` handler
- [ ] T029 [P] [US4] Background worker `backend/src/services/idp_deletion_worker.go` draining `idp_deletion_requests` with backoff via `idp.DeleteUser`
- [ ] T030 [P] [US4] Create `frontend/src/pages/AccountPage.tsx`: display name edit, sign out, delete account with confirmation and the 409 explanations
- [ ] T031 [US4] Integration test `backend/tests/integration/account_deletion_test.go`: Scenario 6, asserting no personal data remains and all verification history rows remain (SC-004)

---

## Phase 7: User Story 5 — Demo and local one-tap sign-in (P2)

**Independent Test**: quickstart.md Scenario 7.

- [ ] T032 [US5] Implement `backend/src/auth/demo_issuer.go` (key generation at startup, discovery, JWKS, token minting) and `backend/src/api/handlers/demo_auth.go` (`GET /demo/accounts`, `POST /demo/sign-in`), routed only when allowed by the safeguard; add the demo issuer to the verifier's trusted list in demo/local
- [ ] T033 [P] [US5] Seed the three demo accounts in `backend/src/db/seed/demo_accounts.sql` (`is_demo = true`, cook profiles in Amsterdam, one verified and one unverified, both `registration_status = active` with KvK numbers from KVK's test set and a recorded declaration); feature 004 extends this seed with listings
- [ ] T034 [US5] Create `frontend/src/pages/SignInPage.tsx`: "Continue" (OIDC redirect) plus one-tap demo buttons when `/api/v1/demo/accounts` responds; `AuthProvider` accepts a demo token as the session
- [ ] T035 [US5] Tests: safeguard start-up failure (`backend/src/auth/safeguard_test.go`), `/demo/*` 404 in production config, Playwright one-tap flow in `frontend/tests/e2e/demo-sign-in.spec.ts`

---

## Phase 8: Polish

- [ ] T036 [P] Update `README`/docs: local sign-in with the mock server, demo accounts, Auth0 setup link, the fake business register's test KvK numbers, and how to get a KVK API key for production
- [ ] T037 Run all quickstart.md scenarios against `docker compose up`; dispatch `qa-engineer`, then `code-reviewer` (Principle I / Product Constraints review required for T027, T028, T038–T040)

---

## Dependencies & Execution Order

- Phase 1 → Phase 2 → user stories.
- US1 needs T015–T018 in order T015 → T018. US2–US5 can run in parallel after Phase 2.
- In US3, T038 comes before T024 and T039; T040 before feature 002's T008.
- Suggested crew split: `database-engineer` T005–T006, T033; `backend-engineer` T007–T012,
  T014–T015, T020–T021, T023–T024, T027–T029, T031–T032, T035, T038–T040, T042; `frontend-engineer` T013,
  T016–T019, T022, T025–T026, T030, T034, T041.

## MVP

Phase 1–3: people can sign up, sign in and come back to what they were doing. Combined with
feature 002's US1–US2 this becomes the product MVP.
