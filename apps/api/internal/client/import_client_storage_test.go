package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/testutil/fixtures"
)

// TestStorageToAPIRequest_SendsEveryField is the `brd config import` half of
// the storage mapping audit (#104): the YAML config is turned into an API
// request here, and a field missing from this switch is dropped on its way
// into the database — where it then wins over the YAML at load time, so the
// user's own config file cannot fix it.
func TestStorageToAPIRequest_SendsEveryField(t *testing.T) {
	fixtures.AssertStorageFieldsEnumerated(t)

	for _, storageType := range fixtures.StorageTypes {
		t.Run(storageType, func(t *testing.T) {
			sample := fixtures.SampleStorage(t, storageType)

			req := storageToAPIRequest(sample)

			assert.Equal(t, sample.Name, req.Name)
			assert.Equal(t, sample.Type, req.Type)
			assert.Equal(t, sample.Keep, req.Keep)
			fixtures.AssertStorageConfigMap(t, storageType, req.Config)

			for _, spec := range fixtures.StorageFieldSpecsFor(storageType) {
				if !spec.Secret {
					continue
				}
				want, ok := spec.Sample.(string)
				require.Truef(t, ok, "secret %q sample must be a string", spec.Key)

				switch spec.Key {
				case "secret_access_key":
					assert.Equal(t, want, req.SecretAccessKey)
				case "password":
					assert.Equal(t, want, req.Password)
				case "private_key_passphrase":
					assert.Equal(t, want, req.PrivateKeyPassphrase)
				default:
					t.Fatalf("secret %q (config.Storage.%s) is not carried by storageToAPIRequest: "+
						"`brd config import` drops it", spec.Key, spec.Field)
				}
			}
		})
	}
}
