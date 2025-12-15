package app

import (
	"fmt"
	"net/url"
	"strings"

	"bared/internal/config"
)

func notifierDestination(cfg *config.Notifier) string {
	if cfg == nil {
		return ""
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "slack", "webhook":
		return sanitizeURLForLogs(cfg.URL)
	case "email":
		toCount := len(cfg.SMTPTo)
		if toCount == 0 {
			return fmt.Sprintf("smtp=%s:%d to=<none>", cfg.SMTPHost, cfg.SMTPPort)
		}
		return fmt.Sprintf("smtp=%s:%d to=%s", cfg.SMTPHost, cfg.SMTPPort, strings.Join(cfg.SMTPTo, ","))
	default:
		// Best-effort: don't drop potentially useful info for unknown types.
		if cfg.URL != "" {
			return sanitizeURLForLogs(cfg.URL)
		}
		return ""
	}
}

func sanitizeURLForLogs(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "<invalid-url>"
	}

	host := u.Hostname()
	path := u.EscapedPath()

	// Slack incoming webhooks embed secrets in the path; never log them.
	if strings.EqualFold(host, "hooks.slack.com") && strings.HasPrefix(path, "/services/") {
		return "hooks.slack.com/services/<redacted>"
	}

	// Drop query/fragment/userinfo; keep only host+path for routing visibility.
	if host == "" {
		return "<unknown-host>"
	}
	if path == "" {
		return host
	}
	return host + path
}
