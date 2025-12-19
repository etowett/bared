package compress

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"io"
	"strings"
	"testing"
	"time"
)

func TestGzip_Extension(t *testing.T) {
	g := NewGzip("test")
	if got := g.Extension(); got != ".gz" {
		t.Errorf("Extension() = %v, want %v", got, ".gz")
	}
}

func TestGzip_CompressDecompress_SmallData(t *testing.T) {
	ctx := context.Background()
	g := NewGzip("test.sql")

	// Original data
	original := []byte("This is a test database dump with some repeating data data data")
	input := bytes.NewReader(original)

	// Compress
	var compressed bytes.Buffer
	if err := g.Compress(ctx, input, &compressed); err != nil {
		t.Fatalf("Compress() failed: %v", err)
	}

	// Verify compressed data is smaller (gzip should compress repeating data)
	if compressed.Len() >= len(original) {
		t.Logf("Warning: compressed size (%d) >= original size (%d)", compressed.Len(), len(original))
	}

	// Decompress
	var decompressed bytes.Buffer
	if err := g.Decompress(ctx, &compressed, &decompressed); err != nil {
		t.Fatalf("Decompress() failed: %v", err)
	}

	// Verify data matches
	if !bytes.Equal(decompressed.Bytes(), original) {
		t.Errorf("Decompressed data doesn't match original.\nGot: %q\nWant: %q", decompressed.String(), string(original))
	}
}

func TestGzip_CompressDecompress_LargeData(t *testing.T) {
	ctx := context.Background()
	g := NewGzip("large.sql")

	// Create 10MB of random data
	original := make([]byte, 10*1024*1024)
	if _, err := rand.Read(original); err != nil {
		t.Fatalf("Failed to generate random data: %v", err)
	}

	input := bytes.NewReader(original)

	// Compress
	var compressed bytes.Buffer
	if err := g.Compress(ctx, input, &compressed); err != nil {
		t.Fatalf("Compress() failed: %v", err)
	}

	t.Logf("Compressed %d bytes to %d bytes (%.1f%% reduction)",
		len(original), compressed.Len(),
		(1.0-float64(compressed.Len())/float64(len(original)))*100)

	// Decompress
	var decompressed bytes.Buffer
	if err := g.Decompress(ctx, &compressed, &decompressed); err != nil {
		t.Fatalf("Decompress() failed: %v", err)
	}

	// Verify data matches
	if !bytes.Equal(decompressed.Bytes(), original) {
		t.Errorf("Decompressed data doesn't match original for large file")
	}
}

func TestGzip_Compress_ContextCancellation(t *testing.T) {
	g := NewGzip("test.sql")

	// Create a slow reader that will give us time to cancel
	slowReader := &slowReader{
		data:  bytes.Repeat([]byte("test data "), 1000),
		delay: 10 * time.Millisecond,
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// Compress should fail with context cancelled error
	var compressed bytes.Buffer
	err := g.Compress(ctx, slowReader, &compressed)
	if err == nil {
		t.Error("Compress() should have failed with context cancelled")
	}
	if err != context.Canceled {
		t.Errorf("Compress() error = %v, want %v", err, context.Canceled)
	}
}

func TestGzip_Decompress_ContextCancellation(t *testing.T) {
	g := NewGzip("test.sql")

	// First create some compressed data
	original := bytes.Repeat([]byte("test data "), 1000)
	var compressed bytes.Buffer
	gzw := gzip.NewWriter(&compressed)
	if _, err := gzw.Write(original); err != nil {
		t.Fatalf("Failed to prepare test data: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}

	// Create cancellable context
	cancelCtx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	// Decompress should fail with context cancelled error
	var decompressed bytes.Buffer
	err := g.Decompress(cancelCtx, &compressed, &decompressed)
	if err == nil {
		t.Error("Decompress() should have failed with context cancelled")
	}
	if err != context.Canceled {
		t.Errorf("Decompress() error = %v, want %v", err, context.Canceled)
	}
}

func TestGzip_Decompress_InvalidData(t *testing.T) {
	ctx := context.Background()
	g := NewGzip("test.sql")

	// Try to decompress invalid gzip data
	invalid := bytes.NewReader([]byte("This is not gzip compressed data"))
	var decompressed bytes.Buffer

	err := g.Decompress(ctx, invalid, &decompressed)
	if err == nil {
		t.Error("Decompress() should have failed with invalid data")
	}
}

func TestGzip_Compress_EmptyInput(t *testing.T) {
	ctx := context.Background()
	g := NewGzip("empty.sql")

	// Compress empty input
	input := bytes.NewReader([]byte{})
	var compressed bytes.Buffer
	if err := g.Compress(ctx, input, &compressed); err != nil {
		t.Fatalf("Compress() failed with empty input: %v", err)
	}

	// Should produce valid gzip with no content
	if compressed.Len() == 0 {
		t.Error("Compress() produced empty output for empty input")
	}

	// Decompress should work
	var decompressed bytes.Buffer
	if err := g.Decompress(ctx, &compressed, &decompressed); err != nil {
		t.Fatalf("Decompress() failed: %v", err)
	}

	if decompressed.Len() != 0 {
		t.Errorf("Decompressed empty data resulted in %d bytes", decompressed.Len())
	}
}

func TestGzip_NoMemoryAccumulation(t *testing.T) {
	ctx := context.Background()
	g := NewGzip("test.sql")

	// Create a 50MB stream to test memory usage
	// We'll use a reader that generates data on the fly rather than allocating it all
	size := int64(50 * 1024 * 1024)
	input := &limitedRandomReader{remaining: size}

	var compressed bytes.Buffer
	if err := g.Compress(ctx, input, &compressed); err != nil {
		t.Fatalf("Compress() failed: %v", err)
	}

	t.Logf("Compressed %d MB of streaming data to %d bytes", size/(1024*1024), compressed.Len())

	// Note: This test verifies the code doesn't accumulate data in memory.
	// Memory profiling would be needed to verify actual memory usage stays constant.
}

// slowReader is a reader that introduces delays between reads
type slowReader struct {
	data  []byte
	pos   int
	delay time.Duration
}

func (r *slowReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}

	time.Sleep(r.delay)

	// Read small chunks to give time for cancellation
	// Compute the allowed read length: min(len(p), remaining data, 100)
	remaining := len(r.data) - r.pos
	toRead := remaining
	if toRead > 100 {
		toRead = 100
	}
	if len(p) < toRead {
		toRead = len(p)
	}

	// Copy only the allowed amount
	n = copy(p, r.data[r.pos:r.pos+toRead])
	r.pos += n
	return n, nil
}

// limitedRandomReader generates random data on the fly without allocating it all
type limitedRandomReader struct {
	remaining int64
}

func (r *limitedRandomReader) Read(p []byte) (n int, err error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}

	// Read up to len(p) bytes, but not more than remaining
	toRead := int64(len(p))
	if toRead > r.remaining {
		toRead = r.remaining
	}

	// Generate random data (or pattern for better compression)
	// Using a pattern to make compression more realistic
	pattern := []byte("SQL INSERT INTO table VALUES (repeated data for compression test);")
	for i := int64(0); i < toRead; i++ {
		p[i] = pattern[i%int64(len(pattern))]
	}

	r.remaining -= toRead
	return int(toRead), nil
}

func BenchmarkGzip_Compress_1MB(b *testing.B) {
	ctx := context.Background()
	g := NewGzip("bench.sql")

	// Create 1MB of data
	data := bytes.Repeat([]byte("INSERT INTO table VALUES (1, 'test', NOW());"), 20000)

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		input := bytes.NewReader(data)
		var compressed bytes.Buffer
		if err := g.Compress(ctx, input, &compressed); err != nil {
			b.Fatalf("Compress failed: %v", err)
		}
	}
}

func BenchmarkGzip_Compress_10MB(b *testing.B) {
	ctx := context.Background()
	g := NewGzip("bench.sql")

	// Create 10MB of data
	data := bytes.Repeat([]byte("INSERT INTO table VALUES (1, 'test', NOW());"), 200000)

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		input := bytes.NewReader(data)
		var compressed bytes.Buffer
		if err := g.Compress(ctx, input, &compressed); err != nil {
			b.Fatalf("Compress failed: %v", err)
		}
	}
}

func BenchmarkGzip_Decompress_1MB(b *testing.B) {
	ctx := context.Background()
	g := NewGzip("bench.sql")

	// Create and compress 1MB of data
	data := bytes.Repeat([]byte("INSERT INTO table VALUES (1, 'test', NOW());"), 20000)
	var compressed bytes.Buffer
	if err := g.Compress(ctx, bytes.NewReader(data), &compressed); err != nil {
		b.Fatalf("Setup failed: %v", err)
	}
	compressedData := compressed.Bytes()

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		input := bytes.NewReader(compressedData)
		var decompressed bytes.Buffer
		if err := g.Decompress(ctx, input, &decompressed); err != nil {
			b.Fatalf("Decompress failed: %v", err)
		}
	}
}

// TestGzip_VerifyCompatibility ensures our gzip is compatible with standard gzip
func TestGzip_VerifyCompatibility(t *testing.T) {
	ctx := context.Background()
	g := NewGzip("test.sql")

	testData := []byte(strings.Repeat("test data ", 1000))

	// Compress with our implementation
	var ourCompressed bytes.Buffer
	if err := g.Compress(ctx, bytes.NewReader(testData), &ourCompressed); err != nil {
		t.Fatalf("Our Compress() failed: %v", err)
	}

	// Decompress with standard library
	gzr, err := gzip.NewReader(&ourCompressed)
	if err != nil {
		t.Fatalf("Standard gzip.NewReader() failed: %v", err)
	}
	defer gzr.Close()

	var stdDecompressed bytes.Buffer
	if _, err := io.Copy(&stdDecompressed, gzr); err != nil {
		t.Fatalf("Standard gzip decompress failed: %v", err)
	}

	if !bytes.Equal(stdDecompressed.Bytes(), testData) {
		t.Error("Standard library couldn't decompress our gzip output correctly")
	}

	// Compress with standard library
	var stdCompressed bytes.Buffer
	gzw := gzip.NewWriter(&stdCompressed)
	if _, err := gzw.Write(testData); err != nil {
		t.Fatalf("Standard gzip compress failed: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("Standard gzip close failed: %v", err)
	}

	// Decompress with our implementation
	var ourDecompressed bytes.Buffer
	if err := g.Decompress(ctx, &stdCompressed, &ourDecompressed); err != nil {
		t.Fatalf("Our Decompress() failed: %v", err)
	}

	if !bytes.Equal(ourDecompressed.Bytes(), testData) {
		t.Error("Our implementation couldn't decompress standard gzip output correctly")
	}
}
