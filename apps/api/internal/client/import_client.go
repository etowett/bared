package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/etowett/bared/apps/api/internal/api"
	"github.com/etowett/bared/apps/api/internal/config"
)

// ImportClient provides HTTP API client for config import operations
type ImportClient struct {
	baseURL    string
	httpClient *http.Client
	user       string
	pass       string
}

// NewImportClient creates a new import client
func NewImportClient(baseURL, user, pass string, timeout time.Duration) *ImportClient {
	// Ensure baseURL doesn't have trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &ImportClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		user: user,
		pass: pass,
	}
}

// Ping checks if the API is reachable and credentials are valid
func (c *ImportClient) Ping(ctx context.Context) error {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/config/source", nil)
	if err != nil {
		return fmt.Errorf("failed to connect to API: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed: invalid username or password")
	}

	if err := parseAPIError(resp); err != nil {
		return err
	}

	return nil
}

// Resource existence checkers

// StorageExists checks if a storage with the given name exists
func (c *ImportClient) StorageExists(ctx context.Context, name string) (bool, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/config/storages/"+name, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if err := parseAPIError(resp); err != nil {
		return false, err
	}

	return true, nil
}

// NotifierExists checks if a notifier with the given name exists
func (c *ImportClient) NotifierExists(ctx context.Context, name string) (bool, error) {
	// List all notifiers and check if name exists
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/config/notifiers", nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := parseAPIError(resp); err != nil {
		return false, err
	}

	var listResp api.ListNotifiersResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	for _, notifier := range listResp.Notifiers {
		if notifier.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// TargetExists checks if a target with the given name exists
func (c *ImportClient) TargetExists(ctx context.Context, name string) (bool, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/config/targets", nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := parseAPIError(resp); err != nil {
		return false, err
	}

	var listResp api.ListTargetsConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	for _, target := range listResp.Targets {
		if target.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// RestoreTargetExists checks if a restore target with the given name exists
func (c *ImportClient) RestoreTargetExists(ctx context.Context, name string) (bool, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/config/restore-targets", nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := parseAPIError(resp); err != nil {
		return false, err
	}

	var listResp api.ListRestoreTargetsConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	for _, rt := range listResp.RestoreTargets {
		if rt.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// Create operations

// CreateStorage creates a new storage configuration
func (c *ImportClient) CreateStorage(ctx context.Context, storage *config.Storage) error {
	req := storageToAPIRequest(storage)
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/config/storages", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	return parseAPIError(resp)
}

// CreateNotifier creates a new notifier configuration
func (c *ImportClient) CreateNotifier(ctx context.Context, name string, notifier *config.Notifier) error {
	req := notifierToAPIRequest(name, notifier)
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/config/notifiers", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	return parseAPIError(resp)
}

// CreateTarget creates a new target configuration
func (c *ImportClient) CreateTarget(ctx context.Context, target *config.Target) error {
	req := targetToAPIRequest(target)
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/config/targets", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	return parseAPIError(resp)
}

// CreateRestoreTarget creates a new restore target configuration
func (c *ImportClient) CreateRestoreTarget(ctx context.Context, rt *config.RestoreTarget) error {
	req := restoreTargetToAPIRequest(rt)
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/config/restore-targets", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	return parseAPIError(resp)
}

// Update operations

// UpdateStorage updates an existing storage configuration
func (c *ImportClient) UpdateStorage(ctx context.Context, name string, storage *config.Storage) error {
	req := storageToAPIRequest(storage)
	resp, err := c.doRequest(ctx, http.MethodPut, "/api/config/storages/"+name, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	return parseAPIError(resp)
}

// UpdateNotifier updates an existing notifier configuration
func (c *ImportClient) UpdateNotifier(ctx context.Context, name string, notifier *config.Notifier) error {
	req := notifierToAPIRequest(name, notifier)
	resp, err := c.doRequest(ctx, http.MethodPut, "/api/config/notifiers/"+name, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	return parseAPIError(resp)
}

// UpdateTarget updates an existing target configuration
func (c *ImportClient) UpdateTarget(ctx context.Context, name string, target *config.Target) error {
	req := targetToAPIRequest(target)
	resp, err := c.doRequest(ctx, http.MethodPut, "/api/config/targets/"+name, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	return parseAPIError(resp)
}

// UpdateRestoreTarget updates an existing restore target configuration
func (c *ImportClient) UpdateRestoreTarget(ctx context.Context, name string, rt *config.RestoreTarget) error {
	req := restoreTargetToAPIRequest(rt)
	resp, err := c.doRequest(ctx, http.MethodPut, "/api/config/restore-targets/"+name, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	return parseAPIError(resp)
}

// Global config operations

// SetGlobalConfig sets a global configuration value
func (c *ImportClient) SetGlobalConfig(ctx context.Context, key, value string) error {
	req := api.UpdateGlobalConfigRequest{Value: value}
	resp, err := c.doRequest(ctx, http.MethodPut, "/api/config/global/"+key, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	return parseAPIError(resp)
}

// GetGlobalConfig gets all global configuration values
func (c *ImportClient) GetGlobalConfig(ctx context.Context) (map[string]string, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/config/global", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := parseAPIError(resp); err != nil {
		return nil, err
	}

	var globalResp api.GlobalConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&globalResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	configs := make(map[string]string)
	if globalResp.DefaultStorage != "" {
		configs["default_storage"] = globalResp.DefaultStorage
	}
	if globalResp.LogLevel != "" {
		configs["log_level"] = globalResp.LogLevel
	}
	if globalResp.LogFormat != "" {
		configs["log_format"] = globalResp.LogFormat
	}

	return configs, nil
}

// Helper methods

// doRequest performs an HTTP request with authentication
func (c *ImportClient) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	fullURL := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set basic auth
	req.SetBasicAuth(c.user, c.pass)

	// Set headers
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// parseAPIError checks the response status and returns an error if not successful
func parseAPIError(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Try to read error message from response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("HTTP %d %s (failed to read body: %v)", resp.StatusCode, resp.Status, err)
	}

	// Try to parse as JSON error response
	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(bodyBytes, &errResp); err == nil {
		if errResp.Error != "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, errResp.Error)
		}
		if errResp.Message != "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, errResp.Message)
		}
	}

	// Fallback to raw body (truncate if too long)
	bodyStr := string(bodyBytes)
	if bodyStr != "" {
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "..."
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, bodyStr)
	}

	return fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
}

// Conversion functions from config types to API request types

func storageToAPIRequest(storage *config.Storage) api.StorageRequest {
	req := api.StorageRequest{
		Name:   storage.Name,
		Type:   storage.Type,
		Keep:   storage.Keep,
		Config: make(map[string]interface{}),
	}

	switch storage.Type {
	case "local":
		if storage.Path != "" {
			req.Config["path"] = storage.Path
		}
	case "s3":
		if storage.Bucket != "" {
			req.Config["bucket"] = storage.Bucket
		}
		// Key prefix inside the bucket (#104).
		if storage.Path != "" {
			req.Config["path"] = storage.Path
		}
		if storage.Region != "" {
			req.Config["region"] = storage.Region
		}
		if storage.AccessKeyID != "" {
			req.Config["access_key_id"] = storage.AccessKeyID
		}
		if storage.EndpointURL != "" {
			req.Config["endpoint_url"] = storage.EndpointURL
		}
		req.SecretAccessKey = storage.SecretAccessKey
	case "sftp":
		if storage.Host != "" {
			req.Config["host"] = storage.Host
		}
		// Remote base directory (#104) — without it `brd config import` wrote
		// a pathless SFTP backend into the DB, and the DB wins over the YAML
		// at load, so the backups moved to the SSH login directory.
		if storage.Path != "" {
			req.Config["path"] = storage.Path
		}
		if storage.Port > 0 {
			req.Config["port"] = storage.Port
		}
		if storage.Username != "" {
			req.Config["username"] = storage.Username
		}
		// Host key settings must survive the import or a verified SFTP backend
		// silently reverts to the ~/.ssh/known_hosts default on the daemon's
		// host — and a key-only config would be rejected outright for having no
		// credentials.
		if storage.KnownHostsPath != "" {
			req.Config["known_hosts_path"] = storage.KnownHostsPath
		}
		if storage.HostKeyFingerprint != "" {
			req.Config["host_key_fingerprint"] = storage.HostKeyFingerprint
		}
		if storage.PrivateKeyPath != "" {
			req.Config["private_key_path"] = storage.PrivateKeyPath
		}
		if storage.InsecureSkipHostKeyVerify {
			req.Config["insecure_skip_host_key_verify"] = true
		}
		req.Password = storage.Password
		req.PrivateKeyPassphrase = storage.PrivateKeyPassphrase
	}

	return req
}

func notifierToAPIRequest(name string, notifier *config.Notifier) api.NotifierRequest {
	req := api.NotifierRequest{
		Name:      name,
		Type:      notifier.Type,
		OnSuccess: notifier.OnSuccess,
		Config:    make(map[string]interface{}),
	}

	// Common fields
	if notifier.URL != "" {
		req.Config["url"] = notifier.URL
	}
	if notifier.Channel != "" {
		req.Config["channel"] = notifier.Channel
	}

	// Email fields
	if notifier.SMTPHost != "" {
		req.Config["smtp_host"] = notifier.SMTPHost
		req.Config["smtp_port"] = notifier.SMTPPort
		req.Config["smtp_username"] = notifier.SMTPUsername
		req.Config["smtp_from"] = notifier.SMTPFrom
		req.Config["smtp_to"] = notifier.SMTPTo
		req.Config["smtp_use_tls"] = notifier.SMTPUseTLS
		req.SMTPPassword = notifier.SMTPPassword
	}

	// Webhook fields
	if notifier.WebhookMethod != "" {
		req.Config["webhook_method"] = notifier.WebhookMethod
	}
	if len(notifier.WebhookHeaders) > 0 {
		req.Config["webhook_headers"] = notifier.WebhookHeaders
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
		req.Config["webhook_auth"] = authMap
		req.WebhookAuthPassword = notifier.WebhookAuth.Password
		req.WebhookAuthToken = notifier.WebhookAuth.Token
		req.WebhookAuthHeaderVal = notifier.WebhookAuth.HeaderValue
	}

	return req
}

func targetToAPIRequest(target *config.Target) api.TargetRequest {
	req := api.TargetRequest{
		Name: target.Name,
		Connection: api.ConnectionRequest{
			Type:     target.Conn.Type,
			User:     target.Conn.User,
			Password: target.Conn.Password,
			Database: target.Conn.Database,
			Host:     target.Conn.Host,
			Port:     target.Conn.Port,
		},
		ExcludeTables:  target.ExcludeTables,
		AdditionalArgs: target.AdditionalArgs,
		Schedule:       target.Schedule,
	}

	if target.Storage != nil && target.Storage.Enabled {
		req.StorageName = target.Storage.Name
	}

	if target.Compress != nil {
		req.Compress = &api.CompressionConfig{
			Enabled: target.Compress.Enabled,
			Type:    target.Compress.Type,
		}
	}

	return req
}

func restoreTargetToAPIRequest(rt *config.RestoreTarget) api.RestoreTargetRequest {
	req := api.RestoreTargetRequest{
		Name: rt.Name,
		Connection: api.ConnectionRequest{
			Type:     rt.Conn.Type,
			User:     rt.Conn.User,
			Password: rt.Conn.Password,
			Database: rt.Conn.Database,
			Host:     rt.Conn.Host,
			Port:     rt.Conn.Port,
		},
		SourceTarget: rt.SourceTarget,
		Description:  rt.Description,
	}

	if rt.Storage != nil && rt.Storage.Enabled {
		req.StorageName = rt.Storage.Name
	}

	return req
}
