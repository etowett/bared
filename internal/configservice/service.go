// Package configservice provides database-backed configuration management
package configservice

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"bared/internal/config"
	"bared/internal/encryption"
)

// Service handles configuration storage and retrieval
type Service struct {
	db         *sql.DB
	encryption *encryption.Service
}

// NewService creates a new config service
func NewService(db *sql.DB, encryptionSvc *encryption.Service) *Service {
	return &Service{
		db:         db,
		encryption: encryptionSvc,
	}
}

// ConfigSource indicates where configs are loaded from
type ConfigSource string

const (
	SourceDatabase ConfigSource = "database"
	SourceYAML     ConfigSource = "yaml"
)

// HasDatabaseConfigs checks if any configs exist in the database
func (s *Service) HasDatabaseConfigs(ctx context.Context) (bool, error) {
	var count int
	query := `
		SELECT
			(SELECT COUNT(*) FROM storages) +
			(SELECT COUNT(*) FROM notifiers) +
			(SELECT COUNT(*) FROM targets) +
			(SELECT COUNT(*) FROM restore_targets)
		AS total
	`
	if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return false, fmt.Errorf("failed to check for configs: %w", err)
	}
	return count > 0, nil
}

// Storage operations

// CreateStorage creates a new storage configuration
func (s *Service) CreateStorage(ctx context.Context, storage *config.Storage) error {
	// Serialize config to JSON
	configJSON, secrets, err := s.serializeStorage(storage)
	if err != nil {
		return fmt.Errorf("failed to serialize storage: %w", err)
	}

	// Insert storage
	query := `INSERT INTO storages (name, type, config_json, keep, enabled, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?)`
	now := time.Now()
	result, err := s.db.ExecContext(ctx, query, storage.Name, storage.Type, configJSON, storage.Keep, true, now, now)
	if err != nil {
		return fmt.Errorf("failed to insert storage: %w", err)
	}

	storageID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get storage ID: %w", err)
	}

	// Store encrypted secrets
	if err := s.storeSecrets(ctx, "storage", storageID, secrets); err != nil {
		return fmt.Errorf("failed to store secrets: %w", err)
	}

	return nil
}

// GetStorage retrieves a storage by name
func (s *Service) GetStorage(ctx context.Context, name string) (*config.Storage, error) {
	var id int64
	var storageType, configJSON string
	var keep int
	var enabled bool
	var createdAt, updatedAt time.Time

	query := `SELECT id, name, type, config_json, keep, enabled, created_at, updated_at
			  FROM storages WHERE name = ?`
	err := s.db.QueryRowContext(ctx, query, name).Scan(&id, &name, &storageType, &configJSON, &keep, &enabled, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("storage not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query storage: %w", err)
	}

	// Get secrets
	secrets, err := s.getSecrets(ctx, "storage", id)
	if err != nil {
		return nil, fmt.Errorf("failed to get secrets: %w", err)
	}

	// Deserialize
	storage, err := s.deserializeStorage(name, storageType, configJSON, keep, secrets)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize storage: %w", err)
	}

	return storage, nil
}

// ListStorages retrieves all storages
func (s *Service) ListStorages(ctx context.Context) (map[string]*config.Storage, error) {
	query := `SELECT id, name, type, config_json, keep, enabled, created_at, updated_at FROM storages WHERE enabled = true`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query storages: %w", err)
	}
	defer rows.Close() //nolint:errcheck // best effort cleanup

	storages := make(map[string]*config.Storage)
	for rows.Next() {
		var id int64
		var name, storageType, configJSON string
		var keep int
		var enabled bool
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&id, &name, &storageType, &configJSON, &keep, &enabled, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan storage: %w", err)
		}

		secrets, err := s.getSecrets(ctx, "storage", id)
		if err != nil {
			return nil, fmt.Errorf("failed to get secrets: %w", err)
		}

		storage, err := s.deserializeStorage(name, storageType, configJSON, keep, secrets)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize storage: %w", err)
		}

		storages[name] = storage
	}

	return storages, nil
}

// UpdateStorage updates an existing storage
func (s *Service) UpdateStorage(ctx context.Context, name string, storage *config.Storage) error {
	// Get existing storage ID
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM storages WHERE name = ?`, name).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("storage not found: %s", name)
		}
		return fmt.Errorf("failed to query storage: %w", err)
	}

	// Serialize config
	configJSON, secrets, err := s.serializeStorage(storage)
	if err != nil {
		return fmt.Errorf("failed to serialize storage: %w", err)
	}

	// Update storage
	query := `UPDATE storages SET type = ?, config_json = ?, keep = ?, updated_at = ? WHERE name = ?`
	if _, err := s.db.ExecContext(ctx, query, storage.Type, configJSON, storage.Keep, time.Now(), name); err != nil {
		return fmt.Errorf("failed to update storage: %w", err)
	}

	// Update secrets
	if err := s.updateSecrets(ctx, "storage", id, secrets); err != nil {
		return fmt.Errorf("failed to update secrets: %w", err)
	}

	return nil
}

// DeleteStorage deletes a storage
func (s *Service) DeleteStorage(ctx context.Context, name string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM storages WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("failed to delete storage: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("storage not found: %s", name)
	}

	return nil
}

// Notifier operations

// CreateNotifier creates a new notifier configuration
func (s *Service) CreateNotifier(ctx context.Context, name string, notifier *config.Notifier) error {
	configJSON, secrets, err := s.serializeNotifier(notifier)
	if err != nil {
		return fmt.Errorf("failed to serialize notifier: %w", err)
	}

	query := `INSERT INTO notifiers (name, type, config_json, on_success, enabled, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?)`
	now := time.Now()
	result, err := s.db.ExecContext(ctx, query, name, notifier.Type, configJSON, notifier.OnSuccess, true, now, now)
	if err != nil {
		return fmt.Errorf("failed to insert notifier: %w", err)
	}

	notifierID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get notifier ID: %w", err)
	}

	if err := s.storeSecrets(ctx, "notifier", notifierID, secrets); err != nil {
		return fmt.Errorf("failed to store secrets: %w", err)
	}

	return nil
}

// GetNotifier retrieves a notifier by name
func (s *Service) GetNotifier(ctx context.Context, name string) (*config.Notifier, error) {
	var id int64
	var notifierType, configJSON string
	var onSuccess, enabled bool
	var createdAt, updatedAt time.Time

	query := `SELECT id, name, type, config_json, on_success, enabled, created_at, updated_at
			  FROM notifiers WHERE name = ?`
	err := s.db.QueryRowContext(ctx, query, name).Scan(&id, &name, &notifierType, &configJSON, &onSuccess, &enabled, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("notifier not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query notifier: %w", err)
	}

	secrets, err := s.getSecrets(ctx, "notifier", id)
	if err != nil {
		return nil, fmt.Errorf("failed to get secrets: %w", err)
	}

	notifier, err := s.deserializeNotifier(notifierType, configJSON, onSuccess, secrets)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize notifier: %w", err)
	}

	return notifier, nil
}

// ListNotifiers retrieves all notifiers
func (s *Service) ListNotifiers(ctx context.Context) (map[string]*config.Notifier, error) {
	query := `SELECT id, name, type, config_json, on_success, enabled FROM notifiers WHERE enabled = true`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query notifiers: %w", err)
	}
	defer rows.Close() //nolint:errcheck // best effort cleanup

	notifiers := make(map[string]*config.Notifier)
	for rows.Next() {
		var id int64
		var name, notifierType, configJSON string
		var onSuccess, enabled bool

		if err := rows.Scan(&id, &name, &notifierType, &configJSON, &onSuccess, &enabled); err != nil {
			return nil, fmt.Errorf("failed to scan notifier: %w", err)
		}

		secrets, err := s.getSecrets(ctx, "notifier", id)
		if err != nil {
			return nil, fmt.Errorf("failed to get secrets: %w", err)
		}

		notifier, err := s.deserializeNotifier(notifierType, configJSON, onSuccess, secrets)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize notifier: %w", err)
		}

		notifiers[name] = notifier
	}

	return notifiers, nil
}

// UpdateNotifier updates an existing notifier
func (s *Service) UpdateNotifier(ctx context.Context, name string, notifier *config.Notifier) error {
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM notifiers WHERE name = ?`, name).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("notifier not found: %s", name)
		}
		return fmt.Errorf("failed to query notifier: %w", err)
	}

	configJSON, secrets, err := s.serializeNotifier(notifier)
	if err != nil {
		return fmt.Errorf("failed to serialize notifier: %w", err)
	}

	query := `UPDATE notifiers SET type = ?, config_json = ?, on_success = ?, updated_at = ? WHERE name = ?`
	if _, err := s.db.ExecContext(ctx, query, notifier.Type, configJSON, notifier.OnSuccess, time.Now(), name); err != nil {
		return fmt.Errorf("failed to update notifier: %w", err)
	}

	if err := s.updateSecrets(ctx, "notifier", id, secrets); err != nil {
		return fmt.Errorf("failed to update secrets: %w", err)
	}

	return nil
}

// DeleteNotifier deletes a notifier
func (s *Service) DeleteNotifier(ctx context.Context, name string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM notifiers WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("failed to delete notifier: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("notifier not found: %s", name)
	}

	return nil
}

// Target operations

// CreateTarget creates a new target configuration
func (s *Service) CreateTarget(ctx context.Context, target *config.Target) error {
	connJSON, connSecrets, err := s.serializeConnection(target.Conn)
	if err != nil {
		return fmt.Errorf("failed to serialize connection: %w", err)
	}

	var storageName sql.NullString
	if target.Storage != nil && target.Storage.Enabled {
		storageName = sql.NullString{String: target.Storage.Name, Valid: true}
	}

	var schedule sql.NullString
	if target.Schedule != "" {
		schedule = sql.NullString{String: target.Schedule, Valid: true}
	}

	var compressEnabled bool
	var compressType sql.NullString
	if target.Compress != nil {
		compressEnabled = target.Compress.Enabled
		if target.Compress.Type != "" {
			compressType = sql.NullString{String: target.Compress.Type, Valid: true}
		}
	}

	excludeTablesJSON, _ := json.Marshal(target.ExcludeTables)   //nolint:errcheck // marshaling string slice never fails
	additionalArgsJSON, _ := json.Marshal(target.AdditionalArgs) //nolint:errcheck // marshaling string slice never fails

	query := `INSERT INTO targets (name, type, conn_type, conn_json, storage_name, schedule,
			  compress_enabled, compress_type, exclude_tables, additional_args, enabled, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now()
	result, err := s.db.ExecContext(ctx, query, target.Name, "backup", target.Conn.Type, connJSON, storageName, schedule,
		compressEnabled, compressType, string(excludeTablesJSON), string(additionalArgsJSON), true, now, now)
	if err != nil {
		return fmt.Errorf("failed to insert target: %w", err)
	}

	targetID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get target ID: %w", err)
	}

	if err := s.storeSecrets(ctx, "target", targetID, connSecrets); err != nil {
		return fmt.Errorf("failed to store secrets: %w", err)
	}

	return nil
}

// GetTarget retrieves a target by name
func (s *Service) GetTarget(ctx context.Context, name string) (*config.Target, error) {
	var id int64
	var targetType, connType, connJSON string
	var storageName, schedule sql.NullString
	var compressEnabled, enabled bool
	var compressType sql.NullString
	var excludeTablesJSON, additionalArgsJSON string
	var createdAt, updatedAt time.Time

	query := `SELECT id, name, type, conn_type, conn_json, storage_name, schedule,
			  compress_enabled, compress_type, exclude_tables, additional_args, enabled, created_at, updated_at
			  FROM targets WHERE name = ?`
	err := s.db.QueryRowContext(ctx, query, name).Scan(&id, &name, &targetType, &connType, &connJSON, &storageName, &schedule,
		&compressEnabled, &compressType, &excludeTablesJSON, &additionalArgsJSON, &enabled, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("target not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query target: %w", err)
	}

	secrets, err := s.getSecrets(ctx, "target", id)
	if err != nil {
		return nil, fmt.Errorf("failed to get secrets: %w", err)
	}

	target, err := s.deserializeTarget(name, connType, connJSON, secrets, storageName, schedule,
		compressEnabled, compressType, excludeTablesJSON, additionalArgsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize target: %w", err)
	}

	return target, nil
}

// ListTargets retrieves all targets
func (s *Service) ListTargets(ctx context.Context) ([]*config.Target, error) {
	query := `SELECT id, name, type, conn_type, conn_json, storage_name, schedule,
			  compress_enabled, compress_type, exclude_tables, additional_args, enabled
			  FROM targets WHERE enabled = true`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query targets: %w", err)
	}
	defer rows.Close() //nolint:errcheck // best effort cleanup

	var targets []*config.Target
	for rows.Next() {
		var id int64
		var name, targetType, connType, connJSON string
		var storageName, schedule sql.NullString
		var compressEnabled, enabled bool
		var compressType sql.NullString
		var excludeTablesJSON, additionalArgsJSON string

		if err := rows.Scan(&id, &name, &targetType, &connType, &connJSON, &storageName, &schedule,
			&compressEnabled, &compressType, &excludeTablesJSON, &additionalArgsJSON, &enabled); err != nil {
			return nil, fmt.Errorf("failed to scan target: %w", err)
		}

		secrets, err := s.getSecrets(ctx, "target", id)
		if err != nil {
			return nil, fmt.Errorf("failed to get secrets: %w", err)
		}

		target, err := s.deserializeTarget(name, connType, connJSON, secrets, storageName, schedule,
			compressEnabled, compressType, excludeTablesJSON, additionalArgsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize target: %w", err)
		}

		targets = append(targets, target)
	}

	return targets, nil
}

// UpdateTarget updates an existing target
func (s *Service) UpdateTarget(ctx context.Context, name string, target *config.Target) error {
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM targets WHERE name = ?`, name).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("target not found: %s", name)
		}
		return fmt.Errorf("failed to query target: %w", err)
	}

	connJSON, connSecrets, err := s.serializeConnection(target.Conn)
	if err != nil {
		return fmt.Errorf("failed to serialize connection: %w", err)
	}

	var storageName sql.NullString
	if target.Storage != nil && target.Storage.Enabled {
		storageName = sql.NullString{String: target.Storage.Name, Valid: true}
	}

	var schedule sql.NullString
	if target.Schedule != "" {
		schedule = sql.NullString{String: target.Schedule, Valid: true}
	}

	var compressEnabled bool
	var compressType sql.NullString
	if target.Compress != nil {
		compressEnabled = target.Compress.Enabled
		if target.Compress.Type != "" {
			compressType = sql.NullString{String: target.Compress.Type, Valid: true}
		}
	}

	excludeTablesJSON, _ := json.Marshal(target.ExcludeTables)   //nolint:errcheck // marshaling string slice never fails
	additionalArgsJSON, _ := json.Marshal(target.AdditionalArgs) //nolint:errcheck // marshaling string slice never fails

	query := `UPDATE targets SET conn_type = ?, conn_json = ?, storage_name = ?, schedule = ?,
			  compress_enabled = ?, compress_type = ?, exclude_tables = ?, additional_args = ?, updated_at = ?
			  WHERE name = ?`
	if _, err := s.db.ExecContext(ctx, query, target.Conn.Type, connJSON, storageName, schedule,
		compressEnabled, compressType, string(excludeTablesJSON), string(additionalArgsJSON), time.Now(), name); err != nil {
		return fmt.Errorf("failed to update target: %w", err)
	}

	if err := s.updateSecrets(ctx, "target", id, connSecrets); err != nil {
		return fmt.Errorf("failed to update secrets: %w", err)
	}

	return nil
}

// UpdateTargetSchedule updates only the schedule of a target
func (s *Service) UpdateTargetSchedule(ctx context.Context, name string, schedule string) error {
	var sched sql.NullString
	if schedule != "" {
		sched = sql.NullString{String: schedule, Valid: true}
	}

	query := `UPDATE targets SET schedule = ?, updated_at = ? WHERE name = ?`
	result, err := s.db.ExecContext(ctx, query, sched, time.Now(), name)
	if err != nil {
		return fmt.Errorf("failed to update target schedule: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("target not found: %s", name)
	}

	return nil
}

// DeleteTarget deletes a target
func (s *Service) DeleteTarget(ctx context.Context, name string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM targets WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("failed to delete target: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("target not found: %s", name)
	}

	return nil
}

// RestoreTarget operations

// CreateRestoreTarget creates a new restore target configuration
func (s *Service) CreateRestoreTarget(ctx context.Context, rt *config.RestoreTarget) error {
	connJSON, connSecrets, err := s.serializeConnection(rt.Conn)
	if err != nil {
		return fmt.Errorf("failed to serialize connection: %w", err)
	}

	var storageName, sourceTarget, description sql.NullString
	if rt.Storage != nil && rt.Storage.Enabled {
		storageName = sql.NullString{String: rt.Storage.Name, Valid: true}
	}
	if rt.SourceTarget != "" {
		sourceTarget = sql.NullString{String: rt.SourceTarget, Valid: true}
	}
	if rt.Description != "" {
		description = sql.NullString{String: rt.Description, Valid: true}
	}

	query := `INSERT INTO restore_targets (name, conn_type, conn_json, storage_name, source_target,
			  description, enabled, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now()
	result, err := s.db.ExecContext(ctx, query, rt.Name, rt.Conn.Type, connJSON, storageName, sourceTarget, description, true, now, now)
	if err != nil {
		return fmt.Errorf("failed to insert restore_target: %w", err)
	}

	rtID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get restore_target ID: %w", err)
	}

	if err := s.storeSecrets(ctx, "restore_target", rtID, connSecrets); err != nil {
		return fmt.Errorf("failed to store secrets: %w", err)
	}

	return nil
}

// GetRestoreTarget retrieves a restore target by name
func (s *Service) GetRestoreTarget(ctx context.Context, name string) (*config.RestoreTarget, error) {
	var id int64
	var connType, connJSON string
	var storageName, sourceTarget, description sql.NullString
	var enabled bool
	var createdAt, updatedAt time.Time

	query := `SELECT id, name, conn_type, conn_json, storage_name, source_target, description, enabled, created_at, updated_at
			  FROM restore_targets WHERE name = ?`
	err := s.db.QueryRowContext(ctx, query, name).Scan(&id, &name, &connType, &connJSON, &storageName, &sourceTarget, &description, &enabled, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("restore_target not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query restore_target: %w", err)
	}

	secrets, err := s.getSecrets(ctx, "restore_target", id)
	if err != nil {
		return nil, fmt.Errorf("failed to get secrets: %w", err)
	}

	rt, err := s.deserializeRestoreTarget(name, connType, connJSON, secrets, storageName, sourceTarget, description)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize restore_target: %w", err)
	}

	return rt, nil
}

// ListRestoreTargets retrieves all restore targets
func (s *Service) ListRestoreTargets(ctx context.Context) ([]*config.RestoreTarget, error) {
	query := `SELECT id, name, conn_type, conn_json, storage_name, source_target, description, enabled
			  FROM restore_targets WHERE enabled = true`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query restore_targets: %w", err)
	}
	defer rows.Close() //nolint:errcheck // best effort cleanup

	var restoreTargets []*config.RestoreTarget
	for rows.Next() {
		var id int64
		var name, connType, connJSON string
		var storageName, sourceTarget, description sql.NullString
		var enabled bool

		if err := rows.Scan(&id, &name, &connType, &connJSON, &storageName, &sourceTarget, &description, &enabled); err != nil {
			return nil, fmt.Errorf("failed to scan restore_target: %w", err)
		}

		secrets, err := s.getSecrets(ctx, "restore_target", id)
		if err != nil {
			return nil, fmt.Errorf("failed to get secrets: %w", err)
		}

		rt, err := s.deserializeRestoreTarget(name, connType, connJSON, secrets, storageName, sourceTarget, description)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize restore_target: %w", err)
		}

		restoreTargets = append(restoreTargets, rt)
	}

	return restoreTargets, nil
}

// UpdateRestoreTarget updates an existing restore target
func (s *Service) UpdateRestoreTarget(ctx context.Context, name string, rt *config.RestoreTarget) error {
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM restore_targets WHERE name = ?`, name).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("restore_target not found: %s", name)
		}
		return fmt.Errorf("failed to query restore_target: %w", err)
	}

	connJSON, connSecrets, err := s.serializeConnection(rt.Conn)
	if err != nil {
		return fmt.Errorf("failed to serialize connection: %w", err)
	}

	var storageName, sourceTarget, description sql.NullString
	if rt.Storage != nil && rt.Storage.Enabled {
		storageName = sql.NullString{String: rt.Storage.Name, Valid: true}
	}
	if rt.SourceTarget != "" {
		sourceTarget = sql.NullString{String: rt.SourceTarget, Valid: true}
	}
	if rt.Description != "" {
		description = sql.NullString{String: rt.Description, Valid: true}
	}

	query := `UPDATE restore_targets SET conn_type = ?, conn_json = ?, storage_name = ?, source_target = ?,
			  description = ?, updated_at = ? WHERE name = ?`
	if _, err := s.db.ExecContext(ctx, query, rt.Conn.Type, connJSON, storageName, sourceTarget, description, time.Now(), name); err != nil {
		return fmt.Errorf("failed to update restore_target: %w", err)
	}

	if err := s.updateSecrets(ctx, "restore_target", id, connSecrets); err != nil {
		return fmt.Errorf("failed to update secrets: %w", err)
	}

	return nil
}

// DeleteRestoreTarget deletes a restore target
func (s *Service) DeleteRestoreTarget(ctx context.Context, name string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM restore_targets WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("failed to delete restore_target: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("restore_target not found: %s", name)
	}

	return nil
}

// Global config operations

// GetGlobalConfig retrieves a global config value
func (s *Service) GetGlobalConfig(ctx context.Context, key string) (string, error) {
	var value string
	query := `SELECT value FROM global_config WHERE key = ?`
	if err := s.db.QueryRowContext(ctx, query, key).Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("config key not found: %s", key)
		}
		return "", fmt.Errorf("failed to query global_config: %w", err)
	}
	return value, nil
}

// SetGlobalConfig sets a global config value
func (s *Service) SetGlobalConfig(ctx context.Context, key, value string) error {
	query := `INSERT INTO global_config (key, value, updated_at) VALUES (?, ?, ?)
			  ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = ?`
	now := time.Now()
	if _, err := s.db.ExecContext(ctx, query, key, value, now, value, now); err != nil {
		return fmt.Errorf("failed to set global_config: %w", err)
	}
	return nil
}

// ListGlobalConfig retrieves all global config values
func (s *Service) ListGlobalConfig(ctx context.Context) (map[string]string, error) {
	query := `SELECT key, value FROM global_config`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query global_config: %w", err)
	}
	defer rows.Close() //nolint:errcheck // best effort cleanup

	configs := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan global_config: %w", err)
		}
		configs[key] = value
	}

	return configs, nil
}
