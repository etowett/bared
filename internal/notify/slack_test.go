package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
)

func TestNewSlack(t *testing.T) {
	cfg := &config.Notifier{
		Type:      "slack",
		URL:       "https://hooks.slack.com/services/TEST/WEBHOOK",
		OnSuccess: true,
	}

	slack := NewSlack(cfg)

	require.NotNil(t, slack)
	assert.Equal(t, cfg, slack.cfg)
}

func TestSlack_Name(t *testing.T) {
	slack := NewSlack(&config.Notifier{Type: "slack"})
	assert.Equal(t, "slack", slack.Name())
}

func TestSlack_ShouldNotifySuccess(t *testing.T) {
	tests := []struct {
		name      string
		onSuccess bool
		expected  bool
	}{
		{
			name:      "success notifications enabled",
			onSuccess: true,
			expected:  true,
		},
		{
			name:      "success notifications disabled",
			onSuccess: false,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slack := NewSlack(&config.Notifier{
				Type:      "slack",
				OnSuccess: tt.onSuccess,
			})

			assert.Equal(t, tt.expected, slack.ShouldNotifySuccess())
		})
	}
}

func TestSlack_NotifySuccess(t *testing.T) {
	tests := []struct {
		name         string
		onSuccess    bool
		msg          *Message
		validateFunc func(*testing.T, map[string]interface{})
		wantErr      bool
	}{
		{
			name:      "successful notification",
			onSuccess: true,
			msg: &Message{
				Target:    "mysql-prod",
				Operation: "backup",
				Duration:  5 * time.Second,
				Path:      "/backups/mysql-prod/2025-12-02/backup.sql.tar.gz",
				Timestamp: time.Date(2025, 12, 2, 10, 30, 0, 0, time.UTC),
			},
			validateFunc: func(t *testing.T, payload map[string]interface{}) {
				attachments := payload["attachments"].([]interface{})
				require.Len(t, attachments, 1)

				attachment := attachments[0].(map[string]interface{})
				text := attachment["text"].(string)
				color := attachment["color"].(string)

				assert.Contains(t, text, "✓ *Backup Successful*")
				assert.Contains(t, text, "Target: `mysql-prod`")
				assert.Contains(t, text, "Duration: 5s")
				assert.Contains(t, text, "Path: `/backups/mysql-prod/2025-12-02/backup.sql.tar.gz`")
				assert.Contains(t, text, "Time: 2025-12-02 10:30:00")
				assert.Equal(t, "good", color)
			},
			wantErr: false,
		},
		{
			name:      "success notifications disabled",
			onSuccess: false,
			msg: &Message{
				Target: "mysql-prod",
			},
			validateFunc: func(t *testing.T, payload map[string]interface{}) {
				// Should not be called
				t.Error("HTTP request should not be made when success notifications disabled")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedPayload map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify HTTP method
				assert.Equal(t, "POST", r.Method)

				// Verify Content-Type
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Parse payload
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				err = json.Unmarshal(body, &receivedPayload)
				require.NoError(t, err)

				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			slack := NewSlack(&config.Notifier{
				Type:      "slack",
				URL:       server.URL,
				OnSuccess: tt.onSuccess,
			})

			ctx := context.Background()
			err := slack.NotifySuccess(ctx, tt.msg)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.onSuccess {
					tt.validateFunc(t, receivedPayload)
				}
			}
		})
	}
}

func TestSlack_NotifyFailure(t *testing.T) {
	tests := []struct {
		name         string
		msg          *Message
		validateFunc func(*testing.T, map[string]interface{})
		wantErr      bool
	}{
		{
			name: "failure notification",
			msg: &Message{
				Target:    "postgres-prod",
				Operation: "backup",
				Error:     fmt.Errorf("connection timeout"),
				Timestamp: time.Date(2025, 12, 2, 11, 45, 0, 0, time.UTC),
			},
			validateFunc: func(t *testing.T, payload map[string]interface{}) {
				attachments := payload["attachments"].([]interface{})
				require.Len(t, attachments, 1)

				attachment := attachments[0].(map[string]interface{})
				text := attachment["text"].(string)
				color := attachment["color"].(string)

				assert.Contains(t, text, "✗ *Backup Failed*")
				assert.Contains(t, text, "Target: `postgres-prod`")
				assert.Contains(t, text, "Error: connection timeout")
				assert.Contains(t, text, "Time: 2025-12-02 11:45:00")
				assert.Equal(t, "danger", color)
			},
			wantErr: false,
		},
		{
			name: "failure with complex error",
			msg: &Message{
				Target:    "redis-cache",
				Operation: "restore",
				Error:     fmt.Errorf("failed to restore: %w", fmt.Errorf("file not found")),
				Timestamp: time.Date(2025, 12, 2, 12, 0, 0, 0, time.UTC),
			},
			validateFunc: func(t *testing.T, payload map[string]interface{}) {
				attachments := payload["attachments"].([]interface{})
				attachment := attachments[0].(map[string]interface{})
				text := attachment["text"].(string)

				assert.Contains(t, text, "redis-cache")
				assert.Contains(t, text, "failed to restore")
				assert.Contains(t, text, "file not found")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedPayload map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify HTTP method
				assert.Equal(t, "POST", r.Method)

				// Verify Content-Type
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Parse payload
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				err = json.Unmarshal(body, &receivedPayload)
				require.NoError(t, err)

				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			slack := NewSlack(&config.Notifier{
				Type: "slack",
				URL:  server.URL,
			})

			ctx := context.Background()
			err := slack.NotifyFailure(ctx, tt.msg)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.validateFunc(t, receivedPayload)
			}
		})
	}
}

func TestSlack_HTTPErrors(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectedError  string
	}{
		{
			name: "slack returns 404",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectedError: "slack returned status 404",
		},
		{
			name: "slack returns 500",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedError: "slack returned status 500",
		},
		{
			name: "slack returns 403",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			expectedError: "slack returned status 403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			slack := NewSlack(&config.Notifier{
				Type:      "slack",
				URL:       server.URL,
				OnSuccess: true,
			})

			msg := &Message{
				Target:    "test-target",
				Timestamp: time.Now(),
			}

			ctx := context.Background()
			err := slack.NotifySuccess(ctx, msg)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestSlack_InvalidURL(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		expectedError string
	}{
		{
			name:          "malformed URL",
			url:           "://invalid-url",
			expectedError: "failed to create request",
		},
		{
			name:          "unreachable host",
			url:           "http://localhost:99999",
			expectedError: "failed to send notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slack := NewSlack(&config.Notifier{
				Type:      "slack",
				URL:       tt.url,
				OnSuccess: true,
			})

			msg := &Message{
				Target:    "test-target",
				Timestamp: time.Now(),
			}

			ctx := context.Background()
			err := slack.NotifySuccess(ctx, msg)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestSlack_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slack := NewSlack(&config.Notifier{
		Type:      "slack",
		URL:       server.URL,
		OnSuccess: true,
	})

	msg := &Message{
		Target:    "test-target",
		Timestamp: time.Now(),
	}

	// Create context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := slack.NotifySuccess(ctx, msg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send notification")
}

func TestSlack_ContextTimeout(t *testing.T) {
	// Create a server that delays response longer than timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slack := NewSlack(&config.Notifier{
		Type:      "slack",
		URL:       server.URL,
		OnSuccess: true,
	})

	msg := &Message{
		Target:    "test-target",
		Timestamp: time.Now(),
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := slack.NotifySuccess(ctx, msg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send notification")
}

func TestSlack_MessageFormatting(t *testing.T) {
	tests := []struct {
		name            string
		msg             *Message
		notifyFunc      func(*Slack, context.Context, *Message) error
		expectedStrings []string
		expectedColor   string
	}{
		{
			name: "success with all fields",
			msg: &Message{
				Target:    "mysql-staging",
				Operation: "backup",
				Duration:  10*time.Minute + 30*time.Second,
				Size:      1024 * 1024 * 100, // 100MB
				Path:      "/backups/mysql-staging/2025-12-02T10-30-00Z/db.sql.tar.gz",
				Timestamp: time.Date(2025, 12, 2, 10, 30, 0, 0, time.UTC),
			},
			notifyFunc: func(s *Slack, ctx context.Context, msg *Message) error {
				return s.NotifySuccess(ctx, msg)
			},
			expectedStrings: []string{
				"✓ *Backup Successful*",
				"Target: `mysql-staging`",
				"Duration: 10m30s",
				"Path: `/backups/mysql-staging/2025-12-02T10-30-00Z/db.sql.tar.gz`",
				"Time: 2025-12-02 10:30:00",
			},
			expectedColor: "good",
		},
		{
			name: "failure with error",
			msg: &Message{
				Target:    "postgres-dev",
				Operation: "restore",
				Error:     fmt.Errorf("database connection refused"),
				Timestamp: time.Date(2025, 12, 2, 14, 15, 0, 0, time.UTC),
			},
			notifyFunc: func(s *Slack, ctx context.Context, msg *Message) error {
				return s.NotifyFailure(ctx, msg)
			},
			expectedStrings: []string{
				"✗ *Backup Failed*",
				"Target: `postgres-dev`",
				"Error: database connection refused",
				"Time: 2025-12-02 14:15:00",
			},
			expectedColor: "danger",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedPayload map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				json.Unmarshal(body, &receivedPayload)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			slack := NewSlack(&config.Notifier{
				Type:      "slack",
				URL:       server.URL,
				OnSuccess: true,
			})

			ctx := context.Background()
			err := tt.notifyFunc(slack, ctx, tt.msg)

			require.NoError(t, err)

			attachments := receivedPayload["attachments"].([]interface{})
			attachment := attachments[0].(map[string]interface{})
			text := attachment["text"].(string)
			color := attachment["color"].(string)

			// Verify all expected strings are present
			for _, expected := range tt.expectedStrings {
				assert.Contains(t, text, expected, "Expected text to contain: %s", expected)
			}

			// Verify color
			assert.Equal(t, tt.expectedColor, color)
		})
	}
}

func TestSlack_PayloadStructure(t *testing.T) {
	var receivedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slack := NewSlack(&config.Notifier{
		Type:      "slack",
		URL:       server.URL,
		OnSuccess: true,
	})

	msg := &Message{
		Target:    "test",
		Timestamp: time.Now(),
	}

	ctx := context.Background()
	err := slack.NotifySuccess(ctx, msg)

	require.NoError(t, err)

	// Verify payload structure
	assert.Contains(t, receivedPayload, "attachments")
	attachments, ok := receivedPayload["attachments"].([]interface{})
	require.True(t, ok)
	require.Len(t, attachments, 1)

	attachment, ok := attachments[0].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, attachment, "text")
	assert.Contains(t, attachment, "color")

	text, ok := attachment["text"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, text)

	color, ok := attachment["color"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, color)
}

func TestSlack_ConcurrentNotifications(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slack := NewSlack(&config.Notifier{
		Type:      "slack",
		URL:       server.URL,
		OnSuccess: true,
	})

	// Send 10 concurrent notifications
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			msg := &Message{
				Target:    fmt.Sprintf("target-%d", id),
				Timestamp: time.Now(),
			}
			ctx := context.Background()
			err := slack.NotifySuccess(ctx, msg)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all notifications
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all requests were received
	assert.Equal(t, 10, requestCount)
}

func TestSlack_LargeMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slack := NewSlack(&config.Notifier{
		Type:      "slack",
		URL:       server.URL,
		OnSuccess: true,
	})

	// Create message with very long path
	longPath := "/backups/" + strings.Repeat("very-long-directory-name/", 50) + "backup.sql.tar.gz"
	msg := &Message{
		Target:    "test-target",
		Path:      longPath,
		Timestamp: time.Now(),
	}

	ctx := context.Background()
	err := slack.NotifySuccess(ctx, msg)

	assert.NoError(t, err)
}
