package util

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// CreateBackupTempFile creates a temporary file for backup operations
// Returns the file handle and a cleanup function that should be called via defer
func CreateBackupTempFile(targetName string) (*os.File, func(), error) {
	// Generate unique filename
	timestamp := time.Now().Format("20060102-150405")
	pid := os.Getpid()
	random := rand.Intn(10000) // #nosec G404 - Used for filename uniqueness, not security
	filename := fmt.Sprintf("bared-backup-%s-%s-%d-%d.tmp", targetName, timestamp, pid, random)

	// Create temp file in system temp directory
	tempDir := os.TempDir()
	fullPath := filepath.Join(tempDir, filename)

	Debug("Creating temp file: %s", fullPath)

	// Create the file
	file, err := os.Create(fullPath) // #nosec G304 - fullPath is constructed from trusted sources (os.TempDir and validated targetName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Create cleanup function
	cleanup := func() {
		// Close the file if still open
		if err := file.Close(); err != nil {
			Warn("Failed to close temp file %s: %v", fullPath, err)
		}

		// Remove the file
		if err := os.Remove(fullPath); err != nil {
			Warn("Failed to remove temp file %s: %v", fullPath, err)
		} else {
			Debug("Removed temp file: %s", fullPath)
		}
	}

	return file, cleanup, nil
}

// GetFileSize returns the current size of a file
func GetFileSize(file *os.File) (int64, error) {
	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}
	return info.Size(), nil
}

// FormatBytes converts bytes to a human-readable format (KB, MB, GB, etc.)
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
