# specs/ — spec-driven feature work

For any non-trivial change (touches more than ~3 files, adds a database/storage/notifier
backend, changes an interface, or has unclear requirements), capture the thinking here
**before** writing code. Small fixes don't need a spec.

## How it works

1. Create a folder: `specs/<YYYY-MM-DD>-<short-slug>/` (e.g. `specs/2026-06-18-add-gcs-storage/`).
2. Copy the skeletons from [`TEMPLATE/`](TEMPLATE/) into it and fill them in as you go:
   - **`research.md`** — what exists today, where the relevant code lives (cite `file:line`),
     which nested `AGENTS.md` governs the area, constraints, prior art in the repo.
   - **`plan.md`** — the concrete change: files to create/edit, interfaces affected, the
     ordered steps, the tests you'll add, and how you'll verify.
   - **`open-questions.md`** — anything you need a human to decide. Resolve these before
     implementing.
   - **`implementation-notes.md`** — written during/after implementation: decisions made,
     surprises, follow-ups, and anything that should later flow into the docs.
3. Implement against the plan. Keep the spec updated when reality diverges.
4. Link the spec folder from the PR description.

This mirrors the `research → plan → implement → verify → ship` workflow in the root
[`AGENTS.md`](../AGENTS.md). The relevant project skills (`/add-database-type`,
`/add-storage-backend`, `/add-notifier`, `/add-api-endpoint`, `/add-config-field`) each
point back at the nested `AGENTS.md` that should inform the research and plan.

> Specs are durable design records, not throwaway notes — keep them in the repo so the next
> agent (or human) can see why a change was made.
