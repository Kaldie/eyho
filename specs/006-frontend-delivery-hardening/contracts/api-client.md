# Contract: Frontend API Client and Errors

`frontend/src/services/apiClient.ts` (research.md R3, R4). The only module that calls the
eyho server. Other modules call it through the data functions in `services/api.ts`,
`services/accounts.ts` and so on.

## Client

```ts
interface ApiClient {
  /** fetch() against the API base; adds Authorization when a session exists and
   *  Content-Type: application/json when a body is sent. Never throws on non-2xx. */
  fetch(path: string, init?: RequestInit): Promise<Response>
  /** Like fetch, but throws ApiError on non-2xx and returns the parsed JSON body. */
  json<T>(path: string, init?: RequestInit): Promise<T>
}

function createApiClient(getToken: () => string | null, base?: string): ApiClient
const ApiClientContext: React.Context<ApiClient>
function useApi(): ApiClient
```

- `getToken` reads the session store's in-memory access token at call time (no render-time
  globals; `setAccessTokenGetter` is removed).
- Data functions take the client first: `listListings(api, { lat, lng, radiusKm })`,
  `getListing(api, id)`, `createClaim(api, { listingId, quantity })`, `listOrders(api)`,
  `getOrder(api, id)`, `getMe(api)`, and so on.
- Hooks (`useOrders`, `useMe`, ...) call `useApi()` and set
  `enabled: session.status === 'signedIn'` when the endpoint needs sign-in.
- Non-eyho calls (PDOK in `services/geocoding.ts`, Stripe.js, the OIDC library) do not use
  the client and never receive the eyho token.

## Errors

```ts
class ApiError extends Error {
  status: number                 // HTTP status
  code: string | null            // body.error, e.g. "email_not_verified", "insufficient_portions"
  body: Record<string, unknown>  // full parsed body (e.g. { field }, { count }, { registration_status })
}
```

- The body follows the server's single error shape from feature 005:
  `{ "error": "<code>", ...details }`.
- A network failure (no response) stays a plain `Error`, so screens can tell "connection
  problem" from "server said no" (003 research).
- `ClaimApiError` is removed. Claim conflict handling checks
  `err instanceof ApiError && err.status === 409` (or other 4xx), as described in 002's
  claims contract.
- 401 handling: an `ApiError` with status 401 on an authenticated query triggers one session
  restore attempt (R1). If that fails, the session is signed out and the cache cleared.
