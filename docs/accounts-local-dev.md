# Accounts and sign-in: local development

Feature 001 (`specs/001-accounts-sign-in/`). Everything runs from `docker compose up`; no provider
account, API key or internet connection is needed.

## Services

| Service | Port | What |
|---|---|---|
| `mock-oidc` | 8081 | Mock OIDC issuer (`ghcr.io/navikt/mock-oauth2-server`, issuer id `eyho`, audience `eyho-api`) |
| `backend` | 8080 | Verifies tokens, hosts `/api/v1/me`, `/api/v1/demo/*` |
| `frontend` | 5173 | Uses `VITE_OIDC_*` to sign in at the mock server |

The backend runs with `APP_ENV=local`, `DEMO_AUTH_ENABLED=true` and `KVK_MODE=fake`. See
`backend/.env.example` for every setting.

## Signing in with the mock server

Press "Sign in". On the mock server's login page:

- **Username**: becomes the token `sub`, so a new username is a new identity.
- **Optional claims**: the backend reads the email from namespaced claims, so enter for example

  ```json
  {"https://eyho.app/email": "sanne@example.nl", "https://eyho.app/email_verified": true}
  ```

  Set `email_verified` to `false` to see the verification gate. Two usernames with the same verified
  email resolve to one account (FR-007); an unverified email never links.

A token without an email claim is rejected with `401`.

## Demo accounts (one tap)

With `DEMO_AUTH_ENABLED=true` (only allowed when `APP_ENV` is `demo` or `local`):

- `GET /api/v1/demo/accounts` lists them; `POST /api/v1/demo/sign-in` with `{"key": "..."}` returns a
  short-lived token from the built-in demo issuer.

| Key | Account | Notes |
|---|---|---|
| `buyer` | Demo buyer | |
| `cook_verified` | Demo cook (verified) | KvK 68750110, registration active |
| `cook_unverified` | Demo cook (unverified) | KvK 69599084, registration active |

They come from `backend/src/db/seed/demo_accounts.sql`. Load it after migrating:

```sh
docker compose exec -T db psql -U eyho -d eyho < backend/src/db/seed/demo_accounts.sql
```

Demo accounts cannot be deleted, and the demo issuer only signs tokens for seeded accounts.

## Fake business register (KVK_MODE=fake)

Cook onboarding checks the KvK number against a register. Locally this is an in-memory fake with
fixed numbers (fixtures of this project; not confirmed against KVK's own test set):

| KvK number | Result |
|---|---|
| `68750110` | active, trade name "Keuken Katrien B.V." |
| `69599084` | active, trade name "Test Eetcafe Jordaan" |
| `90000002` | inactive (`422 kvk_inactive`) |
| any other 8 digits, e.g. `12345678` | not found (`422 kvk_not_found`) |

The NVWA declaration is a checkbox with versioned wording (for example `2026-10-25`), stored with
the time it was made. Cooks need an active registration and the declaration to publish (FR-020).
This status is separate from the reputation-based "verified" status.

A register outage during onboarding gives `202` with `registration_status: "pending"`; a background
job retries with backoff, and re-checks every cook at least every 30 days. A transport error never
downgrades an active cook.

## Demo and production

- Auth0: see `docs/auth0-setup.md`.
- Demo uses KVK's test environment: `KVK_MODE=kvk-test` plus `KVK_API_KEY` (the key published for
  KVK's test environment; see the KVK developer portal).
- Production uses `KVK_MODE=kvk` with a paid key. To get one, create an account on the KVK developer
  portal (<https://developers.kvk.nl>), subscribe to the Handelsregister **Zoeken** API v2 (about
  EUR 6.20 per month at the time of writing; Zoeken queries themselves are free) and put the key in
  `KVK_API_KEY`. Verify the current price and terms there.
- The backend refuses to start with demo auth, the fake or test register, or a non-Auth0 issuer when
  `APP_ENV` is not `demo` or `local` (FR-017).

## Tests

```sh
cd backend
DATABASE_URL='postgres://eyho:eyho@localhost:5432/eyho?sslmode=disable' go test ./...
```

Integration tests (`backend/tests/integration`) skip when `DATABASE_URL` is unset. They need the
migrations applied and the demo seed loaded.
