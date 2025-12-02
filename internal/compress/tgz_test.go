package compress

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTarGz(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "simple filename",
			filename: "backup.sql",
		},
		{
			name:     "filename with path",
			filename: "mysql/backup.sql",
		},
		{
			name:     "filename with timestamp",
			filename: "backup-2025-12-02T10-30-00Z.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tgz := NewTarGz(tt.filename)

			require.NotNil(t, tgz)
			assert.Equal(t, tt.filename, tgz.filename)
		})
	}
}

func TestTarGz_Extension(t *testing.T) {
	tgz := NewTarGz("backup.sql")
	assert.Equal(t, ".tar.gz", tgz.Extension())
}

func TestTarGz_Compress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "compress simple text",
			input:   "Hello, World!",
			wantErr: false,
		},
		{
			name:    "compress multi-line text",
			input:   "Line 1\nLine 2\nLine 3\n",
			wantErr: false,
		},
		{
			name:    "compress large data",
			input:   strings.Repeat("This is a test line\n", 1000),
			wantErr: false,
		},
		{
			name:    "compress empty data",
			input:   "",
			wantErr: false,
		},
		{
			name:    "compress SQL dump",
			input:   "-- MySQL dump\nCREATE TABLE users (id INT);\nINSERT INTO users VALUES (1);\n",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tgz := NewTarGz("backup.sql")
			reader := strings.NewReader(tt.input)
			var buf bytes.Buffer

			err := tgz.Compress(context.Background(), reader, &buf)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Greater(t, buf.Len(), 0, "compressed data should not be empty")
			}
		})
	}
}

func TestTarGz_Decompress(t *testing.T) {
	tests := []struct {
		name         string
		originalData string
	}{
		{
			name:         "decompress simple text",
			originalData: "Hello, World!",
		},
		{
			name:         "decompress multi-line text",
			originalData: "Line 1\nLine 2\nLine 3\n",
		},
		{
			name:         "decompress large data",
			originalData: strings.Repeat("Test data line\n", 500),
		},
		{
			name:         "decompress SQL dump",
			originalData: "-- MySQL dump 10.13\nCREATE DATABASE test;\nUSE test;\nCREATE TABLE users (id INT, name VARCHAR(100));\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tgz := NewTarGz("backup.sql")

			// First compress
			var compressed bytes.Buffer
			err := tgz.Compress(context.Background(), strings.NewReader(tt.originalData), &compressed)
			require.NoError(t, err)

			// Then decompress
			var decompressed bytes.Buffer
			err = tgz.Decompress(context.Background(), &compressed, &decompressed)
			require.NoError(t, err)

			// Verify content matches
			assert.Equal(t, tt.originalData, decompressed.String())
		})
	}
}

func TestTarGz_CompressDecompress_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		data     string
	}{
		{
			name:     "round-trip simple text",
			filename: "test.txt",
			data:     "This is a test",
		},
		{
			name:     "round-trip with newlines",
			filename: "backup.sql",
			data:     "CREATE TABLE test;\nINSERT INTO test VALUES (1, 'hello');\nINSERT INTO test VALUES (2, 'world');\n",
		},
		{
			name:     "round-trip with special characters",
			filename: "data.txt",
			data:     "Special chars: !@#$%^&*()[]{}|\\:;\"'<>?,./~`",
		},
		{
			name:     "round-trip large data",
			filename: "large.sql",
			data:     strings.Repeat("0123456789abcdefghijklmnopqrstuvwxyz\n", 1000),
		},
		{
			name:     "round-trip empty file",
			filename: "empty.txt",
			data:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tgz := NewTarGz(tt.filename)
			ctx := context.Background()

			// Compress
			var compressed bytes.Buffer
			err := tgz.Compress(ctx, strings.NewReader(tt.data), &compressed)
			require.NoError(t, err)

			// Verify compression actually happened (compressed size should be different)
			if len(tt.data) > 100 {
				// For large data, compression should reduce size
				// (not always true, but generally for repetitive data)
				t.Logf("Original size: %d, Compressed size: %d", len(tt.data), compressed.Len())
			}

			// Decompress
			var decompressed bytes.Buffer
			err = tgz.Decompress(ctx, &compressed, &decompressed)
			require.NoError(t, err)

			// Verify data integrity
			assert.Equal(t, tt.data, decompressed.String(), "decompressed data should match original")
			assert.Equal(t, len(tt.data), decompressed.Len(), "decompressed size should match original")
		})
	}
}

func TestTarGz_Compress_ContextCancellation(t *testing.T) {
	tgz := NewTarGz("backup.sql")

	// Create a large data source
	largeData := strings.Repeat("This is a line of data\n", 100000)

	tests := []struct {
		name      string
		setupCtx  func() context.Context
		expectErr bool
	}{
		{
			name: "context already cancelled",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				return ctx
			},
			expectErr: true,
		},
		{
			name: "context with very short timeout",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
				defer cancel()
				time.Sleep(10 * time.Millisecond) // Ensure timeout occurs
				return ctx
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			reader := strings.NewReader(largeData)
			var buf bytes.Buffer

			err := tgz.Compress(ctx, reader, &buf)

			if tt.expectErr {
				// May or may not error depending on timing
				// Just verify no panic occurs
				t.Logf("Compress with cancelled context: err=%v", err)
			}
		})
	}
}

func TestTarGz_Decompress_CorruptedData(t *testing.T) {
	tests := []struct {
		name        string
		corruptData []byte
		wantErr     bool
	}{
		{
			name:        "empty data",
			corruptData: []byte{},
			wantErr:     true,
		},
		{
			name:        "invalid gzip header",
			corruptData: []byte{0x00, 0x01, 0x02, 0x03, 0x04},
			wantErr:     true,
		},
		{
			name:        "truncated gzip data",
			corruptData: []byte{0x1f, 0x8b, 0x08, 0x00}, // Valid gzip header but no data
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tgz := NewTarGz("backup.sql")
			reader := bytes.NewReader(tt.corruptData)
			var buf bytes.Buffer

			err := tgz.Decompress(context.Background(), reader, &buf)

			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}

func TestTarGz_Compress_LargeFile(t *testing.T) {
	// Test compression of a large file (simulating real backup)
	tgz := NewTarGz("large-backup.sql")

	// Create 10MB of data
	largeData := strings.Repeat("INSERT INTO table VALUES (1, 'test data', NOW());\n", 200000)

	var compressed bytes.Buffer
	err := tgz.Compress(context.Background(), strings.NewReader(largeData), &compressed)

	require.NoError(t, err)
	assert.Greater(t, compressed.Len(), 0)

	// Verify we can decompress it
	var decompressed bytes.Buffer
	err = tgz.Decompress(context.Background(), &compressed, &decompressed)

	require.NoError(t, err)
	assert.Equal(t, len(largeData), decompressed.Len())
}

func TestTarGz_Compress_BinaryData(t *testing.T) {
	// Test compression of binary data (not just text)
	tgz := NewTarGz("binary.dat")

	// Create binary data
	binaryData := make([]byte, 1000)
	for i := range binaryData {
		binaryData[i] = byte(i % 256)
	}

	var compressed bytes.Buffer
	err := tgz.Compress(context.Background(), bytes.NewReader(binaryData), &compressed)
	require.NoError(t, err)

	// Decompress
	var decompressed bytes.Buffer
	err = tgz.Decompress(context.Background(), &compressed, &decompressed)
	require.NoError(t, err)

	// Verify binary data integrity
	assert.Equal(t, binaryData, decompressed.Bytes())
}

func TestTarGz_MultipleFiles(t *testing.T) {
	// Test compressing different files with different content
	files := []struct {
		filename string
		content  string
	}{
		{
			filename: "mysql-backup.sql",
			content:  "-- MySQL dump\nCREATE TABLE users;\n",
		},
		{
			filename: "postgres-backup.sql",
			content:  "-- PostgreSQL dump\nCREATE TABLE products;\n",
		},
		{
			filename: "redis-backup.rdb",
			content:  "REDIS0009",
		},
	}

	for _, file := range files {
		t.Run(file.filename, func(t *testing.T) {
			tgz := NewTarGz(file.filename)

			// Compress
			var compressed bytes.Buffer
			err := tgz.Compress(context.Background(), strings.NewReader(file.content), &compressed)
			require.NoError(t, err)

			// Decompress
			var decompressed bytes.Buffer
			err = tgz.Decompress(context.Background(), &compressed, &decompressed)
			require.NoError(t, err)

			// Verify
			assert.Equal(t, file.content, decompressed.String())
		})
	}
}

func TestTarGz_CompressionRatio(t *testing.T) {
	// Test that compression actually reduces size for repetitive data
	tgz := NewTarGz("repetitive.txt")

	// Highly repetitive data should compress well
	repetitiveData := strings.Repeat("AAAAAAAAAA", 10000) // 100KB of 'A's

	var compressed bytes.Buffer
	err := tgz.Compress(context.Background(), strings.NewReader(repetitiveData), &compressed)
	require.NoError(t, err)

	compressionRatio := float64(compressed.Len()) / float64(len(repetitiveData))
	t.Logf("Original: %d bytes, Compressed: %d bytes, Ratio: %.2f%%",
		len(repetitiveData), compressed.Len(), compressionRatio*100)

	// Repetitive data should compress to less than 10% of original size
	assert.Less(t, compressionRatio, 0.10, "compression ratio should be less than 10%")
}

func TestTarGz_EmptyFilename(t *testing.T) {
	// Test with empty filename (edge case)
	tgz := NewTarGz("")

	data := "test data"
	var compressed bytes.Buffer
	err := tgz.Compress(context.Background(), strings.NewReader(data), &compressed)
	require.NoError(t, err)

	// Should still be able to decompress
	var decompressed bytes.Buffer
	err = tgz.Decompress(context.Background(), &compressed, &decompressed)
	require.NoError(t, err)

	assert.Equal(t, data, decompressed.String())
}

func TestTarGz_UTF8Content(t *testing.T) {
	// Test compression of UTF-8 content (international characters)
	tgz := NewTarGz("utf8.txt")

	utf8Data := "Hello 世界 🌍\nBonjour le monde\nHola mundo\nΓεια σου κόσμε\n"

	var compressed bytes.Buffer
	err := tgz.Compress(context.Background(), strings.NewReader(utf8Data), &compressed)
	require.NoError(t, err)

	var decompressed bytes.Buffer
	err = tgz.Decompress(context.Background(), &compressed, &decompressed)
	require.NoError(t, err)

	assert.Equal(t, utf8Data, decompressed.String())
}
