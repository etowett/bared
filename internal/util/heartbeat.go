package util

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// HeartbeatLogger provides periodic status logging during long-running operations
// Reports bytes processed, speed, ETA, and memory usage at regular intervals
type HeartbeatLogger struct {
	target       string
	stage        string
	bytesTotal   int64
	getBytesFunc func() int64
	interval     time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	startTime    time.Time
	lastBytes    int64
	lastLogTime  time.Time
}

// NewHeartbeatLogger creates a new heartbeat logger
// target: the target name (e.g., database name)
// stage: the current stage (e.g., "DUMP_AND_COMPRESS", "UPLOAD")
// bytesTotal: estimated total bytes (0 if unknown)
// getBytesFunc: function that returns current bytes processed
func NewHeartbeatLogger(target string, stage string, bytesTotal int64, getBytesFunc func() int64) *HeartbeatLogger {
	ctx, cancel := context.WithCancel(context.Background())
	return &HeartbeatLogger{
		target:       target,
		stage:        stage,
		bytesTotal:   bytesTotal,
		getBytesFunc: getBytesFunc,
		interval:     30 * time.Second, // Default 30 second interval
		ctx:          ctx,
		cancel:       cancel,
		startTime:    time.Now(),
		lastLogTime:  time.Now(),
	}
}

// SetInterval sets the logging interval (default is 30 seconds)
func (h *HeartbeatLogger) SetInterval(interval time.Duration) {
	h.interval = interval
}

// Start begins periodic heartbeat logging in the background
func (h *HeartbeatLogger) Start() {
	h.wg.Add(1)
	go h.run()
}

// Stop stops the heartbeat logger and waits for it to finish
func (h *HeartbeatLogger) Stop() {
	h.cancel()
	h.wg.Wait()
}

// run is the main heartbeat loop
func (h *HeartbeatLogger) run() {
	defer h.wg.Done()

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			// Log final heartbeat before stopping
			h.logHeartbeat()
			return
		case <-ticker.C:
			h.logHeartbeat()
		}
	}
}

// logHeartbeat logs the current status
func (h *HeartbeatLogger) logHeartbeat() {
	logger := GetLogger()

	currentBytes := h.getBytesFunc()
	elapsed := time.Since(h.startTime)
	elapsedSinceLastLog := time.Since(h.lastLogTime)

	// Calculate speed (bytes per second)
	var speed float64
	if elapsedSinceLastLog.Seconds() > 0 {
		bytesSinceLastLog := currentBytes - h.lastBytes
		speed = float64(bytesSinceLastLog) / elapsedSinceLastLog.Seconds()
	}

	// Update for next calculation
	h.lastBytes = currentBytes
	h.lastLogTime = time.Now()

	// Get memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Build log attributes
	attrs := []interface{}{
		"stage", h.stage,
		"bytes_processed", formatBytes(currentBytes),
		"elapsed", formatDuration(elapsed),
		"memory_alloc", formatBytesUint64(m.Alloc),
		"goroutines", runtime.NumGoroutine(),
	}

	// Add speed if we have data
	if speed > 0 {
		attrs = append(attrs, "speed", fmt.Sprintf("%.1f MB/s", speed/(1024*1024)))
	}

	// Add progress percentage and ETA if we know total size
	if h.bytesTotal > 0 {
		percent := float64(currentBytes) / float64(h.bytesTotal) * 100

		attrs = append(attrs,
			"bytes_total", formatBytes(h.bytesTotal),
			"percent", fmt.Sprintf("%.1f%%", percent),
		)

		// Calculate ETA if we have speed and haven't finished
		if speed > 0 && currentBytes < h.bytesTotal {
			remainingBytes := h.bytesTotal - currentBytes
			etaSeconds := float64(remainingBytes) / speed
			attrs = append(attrs, "eta", formatDuration(time.Duration(etaSeconds)*time.Second))
		}
	}

	logger.InfoS(fmt.Sprintf("[%s] Progress update", h.target), attrs...)
}

// formatBytes formats bytes in human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	for n := bytes / unit; n >= unit && exp < len(units)-1; n /= unit {
		div *= unit
		exp++
	}
	// Ensure exp is within bounds (for static analysis)
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// formatBytesUint64 formats uint64 bytes in human-readable format
func formatBytesUint64(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	for n := bytes / unit; n >= unit && exp < len(units)-1; n /= unit {
		div *= unit
		exp++
	}
	// Ensure exp is within bounds (for static analysis)
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// formatDuration formats duration in human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm%.0fs", d.Minutes(), d.Seconds()-d.Minutes()*60)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) - hours*60
	return fmt.Sprintf("%dh%dm", hours, minutes)
}
