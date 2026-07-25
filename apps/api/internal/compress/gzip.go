package compress

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
)

// Gzip implements gzip compression with streaming (no buffering)
// This compressor maintains constant memory usage regardless of input size,
// making it suitable for very large database backups (100GB+)
type Gzip struct {
	filename string
}

// NewGzip creates a new gzip compressor
func NewGzip(filename string) *Gzip {
	return &Gzip{
		filename: filename,
	}
}

// Extension returns the file extension
func (g *Gzip) Extension() string {
	return ".gz"
}

// Compress reads from reader, compresses with gzip, and writes to writer
// This implementation streams data through a fixed-size buffer without accumulation,
// maintaining constant memory usage regardless of input size
func (g *Gzip) Compress(ctx context.Context, r io.Reader, w io.Writer) error {
	// Create gzip writer
	gzw := gzip.NewWriter(w)
	defer func() {
		//nolint:errcheck // Error closing gzip writer during cleanup is not critical
		_ = gzw.Close()
	}()

	// Stream data through fixed buffer - NO accumulation in memory
	buf := make([]byte, 32*1024) // 32KB buffer, reused for entire stream
	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read chunk from input
		n, readErr := r.Read(buf)

		// Write any data read (even if error occurred)
		if n > 0 {
			if _, writeErr := gzw.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("failed to write compressed data: %w", writeErr)
			}
		}

		// Handle read errors
		if readErr == io.EOF {
			break // End of input, success
		}
		if readErr != nil {
			return fmt.Errorf("failed to read data: %w", readErr)
		}
	}

	// Flush and close gzip writer to ensure all compressed data is written
	if err := gzw.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return nil
}

// Decompress reads gzip compressed data from reader and writes uncompressed content to writer
func (g *Gzip) Decompress(ctx context.Context, r io.Reader, w io.Writer) error {
	// Create gzip reader
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		//nolint:errcheck // Error closing gzip reader during cleanup is not critical
		_ = gzr.Close()
	}()

	// Stream decompressed data through fixed buffer
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read decompressed chunk
		n, readErr := gzr.Read(buf)

		// Write any data read
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("failed to write decompressed data: %w", writeErr)
			}
		}

		// Handle read errors
		if readErr == io.EOF {
			break // End of input, success
		}
		if readErr != nil {
			return fmt.Errorf("failed to decompress data: %w", readErr)
		}
	}

	return nil
}
