package configservice

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/testutil/fixtures"
)

// Storage config is persisted as a JSON blob plus separately-encrypted secrets.
// A field that is serialised but never read back (or vice versa) silently
// reverts to its zero value on the next daemon start — which for
// insecure_skip_host_key_verify or known_hosts_path would silently change how
// the SFTP connection is verified.
func TestStorageRoundTrip(t *testing.T) {
	svc := &Service{}

	tests := []struct {
		name    string
		storage *config.Storage
	}{
		{
			name: "local",
			storage: &config.Storage{
				Name: "disk", Type: "local", Keep: 5,
				Path: "/data/backups",
			},
		},
		{
			name: "s3",
			storage: &config.Storage{
				Name: "s3", Type: "s3", Keep: 10,
				Bucket: "backups", Region: "us-east-1",
				AccessKeyID: "AKIA", SecretAccessKey: "shhh",
				EndpointURL: "https://s3.example.com",
			},
		},
		{
			name: "sftp with known_hosts and password",
			storage: &config.Storage{
				Name: "offsite", Type: "sftp", Keep: 90,
				Host: "backup.example.com", Port: 22, Username: "backup",
				Password:       "shhh",
				KnownHostsPath: "/etc/bared/known_hosts",
			},
		},
		{
			name: "sftp with a pinned fingerprint and a private key",
			storage: &config.Storage{
				Name: "offsite", Type: "sftp", Keep: 90,
				Host: "backup.example.com", Port: 2222, Username: "backup",
				HostKeyFingerprint:   "SHA256:n3s1XbGVUUdN3iVCQwPq3rXMcTMVLh1nZOtCG0Y5yPo",
				PrivateKeyPath:       "/etc/bared/id_ed25519",
				PrivateKeyPassphrase: "shhh",
			},
		},
		{
			name: "sftp with verification explicitly disabled",
			storage: &config.Storage{
				Name: "lan", Type: "sftp", Keep: 3,
				Host: "10.0.0.5", Port: 22, Username: "backup", Password: "shhh",
				InsecureSkipHostKeyVerify: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configJSON, secrets, err := svc.serializeStorage(tt.storage)
			require.NoError(t, err)

			// Secrets must never end up in the plaintext config blob.
			assert.NotContains(t, configJSON, "shhh")

			secretMap := make(map[string]string, len(secrets))
			for _, s := range secrets {
				secretMap[s.FieldName] = s.Value
			}

			got, err := svc.deserializeStorage(
				tt.storage.Name, tt.storage.Type, configJSON, tt.storage.Keep, secretMap)
			require.NoError(t, err)

			assert.Equal(t, tt.storage, got)
		})
	}
}

// TestStorageRoundTrip_EveryField is the persistence half of the storage
// mapping audit (#104). The table above is hand-written, so it only covers the
// fields somebody remembered to set — it happily passed while `path` was
// dropped for s3 and sftp. This one drives the round trip from
// fixtures.StorageFieldSpecs, which is checked against config.Storage by
// reflection, so a new field cannot be added without being threaded through
// serializeStorage and deserializeStorage.
func TestStorageRoundTrip_EveryField(t *testing.T) {
	fixtures.AssertStorageFieldsEnumerated(t)

	svc := &Service{}

	for _, storageType := range fixtures.StorageTypes {
		t.Run(storageType, func(t *testing.T) {
			sample := fixtures.SampleStorage(t, storageType)

			configJSON, secrets, err := svc.serializeStorage(sample)
			require.NoError(t, err)

			var configMap map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(configJSON), &configMap))
			fixtures.AssertStorageConfigMap(t, storageType, configMap)

			secretMap := make(map[string]string, len(secrets))
			for _, s := range secrets {
				secretMap[s.FieldName] = s.Value
			}
			for _, spec := range fixtures.StorageFieldSpecsFor(storageType) {
				if !spec.Secret {
					continue
				}
				assert.Containsf(t, secretMap, spec.Key,
					"secret %q (config.Storage.%s) was not extracted, so it is never stored", spec.Key, spec.Field)
			}

			got, err := svc.deserializeStorage(sample.Name, sample.Type, configJSON, sample.Keep, secretMap)
			require.NoError(t, err)

			fixtures.AssertStorageRoundTrip(t, storageType, got, true)
		})
	}
}
