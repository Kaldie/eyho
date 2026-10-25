# Auth0 setup (demo and production)

Local development does not need Auth0: `docker compose up` starts a mock OIDC server (see
`docs/accounts-local-dev.md`). Follow this guide for the demo and production environments.

Specs: `specs/001-accounts-sign-in/` (research.md, contracts/accounts-api.md).

## 1. Tenant

1. Create an Auth0 tenant and choose an **EU region** (GDPR).
2. Confirm the current free-plan limits (monthly active users, Universal Login branding) before
   go-live.
3. The tenant domain, for example `eyho.eu.auth0.com`, is used below as `<TENANT>`. The token issuer
   is `https://<TENANT>/`.

## 2. Connections

Under Authentication:

- **Social > Google**: enable it. Use your own Google OAuth client for production (Auth0's
  development keys are not for production use). Google identities arrive with a verified email.
- **Database > Username-Password-Authentication**: enable it. Keep "Disable Sign Ups" off.
- Email verification is sent by Auth0 for database sign-ups. Check Branding > Email Templates and
  the email provider settings so verification mails reach people.

**Password reset** (FR-008) is Universal Login's built-in "Forgot password?" flow. Nothing to build.

Apple sign-in is out of scope for this feature; it can be added later as another connection with no
code change.

## 3. API (the audience)

Applications > APIs > Create API:

- Identifier: `eyho-api` (this is `OIDC_AUDIENCE` for the backend and `VITE_OIDC_AUDIENCE` for the
  frontend). Signing algorithm **RS256**.
- Allow offline access (refresh tokens) for the API.

## 4. Single page application

Applications > Create Application > Single Page Application, named for example "eyho web":

- Note the **Client ID** (`VITE_OIDC_CLIENT_ID`).
- Allowed Callback URLs: `http://localhost:5173/auth/callback` and the demo address
  `https://<DEMO HOST>/auth/callback`.
- Allowed Logout URLs and Allowed Web Origins: `http://localhost:5173` and `https://<DEMO HOST>`.
- Refresh Token Rotation: **on**, absolute lifetime 30 days (research.md).
- The frontend uses Authorization Code with PKCE through `oidc-client-ts`; the authority is
  `https://<TENANT>/` (`VITE_OIDC_AUTHORITY`).

## 5. Post-login Action: namespaced email claims

Auth0 access tokens carry no email by default. The backend reads these two claims from the
**access token**:

- `https://eyho.app/email`
- `https://eyho.app/email_verified`

Actions > Library > Build Custom > trigger "Login / Post Login", then add it to the Login flow:

```js
exports.onExecutePostLogin = async (event, api) => {
  const ns = 'https://eyho.app/';
  api.accessToken.setCustomClaim(ns + 'email', event.user.email);
  api.accessToken.setCustomClaim(ns + 'email_verified', event.user.email_verified);
  if (event.user.name) {
    api.accessToken.setCustomClaim(ns + 'name', event.user.name); // optional display name
  }
};
```

The name claim is optional: the backend reads `https://eyho.app/name` (or a standard `name` claim)
for the initial display name and otherwise uses the part of the email before the `@`.

## 6. Management API application (machine to machine)

The backend uses the Management API to resend verification emails and to delete users after account
deletion (FR-006, FR-013).

1. Applications > Create Application > **Machine to Machine**, named "eyho backend".
2. Authorize it for the **Auth0 Management API** with the scopes `update:users` (verification email
   job) and `delete:users`.
3. Set on the backend:
   - `AUTH0_MGMT_DOMAIN=<TENANT>`
   - `AUTH0_MGMT_CLIENT_ID=...`
   - `AUTH0_MGMT_CLIENT_SECRET=...` (a secret: platform secret store, never in the repository)

Management API tokens do not count towards the monthly machine-to-machine token quota of the free
plan (research.md).

## 7. Backend settings per environment

| Setting | Demo | Production |
|---|---|---|
| `APP_ENV` | `demo` | `production` |
| `OIDC_ISSUER` | `https://<TENANT>/` | `https://<TENANT>/` |
| `OIDC_AUDIENCE` | `eyho-api` | `eyho-api` |
| `AUTH0_MGMT_*` | set | set |
| `DEMO_AUTH_ENABLED` | `true` (one-tap accounts) | unset or `false`; startup fails if `true` |
| `KVK_MODE` | `kvk-test` (needs `KVK_API_KEY`) | `kvk` (needs `KVK_API_KEY`) |

In production the backend refuses to start unless `OIDC_ISSUER` is exactly `https://` plus
`AUTH0_MGMT_DOMAIN` (FR-017). If you later add an Auth0 custom domain, set `AUTH0_MGMT_DOMAIN` to
that custom domain as well (the Management API is reachable there too).

## 8. Check

1. Sign in through the frontend; `GET /api/v1/me` with the access token returns your account.
2. Sign up with email and password; the verification email arrives, and
   `POST /api/v1/me/verification-email` resends it (at most once per minute).
3. Delete the account in the app; the user disappears from Auth0 shortly afterwards (the backend
   retries with backoff if Auth0 is unreachable).
