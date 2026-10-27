# Feature roadmap

Features are numbered in dependency order: each one builds only on features with a lower
number. Implement them in this order, one branch and one pull request per feature (see the
root `CLAUDE.md`).

| # | Feature | Builds on | Status |
|---|---|---|---|
| 001 | [Accounts & sign-in](001-accounts-sign-in/spec.md) | — | Merged (Kaldie/eyho#1). Open: T037 full-stack quickstart run + QA/review |
| 002 | [Post & claim surplus food](002-surplus-food-listing/spec.md) | 001 | Implemented, PR to `main` open: Kaldie/eyho#9 (Kaldie/eyho#2 merged into the 001 branch after 001 had landed, so the code never reached `main`). Open: T029 Stripe Connect payouts, T031 full-stack run |
| 003 | [Modern, inviting frontend](003-modern-inviting-frontend/spec.md) | 001, 002 | Done and merged; real API wiring comes with 002 |
| 004 | [Private demo deployment](004-demo-deployment/spec.md) | 001, 002, 003 | Specified; plan after 001 and 002 are implemented |
| 005 | Backend hardening | 001, 002 | Spec + plan on `feature/005-backend-hardening` (draft Kaldie/eyho#7); tasks not generated, implementation not started |
| 006 | Frontend & delivery hardening | 005 | Spec + plan on `feature/006-frontend-delivery-hardening` (draft Kaldie/eyho#8, stacked on 005); tasks not generated, implementation not started |

### Outside the numbered features

| Item | Status |
|---|---|
| Landing page for signed-out visitors (`/`, discovery moved to `/discover` for them) | Done and merged (Kaldie/eyho#5); built without a spec |

### Open work at a glance

- **001 T037** — run the quickstart scenarios on `docker compose up`, then `qa-engineer` and `code-reviewer`.
- **002 T029** — Stripe Connect onboarding so cooks get paid: needs spec/data-model/contract changes first, then tasks.
- **002 T031** — full-stack quickstart run, QA and review (Principle I review for T008, T014, T022).
- **004** — plan and tasks.
- **005 / 006** — generate `tasks.md`, then implement.
- **Frontend tests** — `npx vitest run` crashes on Node 20 (undici) but passes on Node 22; bump the Node requirement.

**Parallel work**: once 001's Phase 2 (accounts, auth middleware, mock sign-in) is merged, the
rest of 001 and all of 002 can proceed in parallel.

**Renumbering (2026-10-25)**: features were originally numbered in the order they were written.
They were renumbered to dependency order: old 001 â†’ 002, old 002 â†’ 003, old 003 â†’ 004,
old 004 â†’ 001. Commit messages before that date use the old numbers.
