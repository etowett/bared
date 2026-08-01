package configservice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/encryption"
	"github.com/etowett/bared/apps/api/internal/testutil/configdb"
)

// newTestService builds a Service over a real, throwaway SQLite database so the
// secret round-trip is exercised end to end rather than mocked — a mocked
// secrets table would not have caught that updateSecrets deletes before it
// inserts.
func newTestService(t *testing.T) *Service {
	t.Helper()

	enc, err := encryption.NewService(configdb.TestKey)
	require.NoError(t, err)

	return NewService(configdb.New(t), enc)
}

// The dashboard never gets a secret back — the API replaces it with
// "***REDACTED***" — so its forms only send one when the user retypes it.
// UpdateStorage replaces the row's entire secret set, so without a carry-forward
// step an edit that only changed `keep` erased the S3 credential and every
// subsequent backup failed to authenticate.
func TestCarryForwardStorageSecrets_S3KeyOutlivesAnUnrelatedEdit(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	require.NoError(t, svc.CreateStorage(ctx, &config.Storage{
		Name: "offsite", Type: "s3", Keep: 5,
		Bucket: "backups", Region: "us-east-1", Path: "prefix",
		AccessKeyID: "AKIA", SecretAccessKey: "original-secret",
	}))

	// Exactly what StorageForm sends when the user edits `keep` and leaves the
	// secret field untouched: no secret_access_key at all.
	edited := &config.Storage{
		Name: "offsite", Type: "s3", Keep: 9,
		Bucket: "backups", Region: "us-east-1", Path: "prefix",
		AccessKeyID: "AKIA",
	}
	require.NoError(t, svc.CarryForwardStorageSecrets(ctx, "offsite", edited))
	assert.Equal(t, "original-secret", edited.SecretAccessKey,
		"the stored key must survive an edit that never touched it")

	require.NoError(t, svc.UpdateStorage(ctx, "offsite", edited))

	reloaded, err := svc.GetStorage(ctx, "offsite")
	require.NoError(t, err)
	assert.Equal(t, "original-secret", reloaded.SecretAccessKey)
	assert.Equal(t, 9, reloaded.Keep, "the edit the user did make must still land")
	assert.Equal(t, "prefix", reloaded.Path)
}

// ValidateStorage requires a password or a private key for SFTP, so before the
// carry-forward this edit was rejected with HTTP 400 and the user could not
// change anything on a password-authenticated SFTP backend without retyping the
// password.
func TestCarryForwardStorageSecrets_SFTPPasswordSatisfiesValidation(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	require.NoError(t, svc.CreateStorage(ctx, &config.Storage{
		Name: "remote", Type: "sftp", Keep: 5,
		Host: "backup.example.com", Port: 22, Username: "backup",
		Path: "/srv/backups", Password: "original-password",
		InsecureSkipHostKeyVerify: true,
	}))

	edited := &config.Storage{
		Name: "remote", Type: "sftp", Keep: 7,
		Host: "backup.example.com", Port: 22, Username: "backup",
		Path: "/srv/backups", InsecureSkipHostKeyVerify: true,
	}
	require.Error(t, ValidateStorage(edited), "guard: the request alone must fail validation")

	require.NoError(t, svc.CarryForwardStorageSecrets(ctx, "remote", edited))
	assert.Equal(t, "original-password", edited.Password)
	assert.NoError(t, ValidateStorage(edited))
}

func TestCarryForwardStorageSecrets_DoesNotOverrideARetypedSecret(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	require.NoError(t, svc.CreateStorage(ctx, &config.Storage{
		Name: "offsite", Type: "s3", Keep: 5,
		Bucket: "backups", Region: "us-east-1",
		AccessKeyID: "AKIA", SecretAccessKey: "original-secret",
	}))

	rotated := &config.Storage{
		Name: "offsite", Type: "s3", Keep: 5,
		Bucket: "backups", Region: "us-east-1",
		AccessKeyID: "AKIA", SecretAccessKey: "rotated-secret",
	}
	require.NoError(t, svc.CarryForwardStorageSecrets(ctx, "offsite", rotated))
	assert.Equal(t, "rotated-secret", rotated.SecretAccessKey)
}

// An unknown name is the update handler's problem to report, not this one's.
func TestCarryForwardStorageSecrets_UnknownNameIsNotAnError(t *testing.T) {
	svc := newTestService(t)

	storage := &config.Storage{Name: "ghost", Type: "local", Path: "/tmp"}
	assert.NoError(t, svc.CarryForwardStorageSecrets(context.Background(), "ghost", storage))
}

// validateWebhookAuth demands the credential for the configured auth type, so
// before the carry-forward every edit of an authenticated webhook notifier —
// even just toggling on_success — came back as HTTP 400.
func TestCarryForwardNotifierSecrets_WebhookAuthSatisfiesValidation(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	require.NoError(t, svc.CreateNotifier(ctx, "hook", &config.Notifier{
		Type: "webhook", OnSuccess: true,
		URL: "https://api.example.com/hook", WebhookMethod: "POST",
		WebhookAuth: &config.WebhookAuth{
			Type: "basic", Username: "bot", Password: "original-password",
		},
	}))

	edited := &config.Notifier{
		Type: "webhook", OnSuccess: false,
		URL: "https://api.example.com/hook", WebhookMethod: "POST",
		WebhookAuth: &config.WebhookAuth{Type: "basic", Username: "bot"},
	}
	require.Error(t, ValidateNotifier(edited), "guard: the request alone must fail validation")

	require.NoError(t, svc.CarryForwardNotifierSecrets(ctx, "hook", edited))
	assert.Equal(t, "original-password", edited.WebhookAuth.Password)
	assert.NoError(t, ValidateNotifier(edited))

	require.NoError(t, svc.UpdateNotifier(ctx, "hook", edited))

	reloaded, err := svc.GetNotifier(ctx, "hook")
	require.NoError(t, err)
	assert.Equal(t, "original-password", reloaded.WebhookAuth.Password)
	assert.False(t, reloaded.OnSuccess, "the edit the user did make must still land")
}

// ValidateNotifier never asked for smtp_password, so this update returned 200
// and destroyed the credential silently — the worse of the two failure modes.
func TestCarryForwardNotifierSecrets_SMTPPasswordSurvivesAnUnrelatedEdit(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	require.NoError(t, svc.CreateNotifier(ctx, "mail", &config.Notifier{
		Type: "email", OnSuccess: false,
		SMTPHost: "smtp.example.com", SMTPPort: 587, SMTPUsername: "u",
		SMTPFrom: "a@example.com", SMTPTo: []string{"b@example.com"},
		SMTPPassword: "original-password",
	}))

	edited := &config.Notifier{
		Type: "email", OnSuccess: true,
		SMTPHost: "smtp.example.com", SMTPPort: 587, SMTPUsername: "u",
		SMTPFrom: "a@example.com", SMTPTo: []string{"b@example.com"},
	}
	require.NoError(t, svc.CarryForwardNotifierSecrets(ctx, "mail", edited))
	require.NoError(t, svc.UpdateNotifier(ctx, "mail", edited))

	reloaded, err := svc.GetNotifier(ctx, "mail")
	require.NoError(t, err)
	assert.Equal(t, "original-password", reloaded.SMTPPassword)
	assert.True(t, reloaded.OnSuccess)
}

// Switching auth type must not resurrect the credential the user moved away
// from: carrying all three forward would leave a superseded password encrypted
// in the secrets table indefinitely.
func TestCarryForwardNotifierSecrets_SwitchingAuthTypeDropsTheOldCredential(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	require.NoError(t, svc.CreateNotifier(ctx, "hook", &config.Notifier{
		Type: "webhook", URL: "https://api.example.com/hook", WebhookMethod: "POST",
		WebhookAuth: &config.WebhookAuth{
			Type: "basic", Username: "bot", Password: "original-password",
		},
	}))

	switched := &config.Notifier{
		Type: "webhook", URL: "https://api.example.com/hook", WebhookMethod: "POST",
		WebhookAuth: &config.WebhookAuth{Type: "bearer", Token: "new-token"},
	}
	require.NoError(t, svc.CarryForwardNotifierSecrets(ctx, "hook", switched))
	assert.Equal(t, "new-token", switched.WebhookAuth.Token)
	assert.Empty(t, switched.WebhookAuth.Password)

	require.NoError(t, svc.UpdateNotifier(ctx, "hook", switched))

	reloaded, err := svc.GetNotifier(ctx, "hook")
	require.NoError(t, err)
	assert.Equal(t, "new-token", reloaded.WebhookAuth.Token)
	assert.Empty(t, reloaded.WebhookAuth.Password)
}

// A storage's name is the YAML map key; the decoder does not populate the
// struct field from it. ImportFromYAML skipped that assignment, so
// POST /api/config/migrate — the dashboard's "migrate to database" button —
// failed with "storage name is required" for every config written in the
// documented shape, including examples/config.example.yml.
func TestImportFromYAML_TakesStorageNameFromTheMapKey(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	err := svc.ImportFromYAML(ctx, &config.Config{
		Storages: map[string]*config.Storage{
			"local_disk": {Type: "local", Path: "/data/backups", Keep: 20},
		},
	})
	require.NoError(t, err)

	stored, err := svc.GetStorage(ctx, "local_disk")
	require.NoError(t, err)
	assert.Equal(t, "local_disk", stored.Name)
	assert.Equal(t, "/data/backups", stored.Path)
}
