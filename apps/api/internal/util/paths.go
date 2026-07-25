package util

import (
	"fmt"
	"path/filepath"
	"time"
)

// GenerateBackupPath generates a unique backup path
// Format: {target}/backup-{timestamp}{extension}
// Example: mysql_prod/backup-2026-01-05T17-29-06Z.sql.tar.gz
func GenerateBackupPath(targetName, dbType, dbName, extension string) string {
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	filename := fmt.Sprintf("backup-%s%s", timestamp, extension)
	return filepath.Join(targetName, filename)
}
