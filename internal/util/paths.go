package util

import (
	"fmt"
	"path/filepath"
	"time"
)

// GenerateBackupPath generates a unique backup path
// Format: {target}/{database}-{dbtype}-{timestamp}{extension}
// Example: athena_local_db/mydb-postgres-2025-12-02T15-04-05Z.sql.tar.gz
func GenerateBackupPath(targetName, dbType, dbName, extension string) string {
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	filename := fmt.Sprintf("%s-%s-%s%s", dbName, dbType, timestamp, extension)
	return filepath.Join(targetName, filename)
}
