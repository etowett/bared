package configservice

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/etowett/bared/apps/api/internal/config"
)

// Secret represents a key-value pair to be encrypted
type Secret struct {
	FieldName string
	Value     string
}

// Serialization and deserialization helpers

// serializeStorage converts a Storage to JSON and extracts secrets
func (s *Service) serializeStorage(storage *config.Storage) (string, []Secret, error) {
	// Create a map for non-secret fields
	configMap := make(map[string]interface{})

	// Type-specific fields
	switch storage.Type {
	case "local":
		if storage.Path != "" {
			configMap["path"] = storage.Path
		}
	case "s3":
		if storage.Bucket != "" {
			configMap["bucket"] = storage.Bucket
		}
		if storage.Region != "" {
			configMap["region"] = storage.Region
		}
		if storage.AccessKeyID != "" {
			configMap["access_key_id"] = storage.AccessKeyID
		}
		if storage.EndpointURL != "" {
			configMap["endpoint_url"] = storage.EndpointURL
		}
		// SecretAccessKey is a secret - handled separately
	case "sftp":
		if storage.Host != "" {
			configMap["host"] = storage.Host
		}
		if storage.Port > 0 {
			configMap["port"] = storage.Port
		}
		if storage.Username != "" {
			configMap["username"] = storage.Username
		}
		if storage.KnownHostsPath != "" {
			configMap["known_hosts_path"] = storage.KnownHostsPath
		}
		if storage.HostKeyFingerprint != "" {
			configMap["host_key_fingerprint"] = storage.HostKeyFingerprint
		}
		if storage.PrivateKeyPath != "" {
			configMap["private_key_path"] = storage.PrivateKeyPath
		}
		if storage.InsecureSkipHostKeyVerify {
			configMap["insecure_skip_host_key_verify"] = true
		}
		// Password and private_key_passphrase are secrets - handled separately
	}

	configJSON, err := json.Marshal(configMap)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Extract secrets
	var secrets []Secret
	if storage.SecretAccessKey != "" {
		secrets = append(secrets, Secret{FieldName: "secret_access_key", Value: storage.SecretAccessKey})
	}
	if storage.Password != "" {
		secrets = append(secrets, Secret{FieldName: "password", Value: storage.Password})
	}
	if storage.PrivateKeyPassphrase != "" {
		secrets = append(secrets, Secret{FieldName: "private_key_passphrase", Value: storage.PrivateKeyPassphrase})
	}

	return string(configJSON), secrets, nil
}

// deserializeStorage converts JSON and secrets back to a Storage
func (s *Service) deserializeStorage(name, storageType, configJSON string, keep int, secrets map[string]string) (*config.Storage, error) {
	storage := &config.Storage{
		Name: name,
		Type: storageType,
		Keep: keep,
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Type-specific fields
	switch storageType {
	case "local":
		if path, ok := configMap["path"].(string); ok {
			storage.Path = path
		}
	case "s3":
		if bucket, ok := configMap["bucket"].(string); ok {
			storage.Bucket = bucket
		}
		if region, ok := configMap["region"].(string); ok {
			storage.Region = region
		}
		if accessKeyID, ok := configMap["access_key_id"].(string); ok {
			storage.AccessKeyID = accessKeyID
		}
		if endpointURL, ok := configMap["endpoint_url"].(string); ok {
			storage.EndpointURL = endpointURL
		}
		if secretKey, ok := secrets["secret_access_key"]; ok {
			storage.SecretAccessKey = secretKey
		}
	case "sftp":
		if host, ok := configMap["host"].(string); ok {
			storage.Host = host
		}
		if port, ok := configMap["port"].(float64); ok {
			storage.Port = int(port)
		}
		if username, ok := configMap["username"].(string); ok {
			storage.Username = username
		}
		if knownHostsPath, ok := configMap["known_hosts_path"].(string); ok {
			storage.KnownHostsPath = knownHostsPath
		}
		if fingerprint, ok := configMap["host_key_fingerprint"].(string); ok {
			storage.HostKeyFingerprint = fingerprint
		}
		if privateKeyPath, ok := configMap["private_key_path"].(string); ok {
			storage.PrivateKeyPath = privateKeyPath
		}
		if insecure, ok := configMap["insecure_skip_host_key_verify"].(bool); ok {
			storage.InsecureSkipHostKeyVerify = insecure
		}
		if password, ok := secrets["password"]; ok {
			storage.Password = password
		}
		if passphrase, ok := secrets["private_key_passphrase"]; ok {
			storage.PrivateKeyPassphrase = passphrase
		}
	}

	return storage, nil
}

// serializeNotifier converts a Notifier to JSON and extracts secrets
func (s *Service) serializeNotifier(notifier *config.Notifier) (string, []Secret, error) {
	configMap := make(map[string]interface{})

	// Common fields
	if notifier.URL != "" {
		configMap["url"] = notifier.URL
	}
	if notifier.Channel != "" {
		configMap["channel"] = notifier.Channel
	}

	// Email fields
	if notifier.SMTPHost != "" {
		configMap["smtp_host"] = notifier.SMTPHost
	}
	if notifier.SMTPPort > 0 {
		configMap["smtp_port"] = notifier.SMTPPort
	}
	if notifier.SMTPUsername != "" {
		configMap["smtp_username"] = notifier.SMTPUsername
	}
	if notifier.SMTPFrom != "" {
		configMap["smtp_from"] = notifier.SMTPFrom
	}
	if len(notifier.SMTPTo) > 0 {
		configMap["smtp_to"] = notifier.SMTPTo
	}
	configMap["smtp_use_tls"] = notifier.SMTPUseTLS

	// Webhook fields
	if notifier.WebhookMethod != "" {
		configMap["webhook_method"] = notifier.WebhookMethod
	}
	if len(notifier.WebhookHeaders) > 0 {
		configMap["webhook_headers"] = notifier.WebhookHeaders
	}
	if notifier.WebhookAuth != nil {
		authMap := make(map[string]interface{})
		authMap["type"] = notifier.WebhookAuth.Type
		if notifier.WebhookAuth.Username != "" {
			authMap["username"] = notifier.WebhookAuth.Username
		}
		if notifier.WebhookAuth.HeaderName != "" {
			authMap["header_name"] = notifier.WebhookAuth.HeaderName
		}
		configMap["webhook_auth"] = authMap
		// Password, Token, HeaderValue are secrets - handled separately
	}

	configJSON, err := json.Marshal(configMap)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Extract secrets
	var secrets []Secret
	if notifier.SMTPPassword != "" {
		secrets = append(secrets, Secret{FieldName: "smtp_password", Value: notifier.SMTPPassword})
	}
	if notifier.WebhookAuth != nil {
		if notifier.WebhookAuth.Password != "" {
			secrets = append(secrets, Secret{FieldName: "webhook_auth_password", Value: notifier.WebhookAuth.Password})
		}
		if notifier.WebhookAuth.Token != "" {
			secrets = append(secrets, Secret{FieldName: "webhook_auth_token", Value: notifier.WebhookAuth.Token})
		}
		if notifier.WebhookAuth.HeaderValue != "" {
			secrets = append(secrets, Secret{FieldName: "webhook_auth_header_value", Value: notifier.WebhookAuth.HeaderValue})
		}
	}

	return string(configJSON), secrets, nil
}

// deserializeNotifier converts JSON and secrets back to a Notifier
func (s *Service) deserializeNotifier(notifierType, configJSON string, onSuccess bool, secrets map[string]string) (*config.Notifier, error) {
	notifier := &config.Notifier{
		Type:      notifierType,
		OnSuccess: onSuccess,
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Common fields
	if url, ok := configMap["url"].(string); ok {
		notifier.URL = url
	}
	if channel, ok := configMap["channel"].(string); ok {
		notifier.Channel = channel
	}

	// Email fields
	if smtpHost, ok := configMap["smtp_host"].(string); ok {
		notifier.SMTPHost = smtpHost
	}
	if smtpPort, ok := configMap["smtp_port"].(float64); ok {
		notifier.SMTPPort = int(smtpPort)
	}
	if smtpUsername, ok := configMap["smtp_username"].(string); ok {
		notifier.SMTPUsername = smtpUsername
	}
	if smtpFrom, ok := configMap["smtp_from"].(string); ok {
		notifier.SMTPFrom = smtpFrom
	}
	if smtpTo, ok := configMap["smtp_to"].([]interface{}); ok {
		for _, to := range smtpTo {
			if toStr, ok := to.(string); ok {
				notifier.SMTPTo = append(notifier.SMTPTo, toStr)
			}
		}
	}
	if smtpUseTLS, ok := configMap["smtp_use_tls"].(bool); ok {
		notifier.SMTPUseTLS = smtpUseTLS
	}
	if smtpPassword, ok := secrets["smtp_password"]; ok {
		notifier.SMTPPassword = smtpPassword
	}

	// Webhook fields
	if webhookMethod, ok := configMap["webhook_method"].(string); ok {
		notifier.WebhookMethod = webhookMethod
	}
	if webhookHeaders, ok := configMap["webhook_headers"].(map[string]interface{}); ok {
		notifier.WebhookHeaders = make(map[string]string)
		for k, v := range webhookHeaders {
			if vStr, ok := v.(string); ok {
				notifier.WebhookHeaders[k] = vStr
			}
		}
	}
	if webhookAuth, ok := configMap["webhook_auth"].(map[string]interface{}); ok {
		notifier.WebhookAuth = &config.WebhookAuth{}
		if authType, ok := webhookAuth["type"].(string); ok {
			notifier.WebhookAuth.Type = authType
		}
		if username, ok := webhookAuth["username"].(string); ok {
			notifier.WebhookAuth.Username = username
		}
		if headerName, ok := webhookAuth["header_name"].(string); ok {
			notifier.WebhookAuth.HeaderName = headerName
		}
		if password, ok := secrets["webhook_auth_password"]; ok {
			notifier.WebhookAuth.Password = password
		}
		if token, ok := secrets["webhook_auth_token"]; ok {
			notifier.WebhookAuth.Token = token
		}
		if headerValue, ok := secrets["webhook_auth_header_value"]; ok {
			notifier.WebhookAuth.HeaderValue = headerValue
		}
	}

	return notifier, nil
}

// serializeConnection converts a Connection to JSON and extracts secrets
func (s *Service) serializeConnection(conn *config.Connection) (string, []Secret, error) {
	configMap := make(map[string]interface{})

	if conn.User != "" {
		configMap["user"] = conn.User
	}
	if conn.Database != "" {
		configMap["database"] = conn.Database
	}
	if conn.Host != "" {
		configMap["host"] = conn.Host
	}
	if conn.Port > 0 {
		configMap["port"] = conn.Port
	}

	configJSON, err := json.Marshal(configMap)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Extract secrets
	var secrets []Secret
	if conn.Password != "" {
		secrets = append(secrets, Secret{FieldName: "password", Value: conn.Password})
	}

	return string(configJSON), secrets, nil
}

// deserializeConnection converts JSON and secrets back to a Connection
func (s *Service) deserializeConnection(connType, configJSON string, secrets map[string]string) (*config.Connection, error) {
	conn := &config.Connection{
		Type: connType,
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if user, ok := configMap["user"].(string); ok {
		conn.User = user
	}
	if database, ok := configMap["database"].(string); ok {
		conn.Database = database
	}
	if host, ok := configMap["host"].(string); ok {
		conn.Host = host
	}
	if port, ok := configMap["port"].(float64); ok {
		conn.Port = int(port)
	}
	if password, ok := secrets["password"]; ok {
		conn.Password = password
	}

	return conn, nil
}

// deserializeTarget converts JSON and secrets back to a Target
func (s *Service) deserializeTarget(name, connType, connJSON string, secrets map[string]string,
	storageName, schedule sql.NullString, compressEnabled bool, compressType sql.NullString,
	excludeTablesJSON, additionalArgsJSON string) (*config.Target, error) {

	conn, err := s.deserializeConnection(connType, connJSON, secrets)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize connection: %w", err)
	}

	target := &config.Target{
		Name: name,
		Conn: conn,
	}

	if storageName.Valid {
		target.Storage = &config.TargetStorage{
			Enabled: true,
			Name:    storageName.String,
		}
	}

	if schedule.Valid {
		target.Schedule = schedule.String
	}

	if compressEnabled {
		target.Compress = &config.CompressionOpts{
			Enabled: true,
		}
		if compressType.Valid {
			target.Compress.Type = compressType.String
		}
	}

	if excludeTablesJSON != "" && excludeTablesJSON != "null" {
		if err := json.Unmarshal([]byte(excludeTablesJSON), &target.ExcludeTables); err != nil {
			return nil, fmt.Errorf("failed to unmarshal exclude_tables: %w", err)
		}
	}

	if additionalArgsJSON != "" && additionalArgsJSON != "null" {
		if err := json.Unmarshal([]byte(additionalArgsJSON), &target.AdditionalArgs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal additional_args: %w", err)
		}
	}

	return target, nil
}

// deserializeRestoreTarget converts JSON and secrets back to a RestoreTarget
func (s *Service) deserializeRestoreTarget(name, connType, connJSON string, secrets map[string]string,
	storageName, sourceTarget, description sql.NullString) (*config.RestoreTarget, error) {

	conn, err := s.deserializeConnection(connType, connJSON, secrets)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize connection: %w", err)
	}

	rt := &config.RestoreTarget{
		Name: name,
		Conn: conn,
	}

	if storageName.Valid {
		rt.Storage = &config.TargetStorage{
			Enabled: true,
			Name:    storageName.String,
		}
	}

	if sourceTarget.Valid {
		rt.SourceTarget = sourceTarget.String
	}

	if description.Valid {
		rt.Description = description.String
	}

	return rt, nil
}

// Secret storage and retrieval

// storeSecrets encrypts and stores secrets in the database
func (s *Service) storeSecrets(ctx context.Context, refType string, refID int64, secrets []Secret) error {
	if len(secrets) == 0 {
		return nil
	}

	for _, secret := range secrets {
		encrypted, err := s.encryption.Encrypt(secret.Value)
		if err != nil {
			return fmt.Errorf("failed to encrypt secret %s: %w", secret.FieldName, err)
		}

		query := `INSERT INTO secrets (ref_type, ref_id, field_name, encrypted_value, created_at, updated_at)
				  VALUES (?, ?, ?, ?, ?, ?)`
		now := time.Now()
		if _, err := s.db.ExecContext(ctx, query, refType, refID, secret.FieldName, encrypted, now, now); err != nil {
			return fmt.Errorf("failed to insert secret: %w", err)
		}
	}

	return nil
}

// getSecrets retrieves and decrypts secrets from the database
func (s *Service) getSecrets(ctx context.Context, refType string, refID int64) (map[string]string, error) {
	query := `SELECT field_name, encrypted_value FROM secrets WHERE ref_type = ? AND ref_id = ?`
	rows, err := s.db.QueryContext(ctx, query, refType, refID)
	if err != nil {
		return nil, fmt.Errorf("failed to query secrets: %w", err)
	}
	defer rows.Close() //nolint:errcheck // best effort cleanup

	secrets := make(map[string]string)
	for rows.Next() {
		var fieldName, encryptedValue string
		if err := rows.Scan(&fieldName, &encryptedValue); err != nil {
			return nil, fmt.Errorf("failed to scan secret: %w", err)
		}

		decrypted, err := s.encryption.Decrypt(encryptedValue)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt secret %s: %w", fieldName, err)
		}

		secrets[fieldName] = decrypted
	}

	return secrets, nil
}

// updateSecrets updates secrets for a config item (replaces all secrets)
func (s *Service) updateSecrets(ctx context.Context, refType string, refID int64, secrets []Secret) error {
	// Delete existing secrets
	if _, err := s.db.ExecContext(ctx, `DELETE FROM secrets WHERE ref_type = ? AND ref_id = ?`, refType, refID); err != nil {
		return fmt.Errorf("failed to delete old secrets: %w", err)
	}

	// Insert new secrets
	return s.storeSecrets(ctx, refType, refID, secrets)
}
