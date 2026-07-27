// Package storage provides storage backend implementations for local, S3, and SFTP.
package storage

import (
	"fmt"

	"github.com/etowett/bared/apps/api/internal/config"
)

// New creates a new storage backend based on the configuration
func New(cfg *config.Storage) (Storage, error) {
	switch cfg.Type {
	case "local":
		return NewLocal(cfg), nil
	case "s3":
		return NewS3(cfg), nil
	case "sftp":
		return NewSFTP(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}
