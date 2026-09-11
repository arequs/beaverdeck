# BeaverDeck Agent Instructions

At the start of every task, read these files before making changes:

- `AGENTS.md`
- `docs/PROJECT_CONTEXT.md`
- `docs/DECISIONS.md`
- `docs/CHANGELOG_AI.md`

Before changing code, study the relevant project structure, `README.md`, configuration files, package/dependency files, CI/CD files if present, and existing implementation patterns.

Do not make large architectural changes unless explicitly requested.

Do not delete or rewrite existing logic without a clear reason tied to the task.

Do not add new dependencies without explaining why they are needed and why existing dependencies or standard library code are not enough.

Follow the current project style:

- Go backend under `cmd/server/` and `internal/`.
- React frontend under `ui/src/`.
- Helm chart under `charts/beaverdeck/`.
- Keep changes scoped to the requested behavior.
- Prefer existing helper functions and data shapes over introducing parallel abstractions.
- Use `gofmt` for Go files.
- Rebuild the frontend with `npm run build` from `ui/` when frontend source changes, because production assets are embedded under `cmd/server/web/dist`.

After every task, update project memory:

- Update `docs/PROJECT_CONTEXT.md` when new facts about the project are learned.
- Update `docs/DECISIONS.md` when an architectural or technical decision is made.
- Update `docs/CHANGELOG_AI.md` when code, configuration, CI/CD, infrastructure, or documentation changes.

At the end of every response, briefly report:

- What changed.
- Which files were touched.
- Which checks were run.
- What project memory was updated.

The repository may have uncommitted user or generated changes. Do not revert unrelated changes.
