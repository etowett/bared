package util

import (
	"io"
	"sync/atomic"
	"time"
)

// ProgressReader wraps an io.Reader to track bytes read with minimal overhead
// Uses atomic operations for thread-safe byte counting and throttled callbacks
type ProgressReader struct {
	r              io.Reader
	bytesRead      *atomic.Int64
	callback       func(bytes int64)
	lastUpdateTime atomic.Int64 // Unix nano
	updateInterval int64         // nanoseconds
}

// NewProgressReader creates a new ProgressReader that tracks bytes read
// callback is called periodically (throttled to ~1 second intervals) with current byte count
// Pass nil callback if you only need the byte counter without callbacks
func NewProgressReader(r io.Reader, callback func(bytes int64)) *ProgressReader {
	pr := &ProgressReader{
		r:              r,
		bytesRead:      &atomic.Int64{},
		callback:       callback,
		updateInterval: int64(time.Second), // 1 second throttle
	}
	pr.lastUpdateTime.Store(time.Now().UnixNano())
	return pr
}

// Read implements io.Reader while tracking bytes read
func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.r.Read(p)

	if n > 0 {
		newTotal := pr.bytesRead.Add(int64(n))

		// Throttled callback - only call if enough time has passed
		if pr.callback != nil {
			now := time.Now().UnixNano()
			lastUpdate := pr.lastUpdateTime.Load()
			if now-lastUpdate >= pr.updateInterval {
				if pr.lastUpdateTime.CompareAndSwap(lastUpdate, now) {
					pr.callback(newTotal)
				}
			}
		}
	}

	return n, err
}

// BytesRead returns the total number of bytes read so far
func (pr *ProgressReader) BytesRead() int64 {
	return pr.bytesRead.Load()
}

// ProgressWriter wraps an io.Writer to track bytes written with minimal overhead
// Uses atomic operations for thread-safe byte counting and throttled callbacks
type ProgressWriter struct {
	w              io.Writer
	bytesWritten   *atomic.Int64
	callback       func(bytes int64)
	lastUpdateTime atomic.Int64 // Unix nano
	updateInterval int64         // nanoseconds
}

// NewProgressWriter creates a new ProgressWriter that tracks bytes written
// callback is called periodically (throttled to ~1 second intervals) with current byte count
// Pass nil callback if you only need the byte counter without callbacks
func NewProgressWriter(w io.Writer, callback func(bytes int64)) *ProgressWriter {
	pw := &ProgressWriter{
		w:              w,
		bytesWritten:   &atomic.Int64{},
		callback:       callback,
		updateInterval: int64(time.Second), // 1 second throttle
	}
	pw.lastUpdateTime.Store(time.Now().UnixNano())
	return pw
}

// Write implements io.Writer while tracking bytes written
func (pw *ProgressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.w.Write(p)

	if n > 0 {
		newTotal := pw.bytesWritten.Add(int64(n))

		// Throttled callback - only call if enough time has passed
		if pw.callback != nil {
			now := time.Now().UnixNano()
			lastUpdate := pw.lastUpdateTime.Load()
			if now-lastUpdate >= pw.updateInterval {
				if pw.lastUpdateTime.CompareAndSwap(lastUpdate, now) {
					pw.callback(newTotal)
				}
			}
		}
	}

	return n, err
}

// BytesWritten returns the total number of bytes written so far
func (pw *ProgressWriter) BytesWritten() int64 {
	return pw.bytesWritten.Load()
}

// Close implements io.Closer if the underlying writer implements it
func (pw *ProgressWriter) Close() error {
	if closer, ok := pw.w.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
