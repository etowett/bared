package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindTarget(t *testing.T) {
	cfg := &Config{
		Targets: []*Target{
			{Name: "db1"},
			{Name: "db2"},
			{Name: "db3"},
		},
	}

	tests := []struct {
		name       string
		targetName string
		wantFound  bool
		wantIndex  int
	}{
		{
			name:       "existing target - first",
			targetName: "db1",
			wantFound:  true,
			wantIndex:  0,
		},
		{
			name:       "existing target - middle",
			targetName: "db2",
			wantFound:  true,
			wantIndex:  1,
		},
		{
			name:       "existing target - last",
			targetName: "db3",
			wantFound:  true,
			wantIndex:  2,
		},
		{
			name:       "non-existing target",
			targetName: "nonexistent",
			wantFound:  false,
		},
		{
			name:       "empty name",
			targetName: "",
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := cfg.FindTarget(tt.targetName)
			if tt.wantFound {
				require.NoError(t, err, "expected no error finding target")
				require.NotNil(t, target, "expected to find target")
				assert.Equal(t, tt.targetName, target.Name)
			} else {
				assert.Error(t, err, "expected error for non-existent target")
				assert.Nil(t, target, "expected nil target")
			}
		})
	}
}

func TestFindStorage(t *testing.T) {
	cfg := &Config{
		Storages: map[string]*Storage{
			"local": {Type: "local"},
			"s3":    {Type: "s3"},
		},
	}

	tests := []struct {
		name        string
		storageName string
		wantFound   bool
	}{
		{
			name:        "existing storage - local",
			storageName: "local",
			wantFound:   true,
		},
		{
			name:        "existing storage - s3",
			storageName: "s3",
			wantFound:   true,
		},
		{
			name:        "non-existing storage",
			storageName: "nonexistent",
			wantFound:   false,
		},
		{
			name:        "empty name",
			storageName: "",
			wantFound:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := cfg.FindStorage(tt.storageName)
			if tt.wantFound {
				require.NoError(t, err, "expected no error finding storage")
				require.NotNil(t, storage, "expected to find storage")
				assert.Equal(t, cfg.Storages[tt.storageName].Type, storage.Type)
			} else {
				assert.Error(t, err, "expected error for non-existent storage")
				assert.Nil(t, storage, "expected nil storage")
			}
		})
	}
}

func TestGetStorageForTarget(t *testing.T) {
	cfg := &Config{
		DefaultStorage: "default_storage",
		Storages: map[string]*Storage{
			"default_storage": {Type: "local", Path: "/default"},
			"custom_storage":  {Type: "s3", Bucket: "custom"},
		},
		Targets: []*Target{
			{
				Name: "target_with_storage",
				Storage: &TargetStorage{
					Enabled: true,
					Name:    "custom_storage",
				},
			},
			{
				Name: "target_without_storage",
				Storage: &TargetStorage{
					Enabled: false,
				},
			},
			{
				Name:    "target_with_nil_storage",
				Storage: nil,
			},
		},
	}

	tests := []struct {
		name            string
		target          *Target
		wantStorageName string
		wantNil         bool
	}{
		{
			name:            "target with explicit storage",
			target:          cfg.Targets[0],
			wantStorageName: "custom_storage",
			wantNil:         false,
		},
		{
			name:            "target with storage disabled - uses default",
			target:          cfg.Targets[1],
			wantStorageName: "default_storage",
			wantNil:         false,
		},
		{
			name:            "target with nil storage - uses default",
			target:          cfg.Targets[2],
			wantStorageName: "default_storage",
			wantNil:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := cfg.GetStorageForTarget(tt.target)
			if tt.wantNil {
				assert.Error(t, err)
				assert.Nil(t, storage)
			} else {
				require.NoError(t, err, "expected no error getting storage")
				require.NotNil(t, storage, "expected to get storage")
				expected := cfg.Storages[tt.wantStorageName]
				assert.Equal(t, expected, storage)
			}
		})
	}
}

func TestGetStorageForTarget_NoDefault(t *testing.T) {
	// Test when default storage is not set and target doesn't specify storage
	cfg := &Config{
		Storages: map[string]*Storage{
			"some_storage": {Type: "local"},
		},
		Targets: []*Target{
			{
				Name:    "target",
				Storage: nil,
			},
		},
	}

	storage, err := cfg.GetStorageForTarget(cfg.Targets[0])
	assert.Error(t, err, "expected error when no default and target has no storage")
	assert.Nil(t, storage)
}

func TestGetStorageForTarget_InvalidStorageName(t *testing.T) {
	// Test when target specifies non-existent storage
	cfg := &Config{
		DefaultStorage: "default",
		Storages: map[string]*Storage{
			"default": {Type: "local"},
		},
		Targets: []*Target{
			{
				Name: "target",
				Storage: &TargetStorage{
					Enabled: true,
					Name:    "nonexistent",
				},
			},
		},
	}

	storage, err := cfg.GetStorageForTarget(cfg.Targets[0])
	assert.Error(t, err, "expected error when storage name doesn't exist")
	assert.Nil(t, storage)
}
