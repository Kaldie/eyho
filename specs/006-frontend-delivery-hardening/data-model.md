# Data Model: Frontend & Delivery Hardening

No server-side or persisted data changes. The entities below are client-side state and
configuration shapes.

## SessionHint (persisted, `sessionStorage["eyho.session"]`)

The only auth-related data written to web storage (research.md R1). Tab-scoped, deleted on
sign-out and when a restore fails.

| Field | Type | Notes |
|---|---|---|
| kind | `"oidc"` \| `"demo"` | Exactly one; writing one kind replaces the other (FR-001) |
| refreshToken | string | `kind = "oidc"` only. Rotated on every refresh; the new value overwrites the old |
| sub | string | `kind = "oidc"` only. Subject of the last session, used to detect an account switch |
| demoKey | string | `kind = "demo"` only. Public seeded key (`buyer`, `cook_verified`, ...), used to re-mint |

**Never stored**: access token, ID token, email or profile data.

The old `eyho.demo-session` key holds a demo access token. It is deleted on first load after
the upgrade.

## Session (in memory, `AuthProvider`)

| Field | Type | Notes |
|---|---|---|
| status | `"loading"` \| `"signedOut"` \| `"signedIn"` | `loading` only while a hint is being restored on start-up |
| kind | `"oidc"` \| `"demo"` \| null | null when signed out |
| accessToken | string \| null | Memory only; read by the API client's `getToken` |
| sub | string \| null | Subject of the current token |
| error | string \| null | Sign-in callback failure message (unchanged from 001) |

### State transitions

```text
start ──(no hint)──────────────────────────────► signedOut
start ──(hint)──► loading ──(restore ok)───────► signedIn
                         └─(restore fails)─────► signedOut   [hint deleted]
signedOut ──(OIDC callback ok | demo sign-in)──► signedIn    [cache cleared, hint written]
signedIn(kind A) ──(sign-in with kind B)───────► signedIn(B) [A ended, cache cleared]
signedIn ──(renewal returns another sub)───────► signedIn    [cache cleared]
signedIn ──(renewal fails | 401 after retry)───► signedOut   [hint deleted, cache cleared]
signedIn ──(sign-out)──────────────────────────► signedOut   [refresh token revoked, hint deleted, cache cleared]
```

## RuntimeConfig

See [contracts/runtime-config.md](./contracts/runtime-config.md). Loaded once before the first
render. Read-only afterwards.

## ApiError

See [contracts/api-client.md](./contracts/api-client.md). Replaces `ApiError` and
`ClaimApiError`.

## Fixture data (dev/test only)

`src/dev/fixtures/` holds the example listings (`MOCK_LISTINGS`), orders (`MOCK_ORDERS`), cook
dashboard listings and the special claim ids (`network-error-listing`, `expired-listing`),
moved unchanged from `services/api.ts` and `CookDashboardPage.tsx`. They are exposed only as
MSW handlers. The production build must not contain them (SC-003).
