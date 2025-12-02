package fixtures

import (
	"bared/internal/config"
)

// LocalStorage returns a valid local storage config for testing
func LocalStorage() *config.Storage {
	return &config.Storage{
		Name: "local",
		Type: "local",
		Path: "/tmp/test-backups",
		Keep: 5,
	}
}

// S3Storage returns a valid S3 storage config for testing
func S3Storage() *config.Storage {
	return &config.Storage{
		Name:            "s3",
		Type:            "s3",
		Bucket:          "test-backup-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
		Keep:            10,
	}
}

// S3StorageWithEndpoint returns S3 storage with custom endpoint (MinIO)
func S3StorageWithEndpoint() *config.Storage {
	storage := S3Storage()
	storage.EndpointURL = "http://localhost:9000"
	return storage
}

// SFTPStorage returns a valid SFTP storage config for testing
func SFTPStorage() *config.Storage {
	return &config.Storage{
		Name:     "sftp",
		Type:     "sftp",
		Host:     "sftp.example.com",
		Port:     22,
		Username: "backup-user",
		Password: "backup-pass",
		Path:     "/backups",
		Keep:     7,
	}
}

// LocalStorageWithCustomPath returns local storage with custom path
func LocalStorageWithCustomPath(path string) *config.Storage {
	storage := LocalStorage()
	storage.Path = path
	return storage
}

// S3StorageWithCustomRegion returns S3 storage with custom region
func S3StorageWithCustomRegion(region string) *config.Storage {
	storage := S3Storage()
	storage.Region = region
	return storage
}

// ConfigWithLocalStorage returns a complete config with local storage
func ConfigWithLocalStorage() *config.Config {
	return &config.Config{
		DefaultStorage: "local",
		Storages: map[string]*config.Storage{
			"local": LocalStorage(),
		},
		Targets: []*config.Target{
			MySQLTarget(),
		},
	}
}

// ConfigWithS3Storage returns a complete config with S3 storage
func ConfigWithS3Storage() *config.Config {
	return &config.Config{
		DefaultStorage: "s3",
		Storages: map[string]*config.Storage{
			"s3": S3Storage(),
		},
		Targets: []*config.Target{
			MySQLTarget(),
		},
	}
}

// ConfigWithMultipleStorages returns a config with multiple storage backends
func ConfigWithMultipleStorages() *config.Config {
	return &config.Config{
		DefaultStorage: "local",
		Storages: map[string]*config.Storage{
			"local": LocalStorage(),
			"s3":    S3Storage(),
			"sftp":  SFTPStorage(),
		},
		Targets: []*config.Target{
			{
				Name: "mysql_local",
				Conn: MySQLConnection(),
				Storage: &config.TargetStorage{
					Enabled: true,
					Name:    "local",
				},
			},
			{
				Name: "postgres_s3",
				Conn: PostgresConnection(),
				Storage: &config.TargetStorage{
					Enabled: true,
					Name:    "s3",
				},
			},
			{
				Name: "redis_sftp",
				Conn: RedisConnection(),
				Storage: &config.TargetStorage{
					Enabled: true,
					Name:    "sftp",
				},
			},
		},
	}
}

// ConfigWithNotifications returns a config with Slack notifications
func ConfigWithNotifications() *config.Config {
	cfg := ConfigWithLocalStorage()
	cfg.Notifiers = map[string]*config.Notifier{
		"slack": {
			Type:      "slack",
			URL:       "https://hooks.slack.com/services/TEST/WEBHOOK/URL",
			OnSuccess: true,
		},
	}
	return cfg
}

// BackupPath returns a typical backup path for testing
func BackupPath(target, dbType, database, extension string) string {
	return target + "/" + dbType + "/2025-12-02T10-30-00Z/" + database + extension
}

// MySQLBackupPath returns a typical MySQL backup path
func MySQLBackupPath() string {
	return BackupPath("mysql_test", "mysql", "testdb", ".sql.tar.gz")
}

// PostgresBackupPath returns a typical PostgreSQL backup path
func PostgresBackupPath() string {
	return BackupPath("postgres_test", "postgres", "testdb", ".sql.tar.gz")
}

// RedisBackupPath returns a typical Redis backup path
func RedisBackupPath() string {
	return BackupPath("redis_test", "redis", "redis", ".rdb.tar.gz")
}
