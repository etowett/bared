package notify

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
)

func TestNew_Slack(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Notifier
		wantErr bool
	}{
		{
			name: "valid slack notifier",
			cfg: &config.Notifier{
				Type:      "slack",
				URL:       "https://hooks.slack.com/services/TEST/WEBHOOK",
				OnSuccess: true,
			},
			wantErr: false,
		},
		{
			name: "slack with minimal config",
			cfg: &config.Notifier{
				Type: "slack",
				URL:  "https://hooks.slack.com/services/TEST",
			},
			wantErr: false,
		},
		{
			name: "slack with all options",
			cfg: &config.Notifier{
				Type:      "slack",
				URL:       "https://hooks.slack.com/services/FULL/CONFIG/URL",
				OnSuccess: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := New(tt.cfg)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, notifier)
			} else {
				require.NoError(t, err)
				require.NotNil(t, notifier)

				// Verify it's a Slack notifier
				slack, ok := notifier.(*Slack)
				require.True(t, ok, "Expected notifier to be *Slack type")
				assert.Equal(t, "slack", slack.Name())
				assert.Equal(t, tt.cfg, slack.cfg)
			}
		})
	}
}

func TestNew_UnsupportedType(t *testing.T) {
	tests := []struct {
		name          string
		notifierType  string
		expectedError string
	}{
		{
			name:          "unsupported discord type",
			notifierType:  "discord",
			expectedError: "unsupported notifier type: discord",
		},
		{
			name:          "empty type",
			notifierType:  "",
			expectedError: "unsupported notifier type: ",
		},
		{
			name:          "random string",
			notifierType:  "random-notifier",
			expectedError: "unsupported notifier type: random-notifier",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Notifier{
				Type: tt.notifierType,
				URL:  "https://example.com/webhook",
			}

			notifier, err := New(cfg)

			require.Error(t, err)
			assert.Nil(t, notifier)
			assert.Equal(t, tt.expectedError, err.Error())
		})
	}
}

func TestNew_EmailNotifier(t *testing.T) {
	cfg := &config.Notifier{
		Type:         "email",
		OnSuccess:    true,
		SMTPHost:     "smtp.gmail.com",
		SMTPPort:     587,
		SMTPUsername: "user@example.com",
		SMTPPassword: "password",
		SMTPFrom:     "backups@example.com",
		SMTPTo:       []string{"admin@example.com"},
		SMTPUseTLS:   true,
	}

	notifier, err := New(cfg)

	require.NoError(t, err)
	require.NotNil(t, notifier)

	email, ok := notifier.(*Email)
	require.True(t, ok, "Expected notifier to be *Email type")
	assert.Equal(t, "email", email.Name())
	assert.True(t, email.ShouldNotifySuccess())
}

func TestNew_WebhookNotifier(t *testing.T) {
	cfg := &config.Notifier{
		Type:           "webhook",
		URL:            "https://api.example.com/webhooks/backups",
		OnSuccess:      true,
		WebhookMethod:  "POST",
		WebhookHeaders: map[string]string{"Content-Type": "application/json"},
	}

	notifier, err := New(cfg)

	require.NoError(t, err)
	require.NotNil(t, notifier)

	webhook, ok := notifier.(*Webhook)
	require.True(t, ok, "Expected notifier to be *Webhook type")
	assert.Equal(t, "webhook", webhook.Name())
	assert.True(t, webhook.ShouldNotifySuccess())
}

func TestNew_MultipleDifferentNotifiers(t *testing.T) {
	// Test creating multiple different notifier instances
	configs := []*config.Notifier{
		{
			Type:      "slack",
			URL:       "https://hooks.slack.com/services/A/B/C",
			OnSuccess: true,
		},
		{
			Type:      "slack",
			URL:       "https://hooks.slack.com/services/X/Y/Z",
			OnSuccess: false,
		},
		{
			Type: "slack",
			URL:  "https://hooks.slack.com/services/1/2/3",
		},
	}

	notifiers := make([]Notifier, 0, len(configs))

	for i, cfg := range configs {
		notifier, err := New(cfg)
		require.NoError(t, err, "Failed to create notifier %d", i)
		require.NotNil(t, notifier, "Notifier %d is nil", i)
		notifiers = append(notifiers, notifier)
	}

	// Verify all notifiers were created
	assert.Len(t, notifiers, 3)

	// Verify each has the correct config
	for i, notifier := range notifiers {
		slack, ok := notifier.(*Slack)
		require.True(t, ok, "Notifier %d is not *Slack", i)
		assert.Equal(t, configs[i], slack.cfg, "Notifier %d has wrong config", i)
	}
}

func TestNew_NotifierInterface(t *testing.T) {
	cfg := &config.Notifier{
		Type:      "slack",
		URL:       "https://hooks.slack.com/services/TEST",
		OnSuccess: true,
	}

	notifier, err := New(cfg)

	require.NoError(t, err)
	require.NotNil(t, notifier)

	// Verify notifier implements the Notifier interface
	assert.Equal(t, "slack", notifier.Name())
	assert.Equal(t, true, notifier.ShouldNotifySuccess())
}

func TestNew_CaseSensitivity(t *testing.T) {
	tests := []struct {
		name         string
		notifierType string
		wantErr      bool
		expectedErr  string
	}{
		{
			name:         "lowercase slack",
			notifierType: "slack",
			wantErr:      false,
		},
		{
			name:         "uppercase SLACK",
			notifierType: "SLACK",
			wantErr:      true,
			expectedErr:  "unsupported notifier type: SLACK",
		},
		{
			name:         "mixed case Slack",
			notifierType: "Slack",
			wantErr:      true,
			expectedErr:  "unsupported notifier type: Slack",
		},
		{
			name:         "mixed case sLaCk",
			notifierType: "sLaCk",
			wantErr:      true,
			expectedErr:  "unsupported notifier type: sLaCk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Notifier{
				Type: tt.notifierType,
				URL:  "https://hooks.slack.com/services/TEST",
			}

			notifier, err := New(cfg)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, notifier)
				assert.Equal(t, tt.expectedErr, err.Error())
			} else {
				require.NoError(t, err)
				require.NotNil(t, notifier)
			}
		})
	}
}

func TestNew_PreservesConfiguration(t *testing.T) {
	cfg := &config.Notifier{
		Type:      "slack",
		URL:       "https://hooks.slack.com/services/PRESERVE/TEST",
		OnSuccess: true,
	}

	notifier, err := New(cfg)

	require.NoError(t, err)
	require.NotNil(t, notifier)

	slack := notifier.(*Slack)

	// Verify configuration is preserved exactly
	assert.Equal(t, cfg.Type, slack.cfg.Type)
	assert.Equal(t, cfg.URL, slack.cfg.URL)
	assert.Equal(t, cfg.OnSuccess, slack.cfg.OnSuccess)
	assert.Equal(t, cfg, slack.cfg, "Configuration should be preserved as-is")
}

func TestNew_WithEmptyURL(t *testing.T) {
	// Factory should still create notifier even with empty URL
	// (validation happens at notification time, not creation time)
	cfg := &config.Notifier{
		Type:      "slack",
		URL:       "",
		OnSuccess: true,
	}

	notifier, err := New(cfg)

	require.NoError(t, err)
	require.NotNil(t, notifier)

	slack := notifier.(*Slack)
	assert.Equal(t, "", slack.cfg.URL)
}

func TestNew_FactoryReturnsConsistentTypes(t *testing.T) {
	// Create same notifier multiple times
	cfg := &config.Notifier{
		Type: "slack",
		URL:  "https://hooks.slack.com/services/CONSISTENT",
	}

	notifier1, err1 := New(cfg)
	notifier2, err2 := New(cfg)
	notifier3, err3 := New(cfg)

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)

	// All should be Slack type
	_, ok1 := notifier1.(*Slack)
	_, ok2 := notifier2.(*Slack)
	_, ok3 := notifier3.(*Slack)

	assert.True(t, ok1)
	assert.True(t, ok2)
	assert.True(t, ok3)

	// All should have the same name
	assert.Equal(t, "slack", notifier1.Name())
	assert.Equal(t, "slack", notifier2.Name())
	assert.Equal(t, "slack", notifier3.Name())
}
