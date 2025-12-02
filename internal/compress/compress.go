package compress

import (
	"context"
	"io"
)

// Compressor handles compression and decompression
type Compressor interface {
	// Compress reads from reader, compresses, and writes to writer
	Compress(ctx context.Context, r io.Reader, w io.Writer) error

	// Decompress reads compressed data from reader and writes uncompressed to writer
	Decompress(ctx context.Context, r io.Reader, w io.Writer) error

	// Extension returns the file extension (e.g., ".tar.gz")
	Extension() string
}
