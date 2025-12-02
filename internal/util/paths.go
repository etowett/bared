package util

import (
	"fmt"
	"path/filepath"
	"time"
)

// GenerateBackupPath generates a unique backup path
// Format: {target}/{dbtype}/{timestamp}/{database}{extension}
// Example: athena/mysql/2025-12-02T15-04-05Z/mydb.sql.tar.gz
func GenerateBackupPath(targetName, dbType, dbName, extension string) string {
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	filename := fmt.Sprintf("%s%s", dbName, extension)
	return filepath.Join(targetName, dbType, timestamp, filename)
}
