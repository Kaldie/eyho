# Phase 1 Data Model: Post & Claim Surplus Food Listing

## CookProfile

Represents a non-profit or professional cook who may publish listings. Verification is
crowd-sourced and reputation-driven (Constitution Principle I, v2.0.0), not a manual upfront gate.

| Field | Type | Notes |
|---|---|---|
| id | UUID | Primary key |
| display_name | text | |
| org_type | enum(`nonprofit`, `professional`) | Drives any visibility rules (Principle III) |
| verification_status | enum(`unverified`, `verified`) | Does NOT gate publishing (FR-001); drives the "unverified" flag shown to buyers (FR-004) |
| good_ratings_since_transition | integer | Count of recent listings rated ≥4 stars since the last status change; resets on transition. Reaching **5** promotes `unverified` → `verified` |
| bad_ratings_since_transition | integer | Count of recent listings rated ≤2 stars since the last status change; resets on transition. Reaching **3** demotes `verified` → `unverified` |
| verification_cycle_count | integer | Total number of times this cook has transitioned between `verified` and `unverified`, in either direction |
| rating_avg | numeric(3,2) | Aggregate rating, denormalized for read performance |
| rating_count | integer | |
| created_at / updated_at | timestamptz | |

**VerificationHistory** (new entity, append-only, one row per transition):

| Field | Type | Notes |
|---|---|---|
| id | UUID | Primary key |
| cook_id | UUID (FK → CookProfile) | |
| from_status | enum(`unverified`, `verified`) | |
| to_status | enum(`unverified`, `verified`) | |
| triggering_listing_ids | UUID[] | The listings/ratings that produced this transition, for auditability |
| created_at | timestamptz | Transition timestamp |

**Validation rules**:
- Publishing a listing does NOT require `verification_status = verified` (FR-001); it only
  determines whether the listing/profile is flagged "unverified" to buyers.
- A "listing rating" for the thresholds below is derived from the buyer ratings on that
  listing's claims. [NEEDS CLARIFICATION: when a listing has several claims, does each buyer
  rating count separately, or does the listing count once using the average of its ratings? And
  when is a listing's rating final, since buyers can rate after the listing expires?]
- On each new listing rating, the system MUST update `good_ratings_since_transition` /
  `bad_ratings_since_transition` and evaluate the promotion/demotion thresholds (5 good → verified, 3
  bad → unverified). A transition MUST write a `VerificationHistory` row and increment
  `verification_cycle_count`.
- If a cook reverts from `verified` to `unverified` while holding active (`published`) listings,
  those listings remain visible but MUST be immediately re-flagged as "unverified" (Edge Case in
  spec) — they are NOT cancelled or removed.

## BuyerProfile

| Field | Type | Notes |
|---|---|---|
| id | UUID | Primary key |
| default_discovery_radius_km | numeric | Default local radius (FR-011); reasonable default ~3km |
| created_at / updated_at | timestamptz | |

## Listing

| Field | Type | Notes |
|---|---|---|
| id | UUID | Primary key |
| cook_id | UUID (FK → CookProfile) | |
| food_description | text | |
| allergen_info | text | Required, may be empty string if explicitly "none" |
| photo_url | text, nullable | Shown on listing cards (feature 002); null renders a placeholder. Upload/storage is out of scope for this feature — the cook supplies a URL |
| total_portions | integer | > 0 |
| portions_remaining | integer | 0 ≤ portions_remaining ≤ total_portions |
| reduced_price_cents | integer | > 0 |
| retail_reference_price_cents | integer | Used to enforce FR-003 (reduced_price < retail_reference_price) |
| pickup_location | geography(Point, 4326) | PostGIS point for radius queries |
| pickup_address_text | text | Human-readable address shown to buyer post-claim |
| pickup_window_start / pickup_window_end | timestamptz | pickup_window_end MUST be > pickup_window_start and > created_at (Edge Case) |
| status | enum(`draft`, `published`, `expired`, `cancelled`) | State machine below |
| created_at / updated_at | timestamptz | |

**State transitions**:

```text
draft --(publish, any cook)--> published   [flagged "unverified" if cook.verification_status = unverified]
published --(pickup_window_end passes)--> expired          [automatic, background job]
published --(cook cancels)--> cancelled                     [notifies + refunds existing claims]
published --(cook demoted verified -> unverified)--> published [re-flagged "unverified", NOT cancelled]
```

**Validation rules**:
- `reduced_price_cents` MUST be less than `retail_reference_price_cents` (FR-003).
- `pickup_window_end` MUST be in the future at creation time (Edge Case).
- `portions_remaining` MUST never go below 0 or above `total_portions` (FR-006).

## Claim

| Field | Type | Notes |
|---|---|---|
| id | UUID | Primary key |
| listing_id | UUID (FK → Listing) | |
| buyer_id | UUID (FK → BuyerProfile) | |
| quantity | integer | > 0, ≤ listing.portions_remaining at time of claim |
| payment_status | enum(`pending_payment`, `paid`, `payment_failed`, `refunded`) | |
| pickup_status | enum(`pending`, `collected`, `no_show`) | |
| stripe_payment_intent_id | text | Nullable until payment attempted |
| reserved_at | timestamptz | Start of payment-timeout window (FR-008) |
| confirmed_at | timestamptz | Nullable, set when payment succeeds |
| rating | smallint, nullable | 1–5 stars, set once by the buyer after `pickup_status = collected` (FR-013) |
| rated_at | timestamptz, nullable | Set together with `rating` |
| created_at / updated_at | timestamptz | |

**State transitions**:

```text
(created) --> payment_status=pending_payment, pickup_status=pending
pending_payment --(Stripe payment succeeds)--> paid
pending_payment --(timeout elapses, e.g. 10 min)--> released
                                                      [portions returned to listing, claim payment_status stays payment_failed]
paid --(pickup_window_end passes without collection)--> pickup_status=no_show   [non-refundable, FR-012]
paid --(buyer confirms pickup)--> pickup_status=collected
paid --(cook cancels listing)--> payment_status=refunded                        [FR-010]
```

**Validation rules**:
- Creating a `Claim` and decrementing `Listing.portions_remaining` MUST happen atomically (FR-006,
  SC-003) — see research.md concurrency decision.
- `rating` may only be set when `pickup_status = collected`, and only once.
- A claim past its `reserved_at` + timeout with `payment_status=pending_payment` MUST release its
  reserved quantity back to `Listing.portions_remaining` (FR-008).

## ImpactRecord

Derived, append-only record created once a `Claim` reaches `payment_status = paid` (Principle II,
FR-009).

| Field | Type | Notes |
|---|---|---|
| id | UUID | Primary key |
| claim_id | UUID (FK → Claim, unique) | One impact record per completed claim |
| listing_id | UUID (FK → Listing) | Denormalized for aggregate queries |
| portions | integer | Copied from claim.quantity at completion time |
| estimated_weight_grams | integer | Derived from a per-listing or per-dish average; exact estimation method is an implementation detail for planning, not this spec |
| created_at | timestamptz | |

**Validation rules**: Immutable once created (append-only); aggregate waste-reduction reporting
(individual + community-wide, Principle II) reads from this table rather than recomputing from
raw claims.

## Relationships summary

```text
CookProfile 1---* Listing
CookProfile 1---* VerificationHistory
Listing 1---* Claim
BuyerProfile 1---* Claim
Claim 1---1 ImpactRecord (created on payment success)
```
