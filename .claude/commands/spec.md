---
description: Scaffold specs/<date>-<slug>/ from the template and run the research phase
argument-hint: <short-slug describing the change> [issue number]
allowed-tools: Bash(mkdir:*), Bash(cp:*), Bash(date:*), Bash(ls:*), Bash(git log:*), Bash(gh issue view:*), Read, Write, Edit, Glob, Grep, Agent
---

Start spec-driven work on: **$ARGUMENTS**

Follow `specs/README.md`. Do the scaffolding and the research phase now; stop before writing any implementation code.

## 1. Scaffold

Derive `<slug>` from the argument (lowercase, hyphenated, no dates). Then:

```bash
SLUG="<slug>"
DIR="specs/$(date +%Y-%m-%d)-$SLUG"
mkdir -p "$DIR" && cp specs/TEMPLATE/*.md "$DIR"/ && ls "$DIR"
```

If a `specs/*-<slug>/` directory already exists, do **not** create a second one — open the existing spec and continue it instead, and say so.

## 2. Research (fill in `research.md`)

Delegate the legwork in parallel rather than reading the repo yourself:

- **`specs-locator`** — what's already written down about this: prior specs, `docs/`, the binding nested `AGENTS.md`, merged PRs/issues. If it's already specced, stop and report that.
- **`codebase-locator`** — where the relevant code lives.
- **`codebase-pattern-finder`** — the closest existing implementation to model after, plus the full touch-point checklist.

If an issue number was given, read it (`gh issue view <n>`) and link it in the spec.

Then fill in `research.md` for real: the goal, the primary area and its governing nested `AGENTS.md`, what exists today **with `file:line` citations**, constraints (streaming, context propagation, config/DB/web touch-points, secret handling), and prior art. Every citation must be a path you actually verified.

## 3. Plan (fill in `plan.md`)

Concrete and ordered: files to create/edit, interfaces affected, the steps in sequence, the tests you'll add and where, and how you'll verify (`make pre-commit` for Go, `make web-validate` for web, `/run-daemon` for a real smoke test).

## 4. Open questions

Put anything that needs a human decision in `open-questions.md`. If any are blocking, say so and **stop** — don't implement past an unresolved blocker.

## 5. Report back

A short summary: the spec path, the governing guide(s), the touch-point count, the verification plan, and any blocking open questions. Do not start implementing unless the user says to.
