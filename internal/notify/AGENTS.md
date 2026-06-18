# Notifications Subsystem — Agent Guide

> Scope: notification channels (e.g. Slack, email/SMTP, webhooks) in `internal/notify/`. Part of the BareD AGENTS.md tree — see the root [`AGENTS.md`](../../AGENTS.md) and the backend guide [`internal/AGENTS.md`](../AGENTS.md). **The innermost guide wins** when instructions conflict.

Channels implement the `Notifier` interface in `notifier.go` (`NotifySuccess`, `NotifyFailure`, `Name`, `ShouldNotifySuccess`), all taking a rich `*Message` describing the backup/restore operation. The current implementations are Slack (`slack.go`), email/SMTP (`email.go`), and webhook (`webhook.go`). The factory `New(cfg *config.Notifier)` in `factory.go` dispatches on `cfg.Type` (`"slack"`, `"email"`, `"webhook"`) and returns an error for unsupported types, so adding a channel means writing an implementation plus registering it in that switch.

> Note: the worked example below uses an illustrative `Notify(ctx, *Event)` shape. The real interface in `notifier.go` splits this into `NotifySuccess`/`NotifyFailure` and passes a `*Message` (not `*Event`). Mirror the existing `slack.go` / `webhook.go` implementations for the exact method set when adding a channel.

## Adding a Notification Channel

**Example**: Adding Discord webhook support

1. **Create implementation** (`internal/notify/discord.go`):

```go
package notify

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "bared/internal/config"
)

type Discord struct {
    webhookURL string
    onSuccess  bool
    onFailure  bool
}

type discordPayload struct {
    Content string        `json:"content"`
    Embeds  []discordEmbed `json:"embeds,omitempty"`
}

type discordEmbed struct {
    Title       string `json:"title"`
    Description string `json:"description"`
    Color       int    `json:"color"`
}

func NewDiscord(cfg *config.Notifier) *Discord {
    return &Discord{
        webhookURL: cfg.WebhookURL,
        onSuccess:  cfg.OnSuccess,
        onFailure:  cfg.OnFailure,
    }
}

func (d *Discord) Notify(ctx context.Context, event *Event) error {
    // Check if we should notify for this event type
    if (event.Success && !d.onSuccess) || (!event.Success && !d.onFailure) {
        return nil
    }

    color := 3066993 // Green
    if !event.Success {
        color = 15158332 // Red
    }

    payload := discordPayload{
        Embeds: []discordEmbed{
            {
                Title:       fmt.Sprintf("Backup %s: %s", statusString(event.Success), event.Target),
                Description: event.Message,
                Color:       color,
            },
        },
    }

    body, _ := json.Marshal(payload)
    req, err := http.NewRequestWithContext(ctx, "POST", d.webhookURL, bytes.NewReader(body))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
    }

    return nil
}

func (d *Discord) Name() string {
    return "discord"
}

func statusString(success bool) string {
    if success {
        return "Success"
    }
    return "Failed"
}
```

2. **Update config** (`internal/config/config.go`):

```go
type Notifier struct {
    Type   string `yaml:"type"`
    Name   string `yaml:"name,omitempty"`

    // ... existing fields

    // Discord-specific (reuses WebhookURL)
    WebhookURL string `yaml:"webhook_url,omitempty"`
    OnSuccess  bool   `yaml:"on_success,omitempty"`
    OnFailure  bool   `yaml:"on_failure,omitempty"`
}
```

3. **Register in factory** (`internal/notify/factory.go`):

```go
func New(cfg *config.Notifier) (Notifier, error) {
    switch cfg.Type {
    case "slack":
        return NewSlack(cfg), nil
    case "email":
        return NewEmail(cfg), nil
    case "webhook":
        return NewWebhook(cfg), nil
    case "discord":  // ADD THIS
        return NewDiscord(cfg), nil
    default:
        return nil, fmt.Errorf("unsupported notifier type: %s", cfg.Type)
    }
}
```

## See also

- [`../AGENTS.md`](../AGENTS.md) — backend (`internal/`) guide
- [`../../AGENTS.md`](../../AGENTS.md) — root BareD agent guide
- [`../database/AGENTS.md`](../database/AGENTS.md) — database subsystem guide
- [`../storage/AGENTS.md`](../storage/AGENTS.md) — storage subsystem guide
