# Database Subsystem — Agent Guide

> Scope: dumpers & restorers for each database engine (MySQL/MariaDB, PostgreSQL, Redis) in `internal/database/`. Part of the BareD AGENTS.md tree — see the root [`AGENTS.md`](../../AGENTS.md) and the backend guide [`internal/AGENTS.md`](../AGENTS.md) for project-wide workflow and Go conventions. **The innermost guide wins** when instructions conflict.

This package abstracts each supported database engine behind two small interfaces in `database.go`: a `Dumper` (`Dump(ctx, w)` plus `Name()` and `Validate()`) for producing a backup stream, and a `Restorer` (`Restore(ctx, r)` plus `Name()` and `ValidateConnection()`) for consuming one. Concrete types — `MySQL`, `Postgres`, and `Redis` — implement both interfaces and wrap the engine's CLI tools (`mysqldump`/`mysql`, `pg_dump`/`psql`, `redis-cli`) via the `internal/util` command executors, streaming data to/from `io.Writer`/`io.Reader` rather than buffering whole dumps. A factory (`factory.go`) dispatches on `target.Conn.Type` through `NewDumper` and `NewRestorer`, so adding an engine means implementing the interfaces once and registering the new type in the factory switch.

## Adding a New Database Type

**Example**: Adding MongoDB support

1. **Create implementation** (`internal/database/mongodb.go`):

```go
package database

import (
    "context"
    "fmt"
    "io"
    "bared/internal/config"
    "bared/internal/util"
)

type MongoDB struct {
    conn *config.Connection
}

func NewMongoDB(conn *config.Connection) *MongoDB {
    return &MongoDB{conn: conn}
}

// Dump implements Dumper interface
func (m *MongoDB) Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error) {
    // Build mongodump command
    args := []string{
        "--host", m.conn.Host,
        "--port", fmt.Sprintf("%d", m.conn.Port),
        "--archive",  // Stream to stdout
    }

    if m.conn.User != "" {
        args = append(args, "--username", m.conn.User)
    }
    if m.conn.Password != "" {
        args = append(args, "--password", m.conn.Password)
    }
    if m.conn.Database != "" {
        args = append(args, "--db", m.conn.Database)
    }

    // Execute with streaming output
    executor := util.NewShellExecutor("mongodump", args...)
    size, err := executor.Execute(ctx, w)
    if err != nil {
        return nil, fmt.Errorf("mongodump failed: %w", err)
    }

    return &DumpMetadata{
        DatabaseType: "mongodb",
        DatabaseName: m.conn.Database,
        Size:         size,
    }, nil
}

// Restore implements Restorer interface
func (m *MongoDB) Restore(ctx context.Context, r io.Reader) error {
    args := []string{
        "--host", m.conn.Host,
        "--port", fmt.Sprintf("%d", m.conn.Port),
        "--archive",  // Read from stdin
    }

    // Add auth if provided
    if m.conn.User != "" {
        args = append(args, "--username", m.conn.User)
    }
    if m.conn.Password != "" {
        args = append(args, "--password", m.conn.Password)
    }

    executor := util.NewShellExecutor("mongorestore", args...)
    _, err := executor.ExecuteWithInput(ctx, r)
    return err
}

func (m *MongoDB) Name() string {
    return "mongodb"
}

func (m *MongoDB) Validate(ctx context.Context) error {
    // Check if mongodump is available
    if !util.CommandExists("mongodump") {
        return fmt.Errorf("mongodump command not found")
    }
    if !util.CommandExists("mongorestore") {
        return fmt.Errorf("mongorestore command not found")
    }
    return nil
}

func (m *MongoDB) ValidateConnection(ctx context.Context) error {
    // Test connection (could use mongo client library)
    return nil
}
```

2. **Register in factory** (`internal/database/factory.go`):

```go
func New(target *config.Target) (Dumper, error) {
    switch target.Conn.Type {
    case "mysql":
        return NewMySQL(target.Conn), nil
    case "postgres":
        return NewPostgres(target.Conn), nil
    case "redis":
        return NewRedis(target.Conn), nil
    case "mongodb":  // ADD THIS
        return NewMongoDB(target.Conn), nil
    default:
        return nil, fmt.Errorf("unsupported database type: %s", target.Conn.Type)
    }
}

func NewRestorer(target *config.Target) (Restorer, error) {
    switch target.Conn.Type {
    case "mysql":
        return NewMySQL(target.Conn), nil
    case "postgres":
        return NewPostgres(target.Conn), nil
    case "redis":
        return NewRedis(target.Conn), nil
    case "mongodb":  // ADD THIS
        return NewMongoDB(target.Conn), nil
    default:
        return nil, fmt.Errorf("unsupported database type: %s", target.Conn.Type)
    }
}
```

> Note: the live factory in `factory.go` names the dumper constructor `NewDumper` (not `New`), and the existing `NewMySQL`/`NewPostgres` constructors also take `target.ExcludeTables` and `target.AdditionalArgs`. Match the signatures of the current code when you register a new engine; the snippet above shows the dispatch shape.

3. **Update validator** (`internal/config/validator.go`):

```go
func (v *Validator) validateConnection(conn *Connection, fieldPath string) error {
    // ... existing validation

    // Validate type
    validTypes := []string{"mysql", "postgres", "redis", "mongodb"}  // ADD mongodb
    if !contains(validTypes, conn.Type) {
        return fmt.Errorf("%s.type: must be one of %v", fieldPath, validTypes)
    }

    // ... rest of validation
}
```

4. **Add tests** (`internal/database/mongodb_test.go`):

```go
package database

import (
    "testing"
    "context"
    "bytes"
    "bared/internal/config"
    "github.com/stretchr/testify/assert"
)

func TestMongoDBDump(t *testing.T) {
    // Skip if mongodump not available
    if !util.CommandExists("mongodump") {
        t.Skip("mongodump not available")
    }

    conn := &config.Connection{
        Type:     "mongodb",
        Host:     "localhost",
        Port:     27017,
        Database: "testdb",
        User:     "testuser",
        Password: "testpass",
    }

    db := NewMongoDB(conn)

    var buf bytes.Buffer
    meta, err := db.Dump(context.Background(), &buf)

    assert.NoError(t, err)
    assert.NotNil(t, meta)
    assert.Equal(t, "mongodb", meta.DatabaseType)
    assert.Greater(t, buf.Len(), 0)
}
```

5. **Update documentation**:
   - Add example to `examples/config.example.yml`
   - Update README.md features list
   - Update `docs/user-guide/configuration.md`

## See also

- [`../AGENTS.md`](../AGENTS.md) — backend guide (`internal/`), Go conventions and workflow
- [`../../AGENTS.md`](../../AGENTS.md) — root BareD agent guide
- [`../storage/AGENTS.md`](../storage/AGENTS.md) — storage backend extension guide (sibling subsystem)
- [`../notify/AGENTS.md`](../notify/AGENTS.md) — notification extension guide (sibling subsystem)
