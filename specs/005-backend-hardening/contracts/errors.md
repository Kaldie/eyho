# Contract: API Error Shape

Applies to every `/api/v1` endpoint (FR-010). Every error is written by one helper package,
`backend/src/api/respond`. This file is the registry of error codes: a unit test fails if
code calls `respond.*Error*` with a code that is not listed here. Feature 002 adds its codes
to this table in the same PR that introduces them.

## Shape

```json
{ "error": "<code>" }
{ "error": "<code>", "field": "<json field path>" }
{ "error": "<code>", "<extra>": <value> }
```

- `Content-Type: application/json`.
- `error` is always present: a stable, lower_snake_case code meant for programs. The web app
  maps codes to text; there is no human-readable message field.
- `field` is present only for validation codes and names the offending request field using
  its JSON path, for example `default_pickup_location` or `nvwa_declaration.version`.
- Extra keys appear only where the table below lists them.
- Bodies of non-JSON or oversized requests (> 64 KiB) are `422 invalid_field` with no
  `field`.

## Code registry

| Status | Code | Extra keys | Meaning | Since |
|---|---|---|---|---|
| 401 | `unauthenticated` | | missing, invalid, expired or too-old token | 001 |
| 403 | `email_not_verified` | | action needs a verified email | 001 |
| 403 | `cook_profile_required` | | action needs cook onboarding | 001 |
| 403 | `cook_not_registered` | `registration_status`: `pending` \| `inactive` | cook may not publish | 001 |
| 403 | `account_deleted` | | token belongs to a deleted account | 001 |
| 403 | `demo_account` | | demo accounts cannot be deleted | 001 |
| 403 | `own_listing` | | a cook claiming their own listing | 002 |
| 404 | `not_found` | | item does not exist **or the caller may not act on it** (contracts/authorization-matrix.md) | **005** |
| 404 | `unknown_demo_account` | `field`: `key` | demo sign-in with an unseeded key | 001 |
| 409 | `active_claims` / `active_listings` | `count` | account deletion blocked | 001 |
| 409 | `cook_profile_exists` | | second onboarding | 001 |
| 409 | `already_verified` | | resend for a verified email | 001 |
| 409 | `insufficient_portions`, `not_collected`, `already_rated` | | claim state conflicts | 002 |
| 422 | `missing_field` / `invalid_field` | `field` | validation | 001 |
| 422 | `declaration_required`, `kvk_not_found`, `kvk_inactive` | | onboarding checks | 001 |
| 429 | `rate_limited` | | with `Retry-After` header | 001 |
| 500 | `internal_error` | | unexpected failure; details only in the server log | 001 |
| 501 | `not_implemented` | | stub endpoints until feature 002 | scaffold |
| 502 | `identity_provider_unavailable` | | verification email could not be sent | 001 |
| 503 | `identity_provider_unavailable` | | token issuer keys unreachable | 001 |

## Rules

- An error body never echoes request input, internal ids or error strings from the database
  or upstream services.
- 404 `not_found` is byte-identical whether the item is missing or belongs to someone else
  (FR-002).
- New codes need a row here first. Reusing an existing code beats inventing a near-duplicate.
