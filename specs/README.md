# Feature roadmap

Features are numbered in dependency order: each one builds only on features with a lower
number. Implement them in this order.

| # | Feature | Builds on | Status |
|---|---|---|---|
| 001 | [Accounts & sign-in](001-accounts-sign-in/spec.md) | — | Planned, `tasks.md` ready |
| 002 | [Post & claim surplus food](002-surplus-food-listing/spec.md) | 001 | Planned, `tasks.md` ready; backend scaffolded (handlers return 501) |
| 003 | [Modern, inviting frontend](003-modern-inviting-frontend/spec.md) | 001, 002 | Done against mock data; real API wiring happens in 002 (T012, T021) and 001 (US1–US5 UI tasks) |
| 004 | [Private demo deployment](004-demo-deployment/spec.md) | 001, 002, 003 | Specified; plan after 001 and 002 are implemented |

**Parallel work**: once 001's Phase 2 (accounts, auth middleware, mock sign-in) is merged, the
rest of 001 and all of 002 can proceed in parallel.

**Renumbering (2026-10-25)**: features were originally numbered in the order they were written.
They were renumbered to dependency order: old 001 → 002, old 002 → 003, old 003 → 004,
old 004 → 001. Commit messages before that date use the old numbers.
