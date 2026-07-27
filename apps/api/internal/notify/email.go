package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"html"
	"net/smtp"
	"strings"
	"time"

	"github.com/etowett/bared/apps/api/internal/config"
)

// Email implements Notifier for SMTP email notifications
type Email struct {
	cfg *config.Notifier
}

// NewEmail creates a new Email notifier
func NewEmail(cfg *config.Notifier) *Email {
	return &Email{cfg: cfg}
}

// Name returns the notifier name
func (e *Email) Name() string {
	return "email"
}

// ShouldNotifySuccess returns true if success notifications are enabled
func (e *Email) ShouldNotifySuccess() bool {
	return e.cfg.OnSuccess
}

// NotifySuccess sends a success notification via email
func (e *Email) NotifySuccess(ctx context.Context, msg *Message) error {
	if !e.cfg.OnSuccess {
		return nil
	}

	subject := fmt.Sprintf("✓ %s Successful: %s", operationTitle(msg.Operation), msg.Target)
	body := e.buildSuccessHTML(msg)

	return e.sendWithRetry(ctx, subject, body, 3)
}

// NotifyFailure sends a failure notification via email
func (e *Email) NotifyFailure(ctx context.Context, msg *Message) error {
	subject := fmt.Sprintf("✗ %s Failed: %s", operationTitle(msg.Operation), msg.Target)
	body := e.buildFailureHTML(msg)

	return e.sendWithRetry(ctx, subject, body, 3)
}

func operationTitle(op string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "backup":
		return "Backup"
	case "restore":
		return "Restore"
	default:
		if op == "" {
			return "Operation"
		}
		return strings.ToUpper(op[:1]) + strings.ToLower(op[1:])
	}
}

// buildSuccessHTML builds HTML email for success notification
func (e *Email) buildSuccessHTML(msg *Message) string {
	opTitle := "Backup"
	if msg.Operation == "restore" {
		opTitle = "Restore"
	}
	if msg.DryRun {
		opTitle += " (DRY-RUN)"
	}

	// Escape all user-controlled fields to prevent HTML injection
	escapedTarget := html.EscapeString(msg.Target)
	escapedScheduledBy := html.EscapeString(msg.ScheduledBy)
	escapedStorageName := html.EscapeString(msg.StorageName)
	escapedStorageType := html.EscapeString(msg.StorageType)
	escapedPath := html.EscapeString(msg.Path)
	escapedDatabaseName := html.EscapeString(msg.DatabaseName)
	escapedDatabaseType := html.EscapeString(msg.DatabaseType)

	htmlContent := `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #333; }
.container { max-width: 600px; margin: 0 auto; padding: 20px; }
.header { background: #28a745; color: white; padding: 20px; border-radius: 5px 5px 0 0; }
.header h1 { margin: 0; font-size: 24px; }
.content { background: #f8f9fa; padding: 20px; border-radius: 0 0 5px 5px; }
.section { background: white; padding: 15px; margin-bottom: 15px; border-radius: 5px; border-left: 4px solid #28a745; }
.section h2 { margin-top: 0; font-size: 16px; color: #28a745; }
.metric { display: flex; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid #e9ecef; }
.metric:last-child { border-bottom: none; }
.label { font-weight: 600; color: #6c757d; }
.value { color: #212529; font-family: 'Monaco', 'Courier New', monospace; }
.stages { list-style: none; padding: 0; }
.stages li { padding: 8px 0; display: flex; justify-content: space-between; }
.stage-name { font-weight: 500; }
.stage-duration { color: #6c757d; font-size: 14px; }
.footer { text-align: center; padding: 20px; color: #6c757d; font-size: 12px; }
</style>
</head>
<body>
<div class="container">
<div class="header">
<h1>✓ ` + opTitle + ` Successful</h1>
</div>
<div class="content">
<div class="section">
<h2>Summary</h2>
<div class="metric"><span class="label">Target</span><span class="value">` + escapedTarget + `</span></div>
<div class="metric"><span class="label">Duration</span><span class="value">` + msg.Duration.String() + `</span></div>`

	// Add trigger info
	if msg.Manual {
		htmlContent += `<div class="metric"><span class="label">Trigger</span><span class="value">Manual</span></div>`
	} else if msg.ScheduledBy != "" {
		htmlContent += `<div class="metric"><span class="label">Trigger</span><span class="value">Scheduled (` + escapedScheduledBy + `)</span></div>`
	}

	htmlContent += `<div class="metric"><span class="label">Time</span><span class="value">` + msg.Timestamp.Format("2006-01-02 15:04:05") + `</span></div>
</div>`

	// Size metrics section for backups
	if msg.Operation == "backup" && msg.Size > 0 {
		htmlContent += `<div class="section">
<h2>Size Metrics</h2>`
		if msg.UncompressedSize > 0 {
			htmlContent += `<div class="metric"><span class="label">Uncompressed Size</span><span class="value">` + formatBytes(msg.UncompressedSize) + `</span></div>`
			htmlContent += `<div class="metric"><span class="label">Compressed Size</span><span class="value">` + formatBytes(msg.Size) + `</span></div>`
			if msg.CompressionRatio > 0 {
				htmlContent += `<div class="metric"><span class="label">Compression Ratio</span><span class="value">` + fmt.Sprintf("%.1f%% reduction", msg.CompressionRatio) + `</span></div>`
			}
		} else {
			htmlContent += `<div class="metric"><span class="label">Size</span><span class="value">` + formatBytes(msg.Size) + `</span></div>`
		}
		htmlContent += `</div>`
	} else if msg.Operation == "restore" && msg.Size > 0 {
		htmlContent += `<div class="section">
<h2>Backup Details</h2>
<div class="metric"><span class="label">Backup Size</span><span class="value">` + formatBytes(msg.Size) + `</span></div>
</div>`
	}

	// Storage section
	if msg.StorageName != "" {
		htmlContent += `<div class="section">
<h2>Storage</h2>
<div class="metric"><span class="label">Storage Name</span><span class="value">` + escapedStorageName + `</span></div>
<div class="metric"><span class="label">Storage Type</span><span class="value">` + escapedStorageType + `</span></div>`
		if msg.Path != "" {
			htmlContent += `<div class="metric"><span class="label">Path</span><span class="value">` + escapedPath + `</span></div>`
		}
		htmlContent += `</div>`
	}

	// Database section
	if msg.DatabaseName != "" {
		htmlContent += `<div class="section">
<h2>Database</h2>
<div class="metric"><span class="label">Database Name</span><span class="value">` + escapedDatabaseName + `</span></div>
<div class="metric"><span class="label">Database Type</span><span class="value">` + escapedDatabaseType + `</span></div>
</div>`
	}

	// Restore validations
	if msg.Operation == "restore" && len(msg.Validations) > 0 {
		htmlContent += `<div class="section">
<h2>Validations</h2>
<div class="metric"><span class="label">Validations Passed</span><span class="value">` + fmt.Sprintf("%d", msg.ValidationsPassed) + `</span></div>
</div>`
	}

	// Stages section
	if len(msg.Stages) > 0 {
		htmlContent += `<div class="section">
<h2>Stages</h2>
<ul class="stages">`
		for _, stage := range msg.Stages {
			icon := "•"
			switch stage.Status {
			case "failed":
				icon = "✗"
			}
			escapedStageName := html.EscapeString(stage.Name)
			htmlContent += `<li><span class="stage-name">` + icon + ` ` + escapedStageName + `</span><span class="stage-duration">` + stage.Duration.String() + `</span></li>`
		}
		htmlContent += `</ul>
</div>`
	}

	htmlContent += `</div>
<div class="footer">
BareD Backup System | ` + time.Now().Format("2006") + `
</div>
</div>
</body>
</html>`

	return htmlContent
}

// buildFailureHTML builds HTML email for failure notification
func (e *Email) buildFailureHTML(msg *Message) string {
	opTitle := "Backup"
	if msg.Operation == "restore" {
		opTitle = "Restore"
	}

	// Escape all user-controlled fields to prevent HTML injection
	escapedTarget := html.EscapeString(msg.Target)
	escapedScheduledBy := html.EscapeString(msg.ScheduledBy)
	escapedStorageName := html.EscapeString(msg.StorageName)
	escapedStorageType := html.EscapeString(msg.StorageType)
	escapedDatabaseName := html.EscapeString(msg.DatabaseName)
	escapedDatabaseType := html.EscapeString(msg.DatabaseType)
	escapedError := html.EscapeString(fmt.Sprintf("%v", msg.Error))

	htmlContent := `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #333; }
.container { max-width: 600px; margin: 0 auto; padding: 20px; }
.header { background: #dc3545; color: white; padding: 20px; border-radius: 5px 5px 0 0; }
.header h1 { margin: 0; font-size: 24px; }
.content { background: #f8f9fa; padding: 20px; border-radius: 0 0 5px 5px; }
.section { background: white; padding: 15px; margin-bottom: 15px; border-radius: 5px; border-left: 4px solid #dc3545; }
.section h2 { margin-top: 0; font-size: 16px; color: #dc3545; }
.error-box { background: #fff3cd; border: 1px solid #ffc107; padding: 15px; border-radius: 5px; margin: 15px 0; }
.error-text { color: #856404; font-family: 'Monaco', 'Courier New', monospace; font-size: 14px; word-break: break-word; }
.metric { display: flex; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid #e9ecef; }
.metric:last-child { border-bottom: none; }
.label { font-weight: 600; color: #6c757d; }
.value { color: #212529; font-family: 'Monaco', 'Courier New', monospace; }
.stages { list-style: none; padding: 0; }
.stages li { padding: 8px 0; display: flex; justify-content: space-between; }
.stage-name { font-weight: 500; }
.stage-duration { color: #6c757d; font-size: 14px; }
.footer { text-align: center; padding: 20px; color: #6c757d; font-size: 12px; }
</style>
</head>
<body>
<div class="container">
<div class="header">
<h1>✗ ` + opTitle + ` Failed</h1>
</div>
<div class="content">
<div class="error-box">
<div class="error-text">` + escapedError + `</div>
</div>
<div class="section">
<h2>Details</h2>
<div class="metric"><span class="label">Target</span><span class="value">` + escapedTarget + `</span></div>`

	// Add trigger info
	if msg.Manual {
		htmlContent += `<div class="metric"><span class="label">Trigger</span><span class="value">Manual</span></div>`
	} else if msg.ScheduledBy != "" {
		htmlContent += `<div class="metric"><span class="label">Trigger</span><span class="value">Scheduled (` + escapedScheduledBy + `)</span></div>`
	}

	if msg.Duration > 0 {
		htmlContent += `<div class="metric"><span class="label">Duration</span><span class="value">` + msg.Duration.String() + `</span></div>`
	}

	htmlContent += `<div class="metric"><span class="label">Time</span><span class="value">` + msg.Timestamp.Format("2006-01-02 15:04:05") + `</span></div>
</div>`

	// Database section
	if msg.DatabaseName != "" {
		htmlContent += `<div class="section">
<h2>Database</h2>
<div class="metric"><span class="label">Database Name</span><span class="value">` + escapedDatabaseName + `</span></div>
<div class="metric"><span class="label">Database Type</span><span class="value">` + escapedDatabaseType + `</span></div>
</div>`
	}

	// Storage section
	if msg.StorageName != "" {
		htmlContent += `<div class="section">
<h2>Storage</h2>
<div class="metric"><span class="label">Storage Name</span><span class="value">` + escapedStorageName + `</span></div>
<div class="metric"><span class="label">Storage Type</span><span class="value">` + escapedStorageType + `</span></div>
</div>`
	}

	// Stages section
	if len(msg.Stages) > 0 {
		htmlContent += `<div class="section">
<h2>Stages</h2>
<ul class="stages">`
		for _, stage := range msg.Stages {
			icon := "✓"
			switch stage.Status {
			case "failed":
				icon = "✗"
			case "running":
				icon = "⋯"
			}
			escapedStageName := html.EscapeString(stage.Name)
			htmlContent += `<li><span class="stage-name">` + icon + ` ` + escapedStageName + `</span><span class="stage-duration">` + stage.Duration.String() + `</span></li>`
		}
		htmlContent += `</ul>
</div>`
	}

	htmlContent += `</div>
<div class="footer">
BareD Backup System | ` + time.Now().Format("2006") + `
</div>
</div>
</body>
</html>`

	return htmlContent
}

// sendWithRetry sends email with retry logic
func (e *Email) sendWithRetry(ctx context.Context, subject, body string, maxRetries int) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := e.send(ctx, subject, body); err != nil {
			lastErr = err
			if attempt < maxRetries {
				// Exponential backoff
				backoff := time.Duration(attempt*attempt) * time.Second
				select {
				case <-time.After(backoff):
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		} else {
			return nil // Success
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// send sends the email via SMTP
func (e *Email) send(ctx context.Context, subject, body string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Build email message
	from := e.cfg.SMTPFrom
	to := e.cfg.SMTPTo

	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = strings.Join(to, ", ")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", e.cfg.SMTPHost, e.cfg.SMTPPort)

	var auth smtp.Auth
	if e.cfg.SMTPUsername != "" {
		auth = smtp.PlainAuth("", e.cfg.SMTPUsername, e.cfg.SMTPPassword, e.cfg.SMTPHost)
	}

	// Use TLS if configured
	if e.cfg.SMTPUseTLS {
		tlsConfig := &tls.Config{
			ServerName: e.cfg.SMTPHost,
			MinVersion: tls.VersionTLS12,
		}

		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to connect with TLS: %w", err)
		}
		defer func() {
			// Error closing conn during cleanup is not critical.
			if err := conn.Close(); err != nil {
				// Best-effort cleanup; ignore close errors.
				_ = err
			}
		}()

		client, err := smtp.NewClient(conn, e.cfg.SMTPHost)
		if err != nil {
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
		defer func() {
			// Error closing client during cleanup is not critical.
			if err := client.Close(); err != nil {
				// Best-effort cleanup; ignore close errors.
				_ = err
			}
		}()

		if err := ctx.Err(); err != nil {
			return err
		}
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
		}

		if err := client.Mail(from); err != nil {
			return fmt.Errorf("failed to set sender: %w", err)
		}

		for _, recipient := range to {
			if err := client.Rcpt(recipient); err != nil {
				return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
			}
		}

		writer, err := client.Data()
		if err != nil {
			return fmt.Errorf("failed to start data: %w", err)
		}

		if _, err := writer.Write([]byte(message)); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}

		if err := writer.Close(); err != nil {
			return fmt.Errorf("failed to close writer: %w", err)
		}

		return client.Quit()
	}

	// Non-TLS connection
	if err := ctx.Err(); err != nil {
		return err
	}
	return smtp.SendMail(addr, auth, from, to, []byte(message))
}
