# Plan — <feature/slug>

> Phase 2. The concrete change. Resolve `open-questions.md` before you start implementing.

## Approach
<One paragraph: the chosen approach and why, referencing the pattern from research.md.>

## Files to create / edit
| File | Change |
|------|--------|
| `internal/…` | <create/edit — what> |
| `web/src/…`  | <create/edit — what> |

## Interfaces affected
- <Which interface(s) — `Dumper`/`Restorer`/`Storage`/`Notifier`/API — and how. If none, say so.>

## Ordered steps
1. <…>
2. <…>

## Tests
- <Unit/integration tests to add. Include a regression test that fails before, passes after.>

## Verification
- Backend: `make fmt && make test && make lint && make build`
- Frontend (if touched): `npm --prefix web run validate`
- Full-stack (if touched): `make build-with-web` then exercise via `/run-daemon`
