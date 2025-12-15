package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bared/internal/config"
)

// Webhook implements Notifier for generic webhook notifications
type Webhook struct {
	cfg *config.Notifier
}

// NewWebhook creates a new Webhook notifier
func NewWebhook(cfg *config.Notifier) *Webhook {
	return &Webhook{cfg: cfg}
}

// Name returns the notifier name
func (w *Webhook) Name() string {
	return "webhook"
}

// ShouldNotifySuccess returns true if success notifications are enabled
func (w *Webhook) ShouldNotifySuccess() bool {
	return w.cfg.OnSuccess
}

// NotifySuccess sends a success notification via webhook
func (w *Webhook) NotifySuccess(ctx context.Context, msg *Message) error {
	if !w.cfg.OnSuccess {
		return nil
	}

	event := "backup.success"
	if msg.Operation == "restore" {
		event = "restore.success"
	}

	return w.sendWithRetry(ctx, event, msg, 3)
}

// NotifyFailure sends a failure notification via webhook
func (w *Webhook) NotifyFailure(ctx context.Context, msg *Message) error {
	event := "backup.failure"
	if msg.Operation == "restore" {
		event = "restore.failure"
	}

	return w.sendWithRetry(ctx, event, msg, 3)
}

// WebhookPayload represents the webhook JSON payload
type WebhookPayload struct {
	Event           string                 `json:"event"`
	Timestamp       string                 `json:"timestamp"`
	Target          string                 `json:"target"`
	Operation       string                 `json:"operation"`
	Status          string                 `json:"status"`
	DurationSeconds float64                `json:"duration_seconds"`
	Error           string                 `json:"error,omitempty"`
	Manual          bool                   `json:"manual"`
	ScheduledBy     string                 `json:"scheduled_by,omitempty"`
	DryRun          bool                   `json:"dry_run,omitempty"`
	Backup          *BackupDetails         `json:"backup,omitempty"`
	Restore         *RestoreDetails        `json:"restore,omitempty"`
	Database        *DatabaseDetails       `json:"database,omitempty"`
	Storage         *StorageDetails        `json:"storage,omitempty"`
	Stages          []StageDetails         `json:"stages,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// BackupDetails contains backup-specific details
type BackupDetails struct {
	Path             string  `json:"path"`
	Size             int64   `json:"size"`
	UncompressedSize int64   `json:"uncompressed_size,omitempty"`
	CompressionRatio float64 `json:"compression_ratio,omitempty"`
}

// RestoreDetails contains restore-specific details
type RestoreDetails struct {
	BackupPath        string   `json:"backup_path"`
	BackupSize        int64    `json:"backup_size"`
	Validations       []string `json:"validations,omitempty"`
	ValidationsPassed int      `json:"validations_passed"`
}

// DatabaseDetails contains database information
type DatabaseDetails struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// StorageDetails contains storage information
type StorageDetails struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

// StageDetails contains stage execution information
type StageDetails struct {
	Name            string  `json:"name"`
	Status          string  `json:"status"`
	DurationSeconds float64 `json:"duration_seconds"`
}

// buildPayload builds the webhook payload from message
func (w *Webhook) buildPayload(event string, msg *Message) *WebhookPayload {
	status := "success"
	if msg.Error != nil {
		status = "failure"
	}

	payload := &WebhookPayload{
		Event:           event,
		Timestamp:       msg.Timestamp.Format(time.RFC3339),
		Target:          msg.Target,
		Operation:       msg.Operation,
		Status:          status,
		DurationSeconds: msg.Duration.Seconds(),
		Manual:          msg.Manual,
		ScheduledBy:     msg.ScheduledBy,
		DryRun:          msg.DryRun,
		Metadata:        make(map[string]interface{}),
	}

	if msg.Error != nil {
		payload.Error = msg.Error.Error()
	}

	// Database details
	if msg.DatabaseName != "" {
		payload.Database = &DatabaseDetails{
			Name: msg.DatabaseName,
			Type: msg.DatabaseType,
		}
	}

	// Storage details
	if msg.StorageName != "" {
		payload.Storage = &StorageDetails{
			Name: msg.StorageName,
			Type: msg.StorageType,
			Path: msg.StoragePath,
		}
	}

	// Operation-specific details
	if msg.Operation == "backup" {
		payload.Backup = &BackupDetails{
			Path:             msg.Path,
			Size:             msg.Size,
			UncompressedSize: msg.UncompressedSize,
			CompressionRatio: msg.CompressionRatio,
		}
	} else if msg.Operation == "restore" {
		payload.Restore = &RestoreDetails{
			BackupPath:        msg.Path,
			BackupSize:        msg.Size,
			Validations:       msg.Validations,
			ValidationsPassed: msg.ValidationsPassed,
		}
	}

	// Stages
	if len(msg.Stages) > 0 {
		payload.Stages = make([]StageDetails, len(msg.Stages))
		for i, stage := range msg.Stages {
			payload.Stages[i] = StageDetails{
				Name:            stage.Name,
				Status:          stage.Status,
				DurationSeconds: stage.Duration.Seconds(),
			}
		}
	}

	return payload
}

// sendWithRetry sends webhook with retry logic
func (w *Webhook) sendWithRetry(ctx context.Context, event string, msg *Message, maxRetries int) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := w.send(ctx, event, msg); err != nil {
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

// send sends the webhook HTTP request
func (w *Webhook) send(ctx context.Context, event string, msg *Message) error {
	payload := w.buildPayload(event, msg)

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Determine HTTP method
	method := w.cfg.WebhookMethod
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequestWithContext(ctx, method, w.cfg.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set default Content-Type
	req.Header.Set("Content-Type", "application/json")

	// Add custom headers
	for key, value := range w.cfg.WebhookHeaders {
		req.Header.Set(key, value)
	}

	// Add authentication
	if w.cfg.WebhookAuth != nil {
		if err := w.addAuth(req); err != nil {
			return fmt.Errorf("failed to add authentication: %w", err)
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	// Consider 2xx status codes as success
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// addAuth adds authentication to the request
func (w *Webhook) addAuth(req *http.Request) error {
	auth := w.cfg.WebhookAuth
	if auth == nil {
		return nil
	}

	switch auth.Type {
	case "basic":
		if auth.Username == "" || auth.Password == "" {
			return fmt.Errorf("basic auth requires username and password")
		}
		req.SetBasicAuth(auth.Username, auth.Password)

	case "bearer":
		if auth.Token == "" {
			return fmt.Errorf("bearer auth requires token")
		}
		req.Header.Set("Authorization", "Bearer "+auth.Token)

	case "header":
		if auth.HeaderName == "" || auth.HeaderValue == "" {
			return fmt.Errorf("header auth requires header_name and header_value")
		}
		req.Header.Set(auth.HeaderName, auth.HeaderValue)

	default:
		return fmt.Errorf("unsupported auth type: %s", auth.Type)
	}

	return nil
}
