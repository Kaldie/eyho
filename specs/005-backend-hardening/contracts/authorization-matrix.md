# Contract: Authorization Matrix for Listings and Claims

Who may call each route that names a listing or claim, and what everyone else gets (US1,
FR-001 to FR-004). "Owner" and "participant" are always decided from the server's records via
the caller's own profile ids (research.md), never from request input.

The sign-in gates from 001 run first, in this order:
1. `401 unauthenticated` / `403 account_deleted` / `503 identity_provider_unavailable`.
2. `403 email_not_verified` / `403 cook_profile_required` where listed in 002's contracts.
3. The ownership check below.

| Route | Allowed caller | Capability (research.md) | Anyone else signed in | Signed out |
|---|---|---|---|---|
| `GET /listings/{id}` | anyone for a `published` listing; only the owning cook for `draft`, `expired`, `cancelled` | `VisibleListing` | `404 not_found` for a non-published listing | same as "anyone else" |
| `PATCH /listings/{id}` | owning cook (`listings.cook_id = caller.CookProfileID`) | `OwnedListing` | `404 not_found` | `401 unauthenticated` |
| `GET /claims/{id}` | the claim's buyer, or the cook of its listing | `VisibleClaim` | `404 not_found` | `401 unauthenticated` |
| `POST /claims/{id}/confirm-pickup` | the claim's buyer, or the cook of its listing (002 contract) | `VisibleClaim` | `404 not_found` | `401 unauthenticated` |
| `POST /claims/{id}/rating` | the claim's buyer only | `BuyerClaim` | `404 not_found` (this includes the listing's cook, FR-002) | `401 unauthenticated` |
| ~~`PATCH /claims/{id}`~~ | removed: in no contract, duplicated confirm-pickup | | `404` (route gone) | `404` |

Routes without an `{id}` are scoped by the caller, not checked per item:
- `GET /claims` lists only the caller's own claims (`buyer_id = caller.BuyerProfileID`).
- `POST /claims` keeps 002's `403 own_listing` guard.
- `POST /listings` uses the caller's own cook profile.

## Changes to feature 002's contracts

Applied to `specs/002-surplus-food-listing/contracts/*.md` in this feature's PR:

| Contract line | Before | After |
|---|---|---|
| listings-api.md, `PATCH /listings/{id}` | `403 Forbidden — requester is not the owning cook` | `404 not_found — the listing does not exist or the requester is not its cook` |
| claims-api.md, `POST /claims/{id}/rating` | `403 Forbidden — requester is not the claim's buyer` | `404 not_found — the claim does not exist or the requester is not its buyer` |
| claims-api.md, `GET /claims/{id}` | already `404` for non-participants | unchanged |

## Test obligation (FR-004, SC-001)

Every row with an "Allowed caller" has a case in `backend/tests/integration/ownership_test.go`
using `testutil.AssertOtherUserRefused`:
1. The other signed-in user gets `404 {"error":"not_found"}`.
2. The item is unchanged in the database.
3. The allowed caller does not get 404.

`TestEveryResourceRouteHasOwnershipCase` walks the router and fails for any `/listings/{id}…`
or `/claims/{id}…` route that has no case.
