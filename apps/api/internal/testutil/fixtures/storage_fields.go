package fixtures

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/etowett/bared/apps/api/internal/config"
)

// StorageFieldSpec describes one field of config.Storage for the storage
// mapping audit.
//
// config.Storage is converted back and forth in five places — the API request
// and response mappings (internal/api/config_handlers.go), the database
// serialize/deserialize pair (internal/configservice/secrets.go) and the
// `brd config import` request mapping (internal/client/import_client.go). Each
// of them is a hand-written switch over the storage type, and every time a
// field was missed from one of them a user's value was silently dropped:
// `username` and `endpoint_url` (#95), then `path` (#104).
//
// StorageFieldSpecs is the single declaration of what each field means, and
// AssertStorageFieldsEnumerated fails the moment config.Storage grows, loses
// or renames a field without this table being updated. Adding the entry then
// forces the round-trip tests in api, configservice and client to exercise the
// new field in every mapping — so the next missing field is a red test, not a
// backup written to the wrong place.
type StorageFieldSpec struct {
	// Field is the Go field name on config.Storage.
	Field string

	// Key is the field's key inside the type-specific config map used by the
	// API (`StorageRequest.Config`) and by the serialized DB column. Empty
	// means the field travels outside that map (name, type, keep) or, for
	// secrets, in a dedicated request field / the secrets table.
	Key string

	// Types lists the storage types the field applies to. Empty means all.
	Types []string

	// Secret marks values that must never appear verbatim in an API response
	// or in the plaintext config column: they are redacted on the way out and
	// carried out of band on the way in.
	Secret bool

	// Sample is a distinctive non-zero value used to populate fixtures. Every
	// mapping must carry it through unchanged.
	Sample interface{}
}

// StorageTypes are the storage types every mapping switches over.
var StorageTypes = []string{"local", "s3", "sftp"}

// StorageFieldSpecs enumerates every field of config.Storage. Keep it in sync
// with the struct — AssertStorageFieldsEnumerated enforces that.
var StorageFieldSpecs = []StorageFieldSpec{
	{Field: "Type", Sample: "local"}, // overridden per storage type
	{Field: "Name", Sample: "audit-store"},
	{Field: "Path", Key: "path", Types: []string{"local", "s3", "sftp"}, Sample: "/srv/backups/audit"},
	{Field: "Keep", Sample: 11},

	{Field: "Bucket", Key: "bucket", Types: []string{"s3"}, Sample: "audit-bucket"},
	{Field: "Region", Key: "region", Types: []string{"s3"}, Sample: "eu-west-1"},
	{Field: "AccessKeyID", Key: "access_key_id", Types: []string{"s3"}, Sample: "AKIAAUDITKEYID"},
	{Field: "SecretAccessKey", Key: "secret_access_key", Types: []string{"s3"}, Secret: true, Sample: "audit-s3-secret"},
	{Field: "EndpointURL", Key: "endpoint_url", Types: []string{"s3"}, Sample: "https://s3.audit.example.com"},

	{Field: "Host", Key: "host", Types: []string{"sftp"}, Sample: "sftp.audit.example.com"},
	{Field: "Port", Key: "port", Types: []string{"sftp"}, Sample: 2222},
	{Field: "Username", Key: "username", Types: []string{"sftp"}, Sample: "audit-user"},
	{Field: "Password", Key: "password", Types: []string{"sftp"}, Secret: true, Sample: "audit-sftp-password"},
	{
		Field: "KnownHostsPath", Key: "known_hosts_path", Types: []string{"sftp"},
		Sample: "/etc/bared/known_hosts",
	},
	{
		Field: "HostKeyFingerprint", Key: "host_key_fingerprint", Types: []string{"sftp"},
		Sample: "SHA256:n3s1XbGVUUdN3iVCQwPq3rXMcTMVLh1nZOtCG0Y5yPo",
	},
	{
		Field: "InsecureSkipHostKeyVerify", Key: "insecure_skip_host_key_verify", Types: []string{"sftp"},
		Sample: true,
	},
	{
		Field: "PrivateKeyPath", Key: "private_key_path", Types: []string{"sftp"},
		Sample: "/etc/bared/ssh/id_ed25519",
	},
	{
		Field: "PrivateKeyPassphrase", Key: "private_key_passphrase", Types: []string{"sftp"},
		Secret: true, Sample: "audit-key-passphrase",
	},
}

// AppliesTo reports whether the field is used by the given storage type.
func (spec StorageFieldSpec) AppliesTo(storageType string) bool {
	if len(spec.Types) == 0 {
		return true
	}
	for _, t := range spec.Types {
		if t == storageType {
			return true
		}
	}
	return false
}

// StorageFieldSpecsFor returns the specs that apply to a storage type.
func StorageFieldSpecsFor(storageType string) []StorageFieldSpec {
	var out []StorageFieldSpec
	for _, spec := range StorageFieldSpecs {
		if spec.AppliesTo(storageType) {
			out = append(out, spec)
		}
	}
	return out
}

// AssertStorageFieldsEnumerated fails when config.Storage and
// StorageFieldSpecs have drifted apart: a new or renamed field with no spec,
// or a spec naming a field that no longer exists. Call it from every package
// that round-trips storage config, so adding a field to config.Storage turns
// every one of those mappings red until the field is threaded through.
func AssertStorageFieldsEnumerated(t *testing.T) {
	t.Helper()

	specByField := make(map[string]StorageFieldSpec, len(StorageFieldSpecs))
	for _, spec := range StorageFieldSpecs {
		if _, dup := specByField[spec.Field]; dup {
			t.Errorf("StorageFieldSpecs: duplicate entry for config.Storage.%s", spec.Field)
		}
		specByField[spec.Field] = spec
	}

	storageType := reflect.TypeOf(config.Storage{})
	seen := make(map[string]bool, storageType.NumField())
	for i := 0; i < storageType.NumField(); i++ {
		field := storageType.Field(i)
		if !field.IsExported() {
			continue
		}
		seen[field.Name] = true

		spec, ok := specByField[field.Name]
		if !ok {
			t.Errorf(
				"config.Storage.%s has no StorageFieldSpec: add it to StorageFieldSpecs and thread it through "+
					"every storage mapping (api requestToStorage/storageToResponse, configservice "+
					"serializeStorage/deserializeStorage, client storageToAPIRequest) — see #104",
				field.Name)
			continue
		}

		sampleType := reflect.TypeOf(spec.Sample)
		if sampleType == nil || !sampleType.AssignableTo(field.Type) {
			t.Errorf("StorageFieldSpec for %s: sample %v is not assignable to %s",
				field.Name, spec.Sample, field.Type)
		}
		if reflect.ValueOf(spec.Sample).IsZero() {
			t.Errorf("StorageFieldSpec for %s: sample must be non-zero so a dropped field is detectable",
				field.Name)
		}
	}

	for _, spec := range StorageFieldSpecs {
		if !seen[spec.Field] {
			t.Errorf("StorageFieldSpecs names config.Storage.%s, which does not exist (renamed or removed?)",
				spec.Field)
		}
	}
}

// SampleStorage builds a config.Storage of the given type with every field
// that applies to that type set to its sample value, and everything else left
// zero.
func SampleStorage(t *testing.T, storageType string) *config.Storage {
	t.Helper()

	storage := &config.Storage{}
	value := reflect.ValueOf(storage).Elem()
	for _, spec := range StorageFieldSpecsFor(storageType) {
		field := value.FieldByName(spec.Field)
		if !field.IsValid() {
			t.Fatalf("StorageFieldSpecs names unknown field config.Storage.%s", spec.Field)
		}
		field.Set(reflect.ValueOf(spec.Sample))
	}
	storage.Type = storageType

	return storage
}

// SampleStorageConfigMap builds the type-specific config map matching
// SampleStorage: every applicable non-secret field keyed as the API and the
// database store it. Secrets are excluded — they travel out of band.
func SampleStorageConfigMap(t *testing.T, storageType string) map[string]interface{} {
	t.Helper()

	configMap := make(map[string]interface{})
	for _, spec := range StorageFieldSpecsFor(storageType) {
		if spec.Key == "" || spec.Secret {
			continue
		}
		configMap[spec.Key] = normalizeConfigValue(spec.Sample)
	}

	return configMap
}

// AssertStorageRoundTrip checks that got carries every field of the sample
// storage of this type. Secrets are checked only when includeSecrets is set —
// the response mapping deliberately redacts them.
func AssertStorageRoundTrip(t *testing.T, storageType string, got *config.Storage, includeSecrets bool) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s: round trip produced a nil storage", storageType)
	}

	want := SampleStorage(t, storageType)
	wantValue := reflect.ValueOf(want).Elem()
	gotValue := reflect.ValueOf(got).Elem()

	for _, spec := range StorageFieldSpecsFor(storageType) {
		if spec.Secret && !includeSecrets {
			continue
		}
		wantField := wantValue.FieldByName(spec.Field).Interface()
		gotField := gotValue.FieldByName(spec.Field).Interface()
		if !reflect.DeepEqual(wantField, gotField) {
			t.Errorf("%s storage: config.Storage.%s was dropped or mangled: got %#v, want %#v",
				storageType, spec.Field, gotField, wantField)
		}
	}
}

// AssertStorageConfigMap checks a type-specific config map (an API
// StorageRequest.Config / StorageResponse.Config, or the serialized DB config
// column) built from SampleStorage: every applicable non-secret field with a
// map key must be present and equal, and no secret may appear verbatim.
func AssertStorageConfigMap(t *testing.T, storageType string, configMap map[string]interface{}) {
	t.Helper()

	for _, spec := range StorageFieldSpecsFor(storageType) {
		if spec.Key == "" {
			continue
		}

		got, ok := configMap[spec.Key]
		if spec.Secret {
			if ok && got == spec.Sample {
				t.Errorf("%s storage: secret %q leaked verbatim into the config map", storageType, spec.Key)
			}
			continue
		}

		if !ok {
			t.Errorf("%s storage: config map is missing %q (config.Storage.%s) — the value is dropped",
				storageType, spec.Key, spec.Field)
			continue
		}
		if normalizeConfigValue(got) != normalizeConfigValue(spec.Sample) {
			t.Errorf("%s storage: config map %q = %#v, want %#v", storageType, spec.Key, got, spec.Sample)
		}
	}
}

// normalizeConfigValue makes values comparable across a JSON round trip, where
// ints come back as float64.
func normalizeConfigValue(v interface{}) interface{} {
	switch typed := v.(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return v
	}
}
