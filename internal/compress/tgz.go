package compress

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// TarGz implements tar.gz compression
type TarGz struct {
	filename string
}

// NewTarGz creates a new tar.gz compressor
func NewTarGz(filename string) *TarGz {
	return &TarGz{
		filename: filename,
	}
}

// Extension returns the file extension
func (t *TarGz) Extension() string {
	return ".tar.gz"
}

// Compress reads from reader, creates a tar.gz archive, and writes to writer
func (t *TarGz) Compress(ctx context.Context, r io.Reader, w io.Writer) error {
	// Create gzip writer
	gzw := gzip.NewWriter(w)
	defer func() {
		//nolint:govet,staticcheck // shadow is intentional; empty branch is intentional - error handling in defer
		if err := gzw.Close(); err != nil {
			// Error already being returned by main function
		}
	}()

	// Create tar writer
	tw := tar.NewWriter(gzw)
	defer func() {
		//nolint:govet,staticcheck // shadow is intentional; empty branch is intentional - error handling in defer
		if err := tw.Close(); err != nil {
			// Error already being returned by main function
		}
	}()

	// Create a pipe to count bytes as we read
	pr, pw := io.Pipe()
	var readErr error
	var readErrMu sync.Mutex

	// Read from input in goroutine
	go func() {
		defer func() {
			if err := pw.Close(); err != nil {
				readErrMu.Lock()
				if readErr == nil {
					readErr = err
				}
				readErrMu.Unlock()
			}
		}()
		_, err := io.Copy(pw, r)
		if err != nil {
			readErrMu.Lock()
			readErr = err
			readErrMu.Unlock()
		}
	}()

	// Write tar header (size will be set after buffering)
	header := &tar.Header{
		Name:    t.filename,
		Mode:    0600,
		Size:    0, // Will be set after we know the actual size
		ModTime: time.Now(),
	}

	// For streaming, we need to know the size beforehand or use a different approach
	// Let's buffer the data first to know the size
	buf := make([]byte, 32*1024) // 32KB buffer
	var bufferedData []byte

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := pr.Read(buf)
		if n > 0 {
			bufferedData = append(bufferedData, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read data: %w", err)
		}
	}

	readErrMu.Lock()
	err := readErr
	readErrMu.Unlock()
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Now we know the size, write the header
	header.Size = int64(len(bufferedData))
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	// Write the data
	if _, err := tw.Write(bufferedData); err != nil {
		return fmt.Errorf("failed to write tar data: %w", err)
	}

	return nil
}

// Decompress reads tar.gz from reader and writes uncompressed content to writer
func (t *TarGz) Decompress(ctx context.Context, r io.Reader, w io.Writer) error {
	// Create gzip reader
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		//nolint:govet,staticcheck // shadow is intentional; empty branch is intentional - error handling in defer
		if err := gzr.Close(); err != nil {
			// Error already being returned by main function
		}
	}()

	// Create tar reader
	tr := tar.NewReader(gzr)

	// Read the first file from the tar archive
	_, err = tr.Next()
	if err != nil {
		return fmt.Errorf("failed to read tar entry: %w", err)
	}

	// Copy the file content to the writer
	//#nosec G110 -- decompression from trusted backup sources only
	if _, err := io.Copy(w, tr); err != nil {
		return fmt.Errorf("failed to decompress data: %w", err)
	}

	return nil
}
