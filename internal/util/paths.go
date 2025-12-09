package util

import (
	"fmt"
	"path/filepath"
	"time"
)

// GenerateBackupPath generates a unique backup path
// Format: {target}/{dbType}/{timestamp}/{database}{extension}
// Example: mysql_prod/mysql/2025-12-02T15-04-05Z/myapp_db.sql.tar.gz
func GenerateBackupPath(targetName, dbType, dbName, extension string) string {
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	filename := fmt.Sprintf("%s%s", dbName, extension)
	return filepath.Join(targetName, dbType, timestamp, filename)
}
