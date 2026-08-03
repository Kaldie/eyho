# eyho dev crew (Claude Code subagents)

This project is developed by a crew of role-specific Claude Code subagents instead of a single
generalist agent. Each crew member is defined as a `.md` file in this directory: YAML frontmatter
sets its `name`, `description` (used for auto-routing), allowed `tools`, and `model`; the body is
its system prompt.

## Crew

| Agent | Role | Model |
|---|---|---|
| `backend-engineer` | Go API, services, DB access, Stripe integration | sonnet |
| `frontend-engineer` | React/TypeScript web app, claim/checkout UI | sonnet |
| `database-engineer` | PostgreSQL/PostGIS schema, migrations, sqlc queries | sonnet |
| `qa-engineer` | Tests implementation against spec.md/quickstart.md, read-only | sonnet |
| `code-reviewer` | Final review against the constitution + spec, read-only | opus |

All of them run under this workspace's Claude (Team) login — no separate API key or per-token
billing is required, unlike external orchestration frameworks (CrewAI, AutoGen, LangGraph) that
call the Anthropic API directly.

## How to invoke them

From the main conversation, dispatch work with the Task/Agent tool and the matching
`subagent_type`, e.g.:

```
Agent(subagent_type: "backend-engineer", description: "Implement FR-006 atomic claim decrement",
      prompt: "Implement the claim-creation endpoint per contracts/claims-api.md, ...")
```

Typical flow once `/speckit-tasks` has produced `specs/<feature>/tasks.md`:

1. Assign each task (or independent group of tasks) to the engineer agent that owns that layer
   (backend/frontend/database).
2. After an engineer agent reports a task done, dispatch `qa-engineer` to validate it against the
   feature's `spec.md` and `quickstart.md` scenarios.
3. Before marking the task complete, dispatch `code-reviewer` for a final pass against the
   constitution and spec/contracts.

Each agent's system prompt already tells it to read the constitution, the feature's plan/data
model/contracts, and `tasks.md` before doing anything — you don't need to repeat that context in
every prompt, just point it at the specific task.
