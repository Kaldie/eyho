# API Contract: Accounts & Sign-in

Base path: `/api/v1`. Sign-in itself happens at the identity provider (OIDC Authorization Code +
PKCE); these endpoints cover everything after that.

## Authentication rules (apply to every feature)

- Clients send `Authorization: Bearer <access token>`.
- The backend accepts tokens whose issuer is the configured provider (Auth0 in demo/production,
  the mock server locally) or, in `demo`/`local` only, the built-in demo issuer.
- On the first request with a new identity, the backend creates (or links, per data-model.md)
  the account and its buyer profile.
- Error bodies:
  - `401 {"error": "unauthenticated"}` — missing, expired or invalid token.
  - `403 {"error": "email_not_verified"}` — action needs a verified email (FR-006).
  - `403 {"error": "cook_profile_required"}` — action needs cook onboarding first.
  - `403 {"error": "cook_not_registered", "registration_status": "pending" | "inactive"}` —
    publishing needs a confirmed active KvK registration and the NVWA declaration (FR-020).
  - `403 {"error": "account_deleted"}` — token belongs to a deleted account.

## GET /me

Signed in. Returns the current account.

```json
{
  "id": "uuid",
  "email": "sanne@example.nl",
  "email_verified": true,
  "display_name": "Sanne",
  "is_demo": false,
  "cook_profile": null
}
```

When the account has completed onboarding, `cook_profile` is:

```json
{
  "id": "uuid",
  "display_name": "Sanne's Kitchen",
  "org_type": "professional",
  "phone": "+31612345678",
  "default_pickup_address_text": "Keizersgracht 88, Amsterdam",
  "default_pickup_location": { "lat": 52.37, "lng": 4.89 },
  "verification_status": "unverified",
  "kvk_number": "68750110",
  "kvk_trade_name": "Sanne's Kitchen B.V.",
  "registration_status": "active",
  "registration_checked_at": "2026-10-25T10:00:00Z",
  "nvwa_declared_at": "2026-10-25T10:00:00Z",
  "can_publish": true
}
```

## POST /me/cook-profile

Signed in, verified email. Cook onboarding (FR-009).

**Request**: `display_name`, `org_type` (`nonprofit` | `professional`), `phone`,
`default_pickup_address_text`, `default_pickup_location` `{lat, lng}`, `kvk_number`, and
`nvwa_declaration` `{ "accepted": true, "version": "2026-10-25" }` — all required.

The backend checks `kvk_number` against the business register before saving (FR-018).

- `201 Created` — the cook profile (shape above), `verification_status: "unverified"`,
  `registration_status: "active"`.
- `202 Accepted` — the register was unreachable: profile saved with
  `registration_status: "pending"` and `can_publish: false`; the check is retried in the
  background.
- `409 Conflict` — `{"error": "cook_profile_exists"}`.
- `422 Unprocessable Entity` — `{"error": "missing_field" | "invalid_field", "field": "..."}`,
  `{"error": "declaration_required"}`, or `{"error": "kvk_not_found" | "kvk_inactive"}`. Nothing
  is saved.

## PATCH /me/cook-profile

Signed in, owning cook. Any onboarding field may be changed; existing listings keep their own
pickup addresses. Returns `200` with the updated profile. Changing `kvk_number` runs the same
check as onboarding (same `202`/`422` outcomes); the old number stays in effect only if the new
one is rejected. The declaration can't be withdrawn through this endpoint.

## PATCH /me

Signed in. Updates `display_name`. Returns `200` with the account.

## POST /me/verification-email

Signed in, unverified email. Asks the provider to resend the verification email.

- `202 Accepted`
- `409 Conflict` — `{"error": "already_verified"}`.
- `429 Too Many Requests` — more than one request per minute.

## DELETE /me

Signed in. Deletes the account (FR-013).

- `204 No Content` — personal data erased, identities removed, provider user deletion queued.
- `409 Conflict` — `{"error": "active_claims" | "active_listings", "count": 2}`.

## Demo and local only

Not routed at all unless `APP_ENV` is `demo` or `local` (FR-017).

### GET /demo/accounts

```json
{ "accounts": [
  { "key": "buyer", "label": "Demo buyer" },
  { "key": "cook_verified", "label": "Demo cook (verified)" },
  { "key": "cook_unverified", "label": "Demo cook (unverified)" }
] }
```

### POST /demo/sign-in

**Request**: `{ "key": "buyer" }`. **Response** `200`:
`{ "access_token": "...", "expires_at": "..." }`, a token from the demo issuer for that seeded
account.

### GET /demo/.well-known/openid-configuration and /demo/jwks.json

Discovery and keys for the demo issuer.
