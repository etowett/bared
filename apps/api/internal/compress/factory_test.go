package compress

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_TarGz(t *testing.T) {
	tests := []struct {
		name         string
		compressType string
		filename     string
	}{
		{
			name:         "type tgz",
			compressType: "tgz",
			filename:     "backup.sql",
		},
		{
			name:         "type tar.gz",
			compressType: "tar.gz",
			filename:     "backup.sql",
		},
		{
			name:         "with path in filename",
			compressType: "tgz",
			filename:     "mysql/backup-2025-12-02.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressor, err := New(tt.compressType, tt.filename)

			require.NoError(t, err)
			require.NotNil(t, compressor)

			// Verify it's a TarGz compressor
			tgz, ok := compressor.(*TarGz)
			require.True(t, ok, "compressor should be *TarGz type")
			assert.Equal(t, tt.filename, tgz.filename)
			assert.Equal(t, ".tar.gz", compressor.Extension())
		})
	}
}

func TestNew_Gzip(t *testing.T) {
	tests := []struct {
		name         string
		compressType string
		filename     string
	}{
		{
			name:         "type gzip",
			compressType: "gzip",
			filename:     "backup.sql",
		},
		{
			name:         "type gz",
			compressType: "gz",
			filename:     "backup.sql",
		},
		{
			name:         "empty type defaults to gzip",
			compressType: "",
			filename:     "backup.sql",
		},
		{
			name:         "with path in filename",
			compressType: "gzip",
			filename:     "mysql/backup-2025-12-02.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressor, err := New(tt.compressType, tt.filename)

			require.NoError(t, err)
			require.NotNil(t, compressor)

			// Verify it's a Gzip compressor
			gz, ok := compressor.(*Gzip)
			require.True(t, ok, "compressor should be *Gzip type")
			assert.Equal(t, tt.filename, gz.filename)
			assert.Equal(t, ".gz", compressor.Extension())
		})
	}
}

func TestNew_UnsupportedType(t *testing.T) {
	tests := []struct {
		name         string
		compressType string
	}{
		{
			name:         "zip type",
			compressType: "zip",
		},
		{
			name:         "bzip2 type",
			compressType: "bz2",
		},
		{
			name:         "unknown type",
			compressType: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressor, err := New(tt.compressType, "backup.sql")

			assert.Error(t, err)
			assert.Nil(t, compressor)
			assert.Contains(t, err.Error(), "unsupported compression type")
			assert.Contains(t, err.Error(), tt.compressType)
		})
	}
}

func TestNew_CaseSensitivity(t *testing.T) {
	tests := []struct {
		name         string
		compressType string
		wantErr      bool
	}{
		{
			name:         "lowercase tgz",
			compressType: "tgz",
			wantErr:      false,
		},
		{
			name:         "uppercase TGZ",
			compressType: "TGZ",
			wantErr:      true, // Case-sensitive
		},
		{
			name:         "mixed case Tgz",
			compressType: "Tgz",
			wantErr:      true,
		},
		{
			name:         "lowercase tar.gz",
			compressType: "tar.gz",
			wantErr:      false,
		},
		{
			name:         "uppercase TAR.GZ",
			compressType: "TAR.GZ",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressor, err := New(tt.compressType, "backup.sql")

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, compressor)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, compressor)
			}
		})
	}
}

func TestNew_BothAliases(t *testing.T) {
	// Test that both "tgz" and "tar.gz" create the same type
	compressor1, err := New("tgz", "backup.sql")
	require.NoError(t, err)

	compressor2, err := New("tar.gz", "backup.sql")
	require.NoError(t, err)

	// Both should be TarGz type
	_, ok1 := compressor1.(*TarGz)
	_, ok2 := compressor2.(*TarGz)
	assert.True(t, ok1)
	assert.True(t, ok2)

	// Both should return same extension
	assert.Equal(t, compressor1.Extension(), compressor2.Extension())
}

func TestNew_MultipleInstances(t *testing.T) {
	// Test creating multiple compressor instances
	filenames := []string{
		"backup1.sql",
		"backup2.sql",
		"backup3.sql",
	}

	compressors := make([]Compressor, len(filenames))

	for i, filename := range filenames {
		var err error
		compressors[i], err = New("tgz", filename)
		require.NoError(t, err)
		require.NotNil(t, compressors[i])
	}

	// All should be independent instances
	for i := 0; i < len(compressors); i++ {
		for j := i + 1; j < len(compressors); j++ {
			assert.NotEqual(t, compressors[i], compressors[j])
		}
	}
}

func TestNew_EmptyFilename(t *testing.T) {
	// Test with empty filename (should succeed)
	compressor, err := New("tgz", "")

	require.NoError(t, err)
	require.NotNil(t, compressor)

	tgz, ok := compressor.(*TarGz)
	require.True(t, ok)
	assert.Equal(t, "", tgz.filename)
}

func TestNew_SpecialCharactersInFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "filename with spaces",
			filename: "backup file.sql",
		},
		{
			name:     "filename with special chars",
			filename: "backup-2025-12-02_10:30:00.sql",
		},
		{
			name:     "filename with unicode",
			filename: "备份.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressor, err := New("tgz", tt.filename)

			require.NoError(t, err)
			require.NotNil(t, compressor)

			tgz, ok := compressor.(*TarGz)
			require.True(t, ok)
			assert.Equal(t, tt.filename, tgz.filename)
		})
	}
}

func TestNew_ErrorMessage(t *testing.T) {
	compressor, err := New("unsupported", "backup.sql")

	require.Error(t, err)
	assert.Nil(t, compressor)
	assert.Equal(t, "unsupported compression type: unsupported", err.Error())
}

func TestNew_InterfaceCompliance(t *testing.T) {
	// Test that the returned compressor implements the Compressor interface
	compressor, err := New("tgz", "backup.sql")
	require.NoError(t, err)

	// Verify interface methods are available
	assert.NotNil(t, compressor.Compress)
	assert.NotNil(t, compressor.Decompress)
	assert.NotNil(t, compressor.Extension)

	// Verify Extension returns expected value
	assert.Equal(t, ".tar.gz", compressor.Extension())
}

func TestNew_CommonFilenames(t *testing.T) {
	// Test with common backup filename patterns
	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "mysql dump",
			filename: "mysql-backup.sql",
		},
		{
			name:     "postgres dump",
			filename: "postgres-backup.sql",
		},
		{
			name:     "redis dump",
			filename: "redis-backup.rdb",
		},
		{
			name:     "timestamped backup",
			filename: "backup-2025-12-02T10-30-00Z.sql",
		},
		{
			name:     "target specific",
			filename: "production-mysql-backup.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressor, err := New("tgz", tt.filename)

			require.NoError(t, err)
			require.NotNil(t, compressor)

			tgz, ok := compressor.(*TarGz)
			require.True(t, ok)
			assert.Equal(t, tt.filename, tgz.filename)
		})
	}
}
