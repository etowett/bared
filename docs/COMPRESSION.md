# Compression Options in BareD

## Overview

BareD supports two compression strategies with different memory and performance characteristics. Choose based on your database size and available memory.

## Supported Compression Types

### 1. **`gzip` (Recommended for Large Databases)**

**Memory Usage:** Constant ~32KB
**Use Case:** Large databases (10GB+), especially 100GB+
**Process:** Streams data directly from database dump → gzip compression → output

```yaml
compress:
  enabled: true
  type: gzip  # or "gz" (alias)
```

**How it works:**
- Uses a fixed 32KB buffer that gets reused
- Compresses data as it streams through
- **Never buffers the entire database in memory**
- Memory usage stays constant regardless of database size

**Example Memory Profile (100GB database):**
```
Time    Memory Usage    Stage
0:00    50 MB          Initializing
0:10    80 MB          Dumping + Compressing (32KB buffer)
1:20    85 MB          Still compressing (same buffer)
2:30    90 MB          Upload starting
3:00    75 MB          Complete
```

**Best for:**
- ✅ Databases over 10GB
- ✅ Systems with limited RAM
- ✅ Preventing OOM kills
- ✅ Streaming backups to remote storage
- ✅ Docker containers with memory limits

---

### 2. **`tgz` (For Small Databases Only)**

**Memory Usage:** Entire database size + compression overhead
**Use Case:** Small databases (<1GB) where tar archive format is needed
**Process:** Database dump → buffer in RAM → create tar → compress → output

```yaml
compress:
  enabled: true
  type: tgz  # or "tar.gz" (alias)
```

**How it works:**
- Buffers the **entire database dump in memory**
- Creates a tar archive with the buffered data
- Compresses the tar archive
- Writes to disk/storage

**Example Memory Profile (100GB database):**
```
Time    Memory Usage         Stage                     Risk
0:00    50 MB               Initializing              ✓
0:10    4.1 GB              Dumping + Buffering       ✓
0:30    14.3 GB             Still buffering           ⚠️
1:00    24.3 GB             Nearly done buffering     ⚠️
1:15    29.5 GB             Tar creation              ❌ OOM KILL
1:15    Container restarts  Process killed by OOM     💥
```

**Risks:**
- ❌ **OOM (Out of Memory) kills** on large databases
- ❌ Memory usage equals database size
- ❌ Container crashes and restarts
- ❌ Orphaned jobs in the job queue
- ❌ System instability

**Only use when:**
- Database is **guaranteed** under 1GB
- You specifically need tar archive format
- RAM is abundant (database size × 3 available)

---

## Migration Guide: tgz → gzip

If you're experiencing OOM kills with large databases, switch to `gzip`:

### Before (OOM Risk):
```yaml
targets:
  - name: large_postgres
    conn:
      type: postgres
      database: production_db
      # ... connection details
    compress:
      enabled: true
      type: tgz  # ❌ Will buffer entire DB in memory
```

### After (OOM Safe):
```yaml
targets:
  - name: large_postgres
    conn:
      type: postgres
      database: production_db
      # ... connection details
    compress:
      enabled: true
      type: gzip  # ✅ Constant memory, no buffering
```

**Note:** Existing `.tar.gz` backups can still be restored. BareD will detect the format automatically.

---

## Compression Format Comparison

| Feature | `gzip` | `tgz` |
|---------|--------|-------|
| **File Extension** | `.gz` | `.tar.gz` |
| **Memory Usage** | Constant 32KB | Full database size |
| **Max Database Size** | Unlimited | Limited by RAM |
| **Streaming** | ✅ Yes | ❌ No (buffers all) |
| **OOM Safe** | ✅ Yes | ❌ No (for large DBs) |
| **Speed** | Fast | Slower (memory ops) |
| **Archive Format** | Single compressed file | Tar archive |
| **Restore** | ✅ Supported | ✅ Supported |

---

## Best Practices

### 1. **Default to `gzip` for All Production Databases**

```yaml
compress:
  enabled: true
  type: gzip
```

**Reasons:**
- No memory surprises as database grows
- Consistent memory usage across all targets
- Safer for automated backups
- Better for Docker/Kubernetes deployments

### 2. **Monitor Memory During Backups**

Even with `gzip`, monitor memory to ensure:
- Database client memory is reasonable
- Upload buffers are sized correctly
- No memory leaks in long-running processes

### 3. **Set Container Memory Limits Appropriately**

```yaml
# docker-compose.yml
services:
  bared:
    deploy:
      resources:
        limits:
          memory: 2G  # Enough for gzip streaming
```

For `gzip`: Base memory + 200-500MB overhead
For `tgz`: Database size × 3 minimum

### 4. **Use Local Storage for Initial Backup**

For very large databases (100GB+):

```yaml
storages:
  local_staging:
    type: local
    path: /mnt/fast-disk/backups
    keep: 2

  s3_archive:
    type: s3
    bucket: long-term-backups
    # ... s3 config

targets:
  - name: huge_postgres
    compress:
      enabled: true
      type: gzip
    storage:
      enabled: true
      name: local_staging  # Fast local write
```

Then use a separate process to upload to S3:
```bash
aws s3 sync /mnt/fast-disk/backups s3://long-term-backups/ --delete
```

---

## Troubleshooting

### OOM Kills with `tgz`

**Symptoms:**
- Container restarts during backup
- `dmesg` shows: `Out of memory: Killed process`
- Memory usage grows continuously during backup
- Orphaned jobs in job queue

**Solution:**
```yaml
compress:
  type: gzip  # Change from tgz
```

### Backup Files Not Compatible

**Q:** Will changing from `tgz` to `gzip` break restores?

**A:** No. BareD auto-detects compression format during restore. Both formats remain supported.

### Performance Concerns

**Q:** Is `gzip` slower than `tgz`?

**A:** No, `gzip` is typically faster because:
- No memory allocation overhead
- No tar archive creation step
- Streams directly to output
- Better CPU cache locality

---

## Configuration Aliases

The following are equivalent:

```yaml
type: gzip   = type: gz
type: tgz    = type: tar.gz
```

Use whichever is clearest for your team.

---

## Summary

**For production and large databases:** Use `gzip`
- ✅ Safe memory usage
- ✅ No OOM risk
- ✅ Scales to any database size
- ✅ Faster for large datasets

**For small dev databases (<1GB):** Either works
- `gzip` is still safer and recommended
- `tgz` is fine if you prefer tar archives

**Never use `tgz` for databases over 10GB** - you will experience OOM kills.
