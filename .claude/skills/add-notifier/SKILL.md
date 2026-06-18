---
name: add-notifier
description: Scaffold a new notification channel implementing the Notifier interface in BareD, following the Slack/Email/Webhook pattern. Use when the user wants backup success/failure alerts sent somewhere new (e.g. "add Discord notifications", "support Telegram alerts", "add a PagerDuty/Opsgenie notifier", "send backup results to Microsoft Teams", "implement a new Notifier", "register a new notifier.type").
---

# Add a Notifier

Scaffold a new notification channel that implements the `Notifier` interface, mirroring the existing Slack / Email / Webhook channels. The authoritative, worked recipe lives in the nested guide — follow it; this skill is the checklist + entry point.

## When to use
- "Add Discord / Telegram / Microsoft Teams / PagerDuty / Opsgenie notifications"
- "Send backup success/failure alerts to a new channel"
- A new `notifier.type` value is needed
- Implementing the `Notifier` interface for a new channel

## Reference
- `internal/notify/AGENTS.md` — the deep recipe. Read it first. Note: its example uses an illustrative `Notify(ctx, *Event)` shape — the REAL interface is different (see below); mirror `slack.go` / `webhook.go` for the exact method set.
- Interface lives in `internal/notify/notifier.go`: `Notifier` requires `NotifySuccess(ctx, *Message)`, `NotifyFailure(ctx, *Message)`, `Name()`, and `ShouldNotifySuccess()`. Channels receive a rich `*Message` (target, operation, duration, sizes, storage/db details), not an `*Event`.

## Steps
1. Create `internal/notify/<channel>.go` with a struct implementing `NotifySuccess`, `NotifyFailure`, `Name`, and `ShouldNotifySuccess`, plus a `New<Channel>(cfg)` constructor (existing constructors are `NewSlack` / `NewEmail` / `NewWebhook`).
2. Add channel-specific fields to the `Notifier` struct in `internal/config/config.go` with `yaml` tags (reuse `WebhookURL`/`OnSuccess`/`OnFailure` where it fits).
3. Register the channel in the `New(cfg *config.Notifier)` switch in `internal/notify/factory.go` (the live factory function is `New`, dispatching on `cfg.Type`).
4. Allow the new `notifier.type` in `internal/config/validator.go` and validate required fields.
5. Add unit tests `internal/notify/<channel>_test.go` following the existing pattern.
6. Wire the web UI: add the notifier type to the React notifier forms and types in `web/src/` (see `web/AGENTS.md`).
7. Update `examples/config.example.yml`, `README.md`, and `docs/user-guide/configuration.md`.

## Verify
- `make test` — unit tests pass
- `make build` — backend compiles
- `make web-validate` and `make build-with-web` — if you touched the React UI
- `make validate` — run before finishing
- If you changed the `Notifier` interface, update `internal/notify/AGENTS.md` and `docs/` accordingly.
