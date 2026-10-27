# eyho

A marketplace where non-profit and professional cooks sell surplus cooked food at a reduced price,
and buyers discover and claim it nearby.

- Project rules: `.specify/memory/constitution.md`; feature specs: `specs/`.
- Run everything locally: `docker compose up` (database with schema and demo data, backend, frontend,
  mock sign-in server). Copy the `.env.example` files first if you want to change settings.
- Sign-in, demo accounts and the fake business register's test KvK numbers:
  [docs/accounts-local-dev.md](docs/accounts-local-dev.md).
- Auth0 for the demo and production, and how to get a KVK API key:
  [docs/auth0-setup.md](docs/auth0-setup.md) and the "Demo and production" section of the local
  development guide.
- Checks: `cd backend && go build ./... && go vet ./... && go test ./...`
