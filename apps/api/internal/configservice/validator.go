package configservice

import (
	"fmt"

	"bared/internal/config"
)

// ValidateStorage validates a storage configuration
func ValidateStorage(storage *config.Storage) error {
	if storage.Name == "" {
		return fmt.Errorf("storage name is required")
	}

	if storage.Type == "" {
		return fmt.Errorf("storage type is required")
	}

	switch storage.Type {
	case "local":
		if storage.Path == "" {
			return fmt.Errorf("local storage requires path")
		}
	case "s3":
		if storage.Bucket == "" {
			return fmt.Errorf("s3 storage requires bucket")
		}
		if storage.Region == "" {
			return fmt.Errorf("s3 storage requires region")
		}
	case "sftp":
		if storage.Host == "" {
			return fmt.Errorf("sftp storage requires host")
		}
		if storage.Port == 0 {
			return fmt.Errorf("sftp storage requires port")
		}
		if storage.Username == "" {
			return fmt.Errorf("sftp storage requires username")
		}
		if storage.Password == "" && storage.PrivateKeyPath == "" {
			return fmt.Errorf("sftp storage requires password or private_key_path")
		}
		if storage.InsecureSkipHostKeyVerify && storage.HostKeyFingerprint != "" {
			return fmt.Errorf(
				"sftp storage: insecure_skip_host_key_verify and host_key_fingerprint are mutually exclusive")
		}
	default:
		return fmt.Errorf("unsupported storage type: %s (must be local, s3, or sftp)", storage.Type)
	}

	if storage.Keep < 0 {
		return fmt.Errorf("keep must be non-negative")
	}

	return nil
}

// ValidateNotifier validates a notifier configuration
func ValidateNotifier(notifier *config.Notifier) error {
	if notifier.Type == "" {
		return fmt.Errorf("notifier type is required")
	}

	switch notifier.Type {
	case "slack":
		if notifier.URL == "" {
			return fmt.Errorf("slack notifier requires url")
		}
	case "email":
		if notifier.SMTPHost == "" {
			return fmt.Errorf("email notifier requires smtp_host")
		}
		if notifier.SMTPPort == 0 {
			return fmt.Errorf("email notifier requires smtp_port")
		}
		if notifier.SMTPFrom == "" {
			return fmt.Errorf("email notifier requires smtp_from")
		}
		if len(notifier.SMTPTo) == 0 {
			return fmt.Errorf("email notifier requires at least one smtp_to address")
		}
	case "webhook":
		if notifier.URL == "" {
			return fmt.Errorf("webhook notifier requires url")
		}
		if notifier.WebhookAuth != nil {
			if err := validateWebhookAuth(notifier.WebhookAuth); err != nil {
				return fmt.Errorf("webhook auth validation failed: %w", err)
			}
		}
	default:
		return fmt.Errorf("unsupported notifier type: %s (must be slack, email, or webhook)", notifier.Type)
	}

	return nil
}

func validateWebhookAuth(auth *config.WebhookAuth) error {
	if auth.Type == "" {
		return fmt.Errorf("webhook auth type is required")
	}

	switch auth.Type {
	case "basic":
		if auth.Username == "" {
			return fmt.Errorf("basic auth requires username")
		}
		if auth.Password == "" {
			return fmt.Errorf("basic auth requires password")
		}
	case "bearer":
		if auth.Token == "" {
			return fmt.Errorf("bearer auth requires token")
		}
	case "header":
		if auth.HeaderName == "" {
			return fmt.Errorf("header auth requires header_name")
		}
		if auth.HeaderValue == "" {
			return fmt.Errorf("header auth requires header_value")
		}
	default:
		return fmt.Errorf("unsupported webhook auth type: %s (must be basic, bearer, or header)", auth.Type)
	}

	return nil
}

// ValidateTarget validates a target configuration
func ValidateTarget(target *config.Target, storages map[string]*config.Storage) error {
	if target.Name == "" {
		return fmt.Errorf("target name is required")
	}

	if target.Conn == nil {
		return fmt.Errorf("target connection is required")
	}

	if err := validateConnection(target.Conn); err != nil {
		return fmt.Errorf("connection validation failed: %w", err)
	}

	// Validate storage reference
	if target.Storage != nil && target.Storage.Enabled {
		if target.Storage.Name == "" {
			return fmt.Errorf("storage name is required when storage is enabled")
		}
		if storages != nil {
			if _, exists := storages[target.Storage.Name]; !exists {
				return fmt.Errorf("storage %s does not exist", target.Storage.Name)
			}
		}
	}

	// Validate compression
	if target.Compress != nil && target.Compress.Enabled {
		if target.Compress.Type != "" && target.Compress.Type != "gzip" && target.Compress.Type != "tgz" {
			return fmt.Errorf("unsupported compression type: %s (must be gzip or tgz)", target.Compress.Type)
		}
	}

	return nil
}

// ValidateRestoreTarget validates a restore target configuration
func ValidateRestoreTarget(rt *config.RestoreTarget, storages map[string]*config.Storage, targets map[string]*config.Target) error {
	if rt.Name == "" {
		return fmt.Errorf("restore target name is required")
	}

	if rt.Conn == nil {
		return fmt.Errorf("restore target connection is required")
	}

	if err := validateConnection(rt.Conn); err != nil {
		return fmt.Errorf("connection validation failed: %w", err)
	}

	// Validate storage reference
	if rt.Storage != nil && rt.Storage.Enabled {
		if rt.Storage.Name == "" {
			return fmt.Errorf("storage name is required when storage is enabled")
		}
		if storages != nil {
			if _, exists := storages[rt.Storage.Name]; !exists {
				return fmt.Errorf("storage %s does not exist", rt.Storage.Name)
			}
		}
	}

	// Validate source target reference
	if rt.SourceTarget != "" {
		if targets != nil {
			found := false
			for _, target := range targets {
				if target.Name == rt.SourceTarget {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("source target %s does not exist", rt.SourceTarget)
			}
		}
	}

	return nil
}

func validateConnection(conn *config.Connection) error {
	if conn.Type == "" {
		return fmt.Errorf("connection type is required")
	}

	switch conn.Type {
	case "mysql", "postgres":
		if conn.Host == "" {
			return fmt.Errorf("%s connection requires host", conn.Type)
		}
		if conn.Port == 0 {
			return fmt.Errorf("%s connection requires port", conn.Type)
		}
		if conn.Database == "" {
			return fmt.Errorf("%s connection requires database", conn.Type)
		}
	case "redis":
		if conn.Host == "" {
			return fmt.Errorf("redis connection requires host")
		}
		if conn.Port == 0 {
			return fmt.Errorf("redis connection requires port")
		}
	default:
		return fmt.Errorf("unsupported connection type: %s (must be mysql, postgres, or redis)", conn.Type)
	}

	return nil
}
