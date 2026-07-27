# Storage Subsystem — Agent Guide

> Scope: storage backends (Local filesystem, S3 / S3-compatible, SFTP) with retry logic in `apps/api/internal/storage/`. Part of the BareD AGENTS.md tree — see the root [`AGENTS.md`](../../../../AGENTS.md) and the backend guide [`apps/api/internal/AGENTS.md`](../AGENTS.md). **The innermost guide wins** when instructions conflict.

Every backend implements the `Storage` interface in `storage.go` — `Store`, `Retrieve`, `List`, `Delete`, `Name`, `Validate`, `Exists`, and `GetInfo` — returning `*BackupInfo` metadata where applicable. Network-facing backends (S3, SFTP) wrap their operations in `util.Retry(ctx, util.DefaultRetryConfig(), fn)` so transient failures back off and retry automatically, while the local filesystem backend runs without retries. Backends are constructed through the factory `New(cfg *config.Storage)` in `factory.go`, which dispatches on `cfg.Type` (`local`, `s3`, `sftp`) to the matching `NewLocal`/`NewS3`/`NewSFTP` constructor.

## SFTP host key verification

The SFTP backend verifies the server's host key by default and **refuses to connect when it cannot** — an unverified SFTP session hands an on-path attacker both the credentials and the dump it is carrying. The logic lives in `sftp_hostkey.go`; `connect()` in `sftp.go` only consumes it. Precedence:

1. `insecure_skip_host_key_verify: true` — accept any key. Explicit opt-in only, warned about at daemon startup (`daemon.warnInsecureStorages`) and at backend construction.
2. `host_key_fingerprint: "SHA256:…"` — pin exactly one key.
3. `known_hosts_path` (default `~/.ssh/known_hosts`) — OpenSSH `known_hosts` via `golang.org/x/crypto/ssh/knownhosts`.

Authentication is `private_key_path` (+ optional `private_key_passphrase`) and/or `password`; at least one is required, and the config validators enforce it.

Rules when touching this area:

- **Every rejection must name the fix.** Failing closed on a host key is only useful if the operator can tell *which* file to edit and *what* to put in it — see `describeKeyError`, which distinguishes an unknown host (setup step) from an unlisted key type (configuration) from a real mismatch (possible attack).
- **Never log or wrap a key, passphrase, or password into an error.** `loadPrivateKey` carries the path only.
- **New SFTP config fields need five touch points**, or they silently revert to zero on the next start: `config.Storage`, `config/validator.go` **and** `configservice/validator.go`, `configservice/secrets.go` (both `serializeStorage` *and* `deserializeStorage`), `api/config_handlers.go` (`requestToStorage` + `storageToResponse`, with secrets redacted), and `client/import_client.go` (`storageToAPIRequest` — `brd config import` otherwise drops the field on the way into the DB, and the DB wins over YAML at load).

## Adding a New Storage Backend

**Example**: Adding Azure Blob Storage

1. **Create implementation** (`apps/api/internal/storage/azure.go`):

```go
package storage

import (
    "context"
    "fmt"
    "io"
    "path/filepath"
    "github.com/etowett/bared/apps/api/internal/config"
    // Import Azure SDK
)

type AzureStorage struct {
    cfg         *config.Storage
    client      *azblob.Client
    container   string
    retryPolicy *util.RetryPolicy
}

func NewAzureStorage(cfg *config.Storage) (*AzureStorage, error) {
    // Initialize Azure client
    client, err := azblob.NewClient(cfg.AccountURL, credential, nil)
    if err != nil {
        return nil, err
    }

    return &AzureStorage{
        cfg:         cfg,
        client:      client,
        container:   cfg.Container,
        retryPolicy: util.NewRetryPolicy(3, time.Second),
    }, nil
}

func (a *AzureStorage) Store(ctx context.Context, path string, r io.Reader, size int64) error {
    // Use retry policy for network operations
    return a.retryPolicy.Do(ctx, func() error {
        blobClient := a.client.ServiceClient().NewContainerClient(a.container).NewBlockBlobClient(path)
        _, err := blobClient.UploadStream(ctx, r, &azblob.UploadStreamOptions{})
        return err
    })
}

func (a *AzureStorage) Retrieve(ctx context.Context, path string, w io.Writer) error {
    return a.retryPolicy.Do(ctx, func() error {
        blobClient := a.client.ServiceClient().NewContainerClient(a.container).NewBlockBlobClient(path)
        downloadResponse, err := blobClient.DownloadStream(ctx, nil)
        if err != nil {
            return err
        }
        defer downloadResponse.Body.Close()

        _, err = io.Copy(w, downloadResponse.Body)
        return err
    })
}

// Implement remaining Storage interface methods...
```

2. **Update config** (`apps/api/internal/config/config.go`):

```go
type Storage struct {
    Type     string `yaml:"type"`
    // ... existing fields

    // Azure-specific fields
    AccountURL  string `yaml:"account_url,omitempty"`
    AccountKey  string `yaml:"account_key,omitempty"`
    Container   string `yaml:"container,omitempty"`
}
```

3. **Register in factory** (`apps/api/internal/storage/factory.go`):

```go
func New(cfg *config.Storage) (Storage, error) {
    switch cfg.Type {
    case "local":
        return NewLocalStorage(cfg), nil
    case "s3":
        return NewS3Storage(cfg)
    case "sftp":
        return NewSFTPStorage(cfg)
    case "azure":  // ADD THIS
        return NewAzureStorage(cfg)
    default:
        return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
    }
}
```

## See also

- [`../AGENTS.md`](../AGENTS.md) — backend (`apps/api/internal/`) guide
- [`../../AGENTS.md`](../../../../AGENTS.md) — root BareD agent guide
- [`../database/AGENTS.md`](../database/AGENTS.md) — database subsystem guide
- [`../notify/AGENTS.md`](../notify/AGENTS.md) — notification subsystem guide
