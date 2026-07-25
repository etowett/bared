---
name: add-notifier
description: Scaffold a new notification channel (Notifier) in BareD, following the Slack/Email/Webhook pattern. Use for "add Discord notifications", "support Telegram alerts", "add a PagerDuty notifier", "send backup results to Teams", "register a new notifier.type".
---

# Add a Notifier

Scaffold a new notification channel that implements the `Notifier` interface, mirroring the existing Slack / Email / Webhook channels. The authoritative, worked recipe lives in the nested guide — follow it; this skill is the checklist + entry point.

## When to use
- "Add Discord / Telegram / Microsoft Teams / PagerDuty / Opsgenie notifications"
- "Send backup success/failure alerts to a new channel"
- A new `notifier.type` value is needed
- Implementing the `Notifier` interface for a new channel

## Reference
- `apps/api/internal/notify/AGENTS.md` — the deep recipe. Read it first. Note: its example uses an illustrative `Notify(ctx, *Event)` shape — the REAL interface is different (see below); mirror `slack.go` / `webhook.go` for the exact method set.
- Interface lives in `apps/api/internal/notify/notifier.go`: `Notifier` requires `NotifySuccess(ctx, *Message)`, `NotifyFailure(ctx, *Message)`, `Name()`, and `ShouldNotifySuccess()`. Channels receive a rich `*Message` (target, operation, duration, sizes, storage/db details), not an `*Event`.

## Steps
1. Create `apps/api/internal/notify/<channel>.go` with a struct implementing `NotifySuccess`, `NotifyFailure`, `Name`, and `ShouldNotifySuccess`, plus a `New<Channel>(cfg)` constructor (existing constructors are `NewSlack` / `NewEmail` / `NewWebhook`).
2. Add channel-specific fields to the `Notifier` struct in `apps/api/internal/config/config.go` with `yaml` tags (reuse `WebhookURL`/`OnSuccess`/`OnFailure` where it fits).
3. Register the channel in the `New(cfg *config.Notifier)` switch in `apps/api/internal/notify/factory.go` (the live factory function is `New`, dispatching on `cfg.Type`).
4. Allow the new `notifier.type` in `apps/api/internal/config/validator.go` and validate required fields.
5. Add unit tests `apps/api/internal/notify/<channel>_test.go` following the existing pattern.
6. Wire the web UI: add the notifier type to the React notifier forms and types in `apps/web/src/` (see `apps/web/AGENTS.md`).
7. Update `examples/config.example.yml`, `README.md`, and `docs/user-guide/configuration.md`.

## Verify
- `make test` — unit tests pass
- `make build` — backend compiles
- `make web-validate` and `make build-with-web` — if you touched the React UI
- `make pre-commit` — the full backend gate; run before finishing
- If you changed the `Notifier` interface, update `apps/api/internal/notify/AGENTS.md` and `docs/` accordingly.
