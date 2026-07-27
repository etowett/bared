package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/testutil/fixtures"
)

// Regression for #104: an SFTP backend created through the API dropped the
// remote path, so backups landed in the SSH login directory instead.
func TestRequestToStorage_SFTPKeepsPath(t *testing.T) {
	s := &Server{}

	storage := s.requestToStorage(&StorageRequest{
		Name: "offsite",
		Type: "sftp",
		Keep: 7,
		Config: map[string]interface{}{
			"host":     "backup.example.com",
			"port":     float64(22),
			"username": "backup",
			"path":     "/srv/backups",
		},
		Password: "hunter2",
	})

	assert.Equal(t, "/srv/backups", storage.Path)
}

// TestStorageFieldsAreEnumerated fails when config.Storage grows a field that
// the mapping audit does not know about. See fixtures.StorageFieldSpecs.
func TestStorageFieldsAreEnumerated(t *testing.T) {
	fixtures.AssertStorageFieldsEnumerated(t)
}

// sampleStorageRequest builds the StorageRequest a client would send for a
// fully-populated storage of this type: non-secret fields in Config, secrets
// in their dedicated top-level fields.
func sampleStorageRequest(t *testing.T, storageType string) *StorageRequest {
	t.Helper()

	sample := fixtures.SampleStorage(t, storageType)
	req := &StorageRequest{
		Name:   sample.Name,
		Type:   sample.Type,
		Keep:   sample.Keep,
		Config: fixtures.SampleStorageConfigMap(t, storageType),
	}

	for _, spec := range fixtures.StorageFieldSpecsFor(storageType) {
		if !spec.Secret {
			continue
		}
		value, ok := spec.Sample.(string)
		require.Truef(t, ok, "secret %q sample must be a string", spec.Key)

		switch spec.Key {
		case "secret_access_key":
			req.SecretAccessKey = value
		case "password":
			req.Password = value
		case "private_key_passphrase":
			req.PrivateKeyPassphrase = value
		default:
			t.Fatalf("secret %q (config.Storage.%s) has no StorageRequest field: add one and read it in "+
				"requestToStorage, or the value never reaches the daemon", spec.Key, spec.Field)
		}
	}

	return req
}

// TestRequestToStorage_MapsEveryField walks every field of config.Storage that
// applies to a storage type and asserts requestToStorage carries it across.
// This is the inbound half of the mapping audit (#104).
func TestRequestToStorage_MapsEveryField(t *testing.T) {
	s := &Server{}

	for _, storageType := range fixtures.StorageTypes {
		t.Run(storageType, func(t *testing.T) {
			storage := s.requestToStorage(sampleStorageRequest(t, storageType))

			fixtures.AssertStorageRoundTrip(t, storageType, storage, true)
		})
	}
}

// TestStorageToResponse_EmitsEveryField is the outbound half: every non-secret
// field must come back so the dashboard pre-fills the edit form, and every
// secret must be redacted.
func TestStorageToResponse_EmitsEveryField(t *testing.T) {
	s := &Server{}

	for _, storageType := range fixtures.StorageTypes {
		t.Run(storageType, func(t *testing.T) {
			sample := fixtures.SampleStorage(t, storageType)

			resp := s.storageToResponse(sample)

			assert.Equal(t, sample.Name, resp.Name)
			assert.Equal(t, sample.Type, resp.Type)
			assert.Equal(t, sample.Keep, resp.Keep)
			fixtures.AssertStorageConfigMap(t, storageType, resp.Config)
		})
	}
}

// TestStorageRequestResponseRoundTrip is the loop the dashboard actually
// performs: create a backend, read it back, submit the edit form again. The
// response goes over the wire as JSON, so it is re-decoded here rather than
// handed back as Go values. Everything but the secrets must survive unchanged,
// and the redacted placeholders must not be written back as real values.
func TestStorageRequestResponseRoundTrip(t *testing.T) {
	s := &Server{}

	for _, storageType := range fixtures.StorageTypes {
		t.Run(storageType, func(t *testing.T) {
			created := s.requestToStorage(sampleStorageRequest(t, storageType))
			resp := s.storageToResponse(created)

			encoded, err := json.Marshal(resp)
			require.NoError(t, err)
			var decoded StorageRequest
			require.NoError(t, json.Unmarshal(encoded, &decoded))

			resubmitted := s.requestToStorage(&decoded)

			fixtures.AssertStorageRoundTrip(t, storageType, resubmitted, false)

			assert.NotContains(t, resubmitted.Password, "REDACTED")
			assert.NotContains(t, resubmitted.SecretAccessKey, "REDACTED")
			assert.NotContains(t, resubmitted.PrivateKeyPassphrase, "REDACTED")
		})
	}
}
