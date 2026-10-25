---
description: "Task list for Post & Claim Surplus Food Listing"
---

# Tasks: Post & Claim Surplus Food Listing

**Input**: Design documents from `/specs/002-surplus-food-listing/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/listings-api.md,
contracts/claims-api.md, quickstart.md

**Tests**: included for the guarantees the spec makes measurable: no overselling (SC-003), impact
records for every completed claim (SC-005), and the verification thresholds (FR-013/FR-014).

**Organization**: grouped by user story (US1–US2 = P1, US3 = P2), plus a phase for the rating →
verification loop, which spans stories.

**Builds on**: feature 001 Phase 1–2 (`specs/001-accounts-sign-in/tasks.md`
T001–T014): accounts, auth middleware, mock sign-in server, test token issuer. Every write
endpoint here uses `RequireVerifiedEmail` and takes the buyer/cook from `AccountFromContext`.

**State of the code (2026-10-25)**: router, handler stubs (501), models, migration 000001 and
sqlc queries exist; sqlc code is not generated; services are stubs; the frontend runs on mock
data in `frontend/src/services/api.ts`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1–US3, or VER for the verification loop
- Paths are relative to the repository root

---

## Phase 1: Setup

- [ ] T001 Run `sqlc generate` for `backend/src/db/sqlc.yaml` into `backend/src/db/generated/`; fix any query/schema mismatches it reports; commit the generated code
- [ ] T002 [P] Create `backend/src/db/pool.go` (pgx pool from config, used by all services) and replace the unused-pool workaround in `backend/main.go`
- [ ] T003 [P] Create `backend/src/services/payments.go`: a `PaymentProvider` interface (`CreateIntent`, `Refund`, `ParseWebhook`) with a Stripe implementation in `backend/src/services/stripe.go`, so feature 004 can add its simulated provider without touching claim logic
- [ ] T004 [P] Create `backend/src/db/seed/local.sql` with Amsterdam listings matching today's frontend mocks (pickup windows relative to `now()`), loaded by the `migrate` step in `docker-compose.yml` for local runs

---

## Phase 2: Foundational

- [ ] T005 Confirm feature 001 Phase 1–2 is merged (accounts, middleware, mock sign-in, test issuer); all stories below use its `RequireVerifiedEmail`/`RequireCook` and `AccountFromContext`
- [ ] T006 Create `backend/src/api/handlers/errors.go`: shared JSON error writer for the contract error bodies (`insufficient_portions`, `own_listing`, `not_collected`, `already_rated`, validation `{error, field}`)
- [ ] T007 Create `backend/src/services/jobs.go`: a once-a-minute ticker started from `backend/main.go` that runs registered jobs with logging and panic recovery (research.md, listing expiry and payment-timeout release)

**Checkpoint**: user stories can start; US1 and US2 can run in parallel.

---

## Phase 3: User Story 1 — Cook posts a surplus listing (P1) 🎯 MVP

**Goal**: any cook can publish; unverified cooks are flagged; listings expire on time.

**Independent Test**: quickstart.md Scenario 1 and the expiry half of Scenario 5.

- [ ] T008 [US1] Implement `listingService.Create` in `backend/src/services/listing_service.go`: cook from account (`RequireCook`), price < retail reference (FR-003), pickup window in the future and ordered, default pickup address from the cook profile when omitted, `published` when `publish: true` regardless of verification (FR-001)
- [ ] T009 [US1] Implement `CreateListing` in `backend/src/api/handlers/listings.go` per contracts/listings-api.md (201/403/422), routed behind `RequireVerifiedEmail` + `RequireCook`
- [ ] T010 [P] [US1] Implement the listing-expiry job in `backend/src/services/listing_expiry_job.go` and register it in `jobs.go` (FR-007)
- [ ] T011 [P] [US1] Unit tests `backend/src/services/listing_service_test.go`: price rule, past window, unverified cook publishes and is flagged
- [ ] T012 [US1] Wire `frontend/src/components/ListingForm.tsx` and `frontend/src/pages/CookDashboardPage.tsx` to `POST /api/v1/listings` and the cook's real listings (replacing mocks)

---

## Phase 4: User Story 2 — Buyer discovers and claims (P1) 🎯 MVP

**Goal**: local discovery, atomic claims, payment, impact records, order history.

**Independent Test**: quickstart.md Scenarios 2–5.

- [ ] T013 [P] [US2] Implement `listingService.List`/`Get` and handlers `ListListings`/`GetListing`: public, buyer's default radius when signed in (else 2 km), non-profits first then distance (research.md), response shape with `rating_count` and `verification_cycle_count`
- [ ] T014 [US2] Implement `claimService.Create` in `backend/src/services/claim_service.go`: one transaction with `DecrementPortionsAtomic` + `CreateClaim`, then `PaymentProvider.CreateIntent`; `409 insufficient_portions`; buyer from account; own-listing guard (`403 own_listing`, feature 001 FR-011) using 001's `AccountOwnsCookProfile`, with test `backend/tests/integration/own_listing_test.go`
- [ ] T015 [US2] Implement `CreateClaim` handler (201/404/409) behind `RequireVerifiedEmail`
- [ ] T016 [US2] Implement `StripeWebhook` in `backend/src/api/handlers/webhooks.go`: verify signature, `payment_intent.succeeded` → `paid` + `confirmed_at` + `ImpactRecord` in one transaction (FR-009); `payment_failed` → release portions (FR-008)
- [ ] T017 [P] [US2] Implement the payment-timeout release job in `backend/src/services/payment_timeout_job.go` using `ReleasePendingClaims` and returning portions (FR-008); register in `jobs.go`
- [ ] T018 [US2] Implement `confirm-pickup` (buyer or listing's cook) and the no-show marking inside the expiry job (FR-012)
- [ ] T019 [P] [US2] Implement `ListClaims`/`GetClaim` handlers with `ListClaimsByBuyer` and keyset cursor (contracts/claims-api.md)
- [ ] T020 [US2] Integration tests `backend/tests/integration/claims_test.go`: 50 concurrent claims on a 1-portion listing produce exactly 1 success (SC-003); timeout release; impact record per paid claim (SC-005)
- [ ] T021 [US2] Replace mocks in `frontend/src/services/api.ts` with real calls (`getListings`, `getListing`, `createClaim`, `getOrders`, `getOrder` via `toOrderSummary`), and point `ClaimFlow.tsx` at the real `stripe_client_secret` and publishable key

---

## Phase 5: Verification loop — ratings drive cook status (FR-013, FR-014)

**Independent Test**: quickstart.md Scenario 7.

- [ ] T022 [VER] Implement `claimService.Rate` in `backend/src/services/claim_service.go`: `RateClaim` query, then in the same transaction recompute `rating_avg`/`rating_count`, update since-transition counters (each buyer rating counts separately, constitution v3.0.0), apply 5-good / 3-bad thresholds, write `VerificationHistory` with triggering claim IDs, increment `verification_cycle_count`, reset counters
- [ ] T023 [VER] Implement `RateClaim` handler (201/403/409/422) behind `RequireAccount`
- [ ] T024 [P] [VER] Integration test `backend/tests/integration/verification_test.go`: Scenario 7 including demotion keeping listings published but flagged
- [ ] T025 [P] [VER] Frontend: rating prompt on collected orders in `frontend/src/pages/OrderHistoryPage.tsx`

---

## Phase 6: User Story 3 — Cook manages an active listing (P2)

**Independent Test**: quickstart.md Scenario 6.

- [ ] T026 [US3] Implement `listingService.Update` and `UpdateListing` handler: owner only, portions not below claimed (409), extend window, cancel → refund paid claims via `PaymentProvider.Refund` and queue a buyer notification (FR-010)
- [ ] T027 [P] [US3] Integration test for cancel-with-refund in `backend/tests/integration/cancel_listing_test.go`
- [ ] T028 [US3] Cook dashboard actions (edit portions, extend, cancel) in `frontend/src/pages/CookDashboardPage.tsx`

---

## Phase 7: Cook payouts (gap — design first)

research.md chooses Stripe Connect Express for paying cooks, but no spec requirement, data-model
field or contract covers connecting a cook's Stripe account. Not needed for the demo (simulated
payments, feature 004). Before a pilot with real money:

- [ ] T029 Add Connect onboarding to the 002 spec/data-model/contracts (cook `stripe_account_id`, onboarding link endpoint, listing publish rule if not connected), then generate tasks for it

---

## Phase 8: Polish

- [ ] T030 [P] Remove the remaining mock data and `TODO` fetch comments from `frontend/src/services/api.ts`; keep `toOrderSummary`
- [ ] T031 Run all quickstart.md scenarios against `docker compose up`; dispatch `qa-engineer`, then `code-reviewer` (Principle I review required for T008, T014, T022)

---

## Dependencies & Execution Order

- Phase 1 → Phase 2 (incl. feature 001 Phase 1–2) → stories.
- US1 and US2 in parallel after Phase 2; T021 needs T013–T019; T022 needs T018 (ratings need
  collected claims); US3 after US2's T014/T016.
- Suggested crew split: `database-engineer` T001, T004; `backend-engineer` T002–T003,
  T006–T011, T013–T020, T022–T024, T026–T027; `frontend-engineer` T012, T021, T025, T028, T030.

## MVP

Phase 1–4 here plus feature 001 Phase 1–3: a signed-in cook publishes, a signed-in buyer
discovers, claims and pays, and the order shows in history.
