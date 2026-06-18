# Storage Subsystem — Agent Guide

> Scope: storage backends (Local filesystem, S3 / S3-compatible, SFTP) with retry logic in `internal/storage/`. Part of the BareD AGENTS.md tree — see the root [`AGENTS.md`](../../AGENTS.md) and the backend guide [`internal/AGENTS.md`](../AGENTS.md). **The innermost guide wins** when instructions conflict.

Every backend implements the `Storage` interface in `storage.go` — `Store`, `Retrieve`, `List`, `Delete`, `Name`, `Validate`, `Exists`, and `GetInfo` — returning `*BackupInfo` metadata where applicable. Network-facing backends (S3, SFTP) wrap their operations in `util.Retry(ctx, util.DefaultRetryConfig(), fn)` so transient failures back off and retry automatically, while the local filesystem backend runs without retries. Backends are constructed through the factory `New(cfg *config.Storage)` in `factory.go`, which dispatches on `cfg.Type` (`local`, `s3`, `sftp`) to the matching `NewLocal`/`NewS3`/`NewSFTP` constructor.

## Adding a New Storage Backend

**Example**: Adding Azure Blob Storage

1. **Create implementation** (`internal/storage/azure.go`):

```go
package storage

import (
    "context"
    "fmt"
    "io"
    "path/filepath"
    "bared/internal/config"
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

2. **Update config** (`internal/config/config.go`):

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

3. **Register in factory** (`internal/storage/factory.go`):

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

- [`../AGENTS.md`](../AGENTS.md) — backend (`internal/`) guide
- [`../../AGENTS.md`](../../AGENTS.md) — root BareD agent guide
- [`../database/AGENTS.md`](../database/AGENTS.md) — database subsystem guide
- [`../notify/AGENTS.md`](../notify/AGENTS.md) — notification subsystem guide
