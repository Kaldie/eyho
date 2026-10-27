# Contract: Runtime Configuration (`/app-config.json`)

Served by the production web image, rendered from environment variables at container
start-up (research.md R7). The app fetches it once before rendering (`src/config/runtimeConfig.ts`).
In Vite dev and fixtures modes the same shape is built from `import.meta.env.VITE_*`.

```json
{
  "oidc": {
    "authority": "https://eyho.eu.auth0.com/",
    "clientId": "abc123",
    "audience": "eyho-api"
  },
  "stripePublishableKey": "pk_test_...",
  "imageHosts": ["https://images.eyho.app"],
  "geocodingBaseUrl": "https://api.pdok.nl/bzk/locatieserver/search/v3_1",
  "demoSignIn": false
}
```

| Field | Env var (image) | Dev fallback | Rules |
|---|---|---|---|
| `oidc` | `OIDC_AUTHORITY`, `OIDC_CLIENT_ID`, `OIDC_AUDIENCE` | `VITE_OIDC_*` | `null` when authority or client id is empty: redirect sign-in hidden (today's behaviour) |
| `stripePublishableKey` | `STRIPE_PUBLISHABLE_KEY` | `VITE_STRIPE_PUBLISHABLE_KEY` | Required in production builds; startup fails loudly when missing (no `pk_test_mock_placeholder` fallback) |
| `imageHosts` | `IMAGE_HOSTS` (space-separated origins) | `VITE_IMAGE_HOSTS` | Each entry an `https:` origin; empty list = placeholders only |
| `geocodingBaseUrl` | `GEOCODING_BASE_URL` | PDOK default | Its origin must equal the CSP `connect-src` entry |
| `demoSignIn` | `DEMO_SIGN_IN` | `true` in dev | Only hides or shows the demo buttons. The backend's 001 safeguard still decides whether `/demo/*` exists |

- Contains no secrets. Every value is public by nature (client id, publishable key).
- `Cache-Control: no-cache`, so a redeploy with new values takes effect on the next load.
