# eyho — working agreements for Claude Code

Read first: `.specify/memory/constitution.md` (non-negotiable rules), `specs/README.md` (feature
roadmap and implementation order), `.claude/agents/README.md` (the dev crew).

## Feature branches and pull requests

- Implement features in roadmap order (`specs/README.md`). Each feature gets **its own branch and
  its own pull request**; never mix two features in one branch.
- Branch name: `feature/<NNN>-<short-name>`, matching the spec directory
  (e.g. `feature/001-accounts-sign-in`). Branch from `main`. When a feature depends on one whose
  PR is not merged yet, branch from that feature's branch and set the PR base to it (stacked PR);
  retarget to `main` once the parent merges.
- Open the PR when the feature is done: all tasks in its `tasks.md` checked off (or explicitly
  listed as deferred in the PR body), the repo's checks passing locally, and `qa-engineer` plus
  `code-reviewer` run per `.claude/agents/README.md`.
- PR body: link the spec, summarise what was built per user story, list deferred tasks and how it
  was tested.
- Update the feature's status in `specs/README.md` in the same PR.
- Process and documentation changes that are not part of a feature (constitution, this file,
  agent definitions, roadmap) may be committed directly to `main`.

## Checks to run before pushing

- Backend: `cd backend && go build ./... && go vet ./... && go test ./...`
- Frontend: `cd frontend && npm ci && npx tsc --noEmit && npx vitest run`
- Full stack: `docker compose up` (constitution: Local Development Constraint)
