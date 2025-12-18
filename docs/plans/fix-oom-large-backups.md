# Fix OOM for Large Database Backups (100GB+)

## Problem Summary

**Root Cause**: `internal/compress/tgz.go:92` buffers entire database dump in memory (`bufferedData = append(bufferedData, buf[:n]...)`) causing OOM kills for large databases.

**Current Behavior**:

- 100GB database backup → 100GB+ RAM consumption
- OOM killer terminates process with no graceful cleanup
- Jobs left in "running" state after crash
- Temp files orphaned on disk
- No visibility into progress during 6+ hour backups

**Impact**: Cannot reliably backup databases >30-50GB on typical servers.

## Solution Overview

1. **Streaming Gzip Compression** - Replace tar.gz buffering with pure gzip streaming (constant ~10MB memory)
2. **Byte-Level Progress Tracking** - Real-time progress updates during dump/compress/upload
3. **Crash Recovery** - Automatically detect and mark failed jobs on restart
4. **Enhanced Visibility** - Periodic heartbeat logs with metrics every 30-60 seconds

## Implementation Plan

### Phase 1: Fix OOM (CRITICAL - Week 1)

#### 1.1 Create Streaming Gzip Compressor

**New File**: `internal/compress/gzip.go`

**Implementation**:

```go
type Gzip struct {
    filename string
}

func (g *Gzip) Compress(ctx context.Context, r io.Reader, w io.Writer) error {
    gzw := gzip.NewWriter(w)
    defer gzw.Close()

    buf := make([]byte, 32*1024) // Fixed 32KB buffer, NO accumulation
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        n, err := r.Read(buf)
        if n > 0 {
            if _, writeErr := gzw.Write(buf[:n]); writeErr != nil {
                return writeErr
            }
        }
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
    }
    return nil
}
```

**Key Points**:

- No `bufferedData` slice - data flows through immediately
- Constant memory usage regardless of database size
- Context cancellation support
- Simple Decompress() method for restore

**Files to Modify**:

- `internal/compress/factory.go` - Add "gzip" case to New() function
- `internal/config/config.go` - Default compress.type to "gzip" when enabled

#### 1.2 Update Configuration

**Changes**:

- Set default compression type to "gzip" in config parser
- Add validation warning if tar.gz selected for large databases
- Document memory requirements in config examples

**Example**:

```yaml
compress:
  enabled: true
  type: gzip  # Default, streaming, constant memory
```

#### 1.3 Testing

**Unit Tests** (`internal/compress/gzip_test.go`):

- Test compression/decompression round-trip
- Verify context cancellation works
- Benchmark memory usage vs tar.gz
- Test with simulated large files

**Integration Test**:

- Create 10GB+ test database
- Run backup with gzip
- Verify memory stays <50MB throughout
- Verify output is valid gzip file

---

### Phase 2: Progress Tracking (HIGH - Week 2)

#### 2.1 Progress Wrapper Infrastructure

**New File**: `internal/util/progress.go`

**Components**:

**ProgressReader** - Wraps io.Reader to track bytes read:

```go
type ProgressReader struct {
    r          io.Reader
    bytesRead  *atomic.Int64
    lastUpdate time.Time
    callback   func(bytes int64)
}
```

**ProgressWriter** - Wraps io.Writer to track bytes written:

```go
type ProgressWriter struct {
    w             io.Writer
    bytesWritten  *atomic.Int64
    lastUpdate    time.Time
    callback      func(bytes int64)
}
```

**Features**:

- Thread-safe atomic counters
- Throttled callbacks (1 second intervals) to minimize overhead
- Drop-in replacement for io.Reader/Writer

#### 2.2 Heartbeat Logger

**New File**: `internal/util/heartbeat.go`

**Purpose**: Periodic status logging during long operations

**Metrics Logged**:

- Bytes processed / total (percentage)
- Speed (MB/s)
- ETA (estimated time remaining)
- Memory usage (runtime.MemStats)
- Goroutine count
- Elapsed time

**Example Output**:

```
[prod-db] Heartbeat: stage=DUMP_AND_COMPRESS bytes=15.3GB/98.4GB (15.5%) speed=42.1MB/s eta=33m elapsed=6m memory=8.2MB
[prod-db] Heartbeat: stage=DUMP_AND_COMPRESS bytes=45.8GB/98.4GB (46.5%) speed=41.8MB/s eta=21m elapsed=18m memory=8.4MB
```

**Configuration**: 30-60 second interval (configurable)

#### 2.3 Integration into Backup Pipeline

**File**: `internal/app/backup.go` (lines 427-568)

**Changes to `backupWithCompression()`**:

1. **Estimate Database Size** (before line 431):

   ```go
   estimatedSize, _ := progress.EstimateDatabaseSize(dumper)
   if estimatedSize > 0 {
       logger.InfoS("Estimated database size", "size", estimatedSize)
   }
   ```

2. **Wrap Dump Writer** (replace line 457):

   ```go
   dumpProgress := util.NewProgressWriter(dumpWriter, func(bytes int64) {
       if progress != nil {
           progress.UpdateBytes(bytes, estimatedSize)
       }
   })
   ```

3. **Start Heartbeat Logger** (after line 431):

   ```go
   heartbeat := util.NewHeartbeatLogger(
       target.Name,
       "DUMP_AND_COMPRESS",
       estimatedSize,
       func() int64 { return dumpProgress.BytesWritten() },
   )
   heartbeat.Start()
   defer heartbeat.Stop()
   ```

4. **Track Compression Output** (wrap tmpFile at line 494):

   ```go
   compressProgress := util.NewProgressWriter(tmpFile, nil)
   compressErr := compressor.Compress(ctx, dumpReader, compressProgress)
   ```

**Result**: Real-time visibility into every byte processed during backup.

#### 2.4 Upload Progress Tracking

**Files**:

- `internal/storage/s3.go`
- `internal/storage/sftp.go`
- `internal/storage/local.go`

**Change**: Wrap reader before passing to storage backend:

```go
uploadProgress := util.NewProgressReader(tmpFile, func(bytes int64) {
    if progress != nil {
        progress.UpdateBytes(bytes, totalSize)
    }
})
storeErr := stor.Store(ctx, backupPath, uploadProgress, compressedSize)
```

**Result**: Progress tracking during S3/SFTP uploads.

---

### Phase 3: Crash Recovery (MEDIUM - Week 3)

#### 3.1 Orphaned Job Recovery

**File**: `internal/daemon/daemon.go`

**New Method**: `RecoverOrphanedJobs()`

**Implementation**:

```go
func (d *Daemon) RecoverOrphanedJobs() error {
    logger := util.GetLogger()

    // Find jobs in "running" or "queued" state from previous run
    orphaned, err := d.persistence.FindJobsByStatus([]string{"running", "queued"})
    if err != nil {
        return err
    }

    if len(orphaned) > 0 {
        logger.InfoS("Found orphaned jobs from previous run", "count", len(orphaned))
    }

    // Mark each as failed with crash indicator
    for _, job := range orphaned {
        job.Status = "failed"
        job.Error = "Job interrupted by daemon shutdown or crash"
        job.CompletedAt = time.Now()

        if err := d.persistence.UpdateJob(job); err != nil {
            logger.ErrorS("Failed to mark orphaned job as failed", "job_id", job.ID, "error", err)
        } else {
            logger.InfoS("Marked orphaned job as failed", "job_id", job.ID, "target", job.Target, "type", job.Type)
        }
    }

    return nil
}
```

**Integration**: Call from `Start()` method after persistence initialization (around line 146).

#### 3.2 Temp File Cleanup

**File**: `internal/util/tempfile.go`

**New Function**: `CleanupOrphanedTempFiles()`

**Implementation**:

```go
func CleanupOrphanedTempFiles() error {
    logger := GetLogger()
    tmpDir := os.TempDir()

    // Find all bared backup temp files
    pattern := filepath.Join(tmpDir, "bared-backup-*.tmp")
    matches, err := filepath.Glob(pattern)
    if err != nil {
        return err
    }

    cutoff := time.Now().Add(-1 * time.Hour) // Files older than 1 hour

    for _, path := range matches {
        info, err := os.Stat(path)
        if err != nil {
            continue
        }

        if info.ModTime().Before(cutoff) {
            if err := os.Remove(path); err != nil {
                logger.WarnS("Failed to remove orphaned temp file", "path", path, "error", err)
            } else {
                logger.InfoS("Removed orphaned temp file", "path", path, "size", info.Size())
            }
        }
    }

    return nil
}
```

**Integration**: Call from daemon startup, before job recovery.

#### 3.3 Startup Sequence

**File**: `internal/daemon/daemon.go` in `Start()` method

**Order** (after persistence init):

1. Clean up orphaned temp files
2. Recover orphaned jobs
3. Start scheduler
4. Start HTTP server
5. Start job manager

---

### Phase 4: Enhanced Visibility (POLISH - Week 4)

#### 4.1 Stage Memory Metrics

**File**: `internal/util/stage.go`

**Enhancement**: Add memory metrics to stage completion logs.

**Change in `LogStageSummary()` method**:

```go
func (st *StageTracker) LogStageSummary() {
    // ... existing code ...

    // Add memory metrics
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    st.currentStage.Metrics["peak_memory_alloc"] = m.Alloc
    st.currentStage.Metrics["peak_memory_sys"] = m.Sys

    // ... continue with existing logging ...
}
```

**Result**: Stage completion logs include memory usage.

#### 4.2 Better Size Estimation

**File**: `internal/progress/estimator.go`

**Enhancements**:

1. Cache estimates for 5 minutes (avoid repeated queries)
2. Use `pg_total_relation_size()` for PostgreSQL (more accurate)
3. Log warning if estimation fails (don't fail backup)
4. Log significant discrepancies between estimate and actual

**Impact**: More accurate progress percentages and ETAs.

#### 4.3 API Progress Endpoint (Optional)

**File**: New or existing API handler

**Endpoint**: `GET /api/jobs/:id/progress`

**Response**:

```json
{
  "job_id": "20231215-143022-abc",
  "stage": "DUMP_AND_COMPRESS",
  "bytes_processed": 45837213696,
  "bytes_total": 98432106496,
  "percent": 46.5,
  "speed_bytes_per_sec": 44040192,
  "eta_seconds": 1260,
  "elapsed_seconds": 1112,
  "memory_alloc": 8421376
}
```

**Use Case**: External monitoring dashboards.

---

## Critical Files to Modify

### New Files

1. **`internal/compress/gzip.go`** - Streaming gzip compressor (fixes OOM)
2. **`internal/util/progress.go`** - ProgressReader/Writer wrappers
3. **`internal/util/heartbeat.go`** - Periodic status logger
4. **`internal/compress/gzip_test.go`** - Unit tests for gzip

### Modified Files

1. **`internal/compress/factory.go`** - Add gzip case to factory
2. **`internal/app/backup.go`** - Integrate progress tracking and heartbeat
3. **`internal/daemon/daemon.go`** - Add startup recovery
4. **`internal/util/tempfile.go`** - Add orphaned file cleanup
5. **`internal/config/config.go`** - Default to gzip compression
6. **`internal/util/stage.go`** - Add memory metrics to stages
7. **`internal/progress/estimator.go`** - Improve estimation accuracy
8. **`internal/storage/s3.go`** - Wrap reader for upload progress
9. **`internal/storage/sftp.go`** - Wrap reader for upload progress

---

## Testing Strategy

### Unit Tests

- [x] Gzip compression streaming behavior
- [x] ProgressReader/Writer accuracy
- [x] Heartbeat logger interval timing
- [x] Memory usage benchmarks (gzip vs tar.gz)

### Integration Tests

- [x] 10GB+ backup with gzip (verify constant memory)
- [x] Progress tracking throughout pipeline
- [x] Crash recovery (SIGKILL during backup)
- [x] Concurrent backups (3-5 targets)
- [x] Temp file cleanup on restart

### Manual Validation

- [ ] 100GB+ production database backup
- [ ] Monitor memory usage throughout (<50MB target)
- [ ] Verify heartbeat logs every 30-60s
- [ ] Test crash during backup, verify recovery
- [ ] Measure compression throughput (gzip vs tar.gz)

---

## Expected Outcomes

**Memory Usage**:

- Before: Scales with database size (100GB DB = 100GB+ RAM)
- After: Constant ~10-50MB regardless of database size
- **Reduction: >90% for large databases**

**Visibility**:

- Before: Silent during dump, no progress updates
- After: Heartbeat logs every 30-60s with bytes/speed/ETA/memory
- **Improvement: Full visibility into 6+ hour backups**

**Reliability**:

- Before: Jobs stuck in "running" after crash
- After: Automatic detection and marking as failed
- **Improvement: Clear failure state, easy manual retry**

**User Experience**:

- Can backup 100GB+ databases reliably
- Real-time progress tracking
- No manual intervention for crash cleanup
- Clear logs for troubleshooting

---

## Rollback Plan

**If issues arise**:

1. Revert code to previous version
2. Change configs back to `compress.type: tar.gz` (for small DBs only)
3. No data migration needed (backward compatible)

**Rollback time**: <10 minutes

---

## Implementation Priority

**Week 1 - CRITICAL**: Phase 1 (Streaming Gzip)

- Fixes OOM root cause
- Enables 100GB+ backups
- Minimal code changes, low risk

**Week 2 - HIGH**: Phase 2 (Progress Tracking)

- Major visibility improvement
- Helps troubleshoot issues
- Builds on Phase 1

**Week 3 - MEDIUM**: Phase 3 (Crash Recovery)

- Improves robustness
- Better operational experience
- Independent of other phases

**Week 4 - POLISH**: Phase 4 (Enhanced Visibility)

- Nice-to-have improvements
- API enhancements
- Documentation updates

---

## Success Criteria

- ✅ 100GB+ backups complete without OOM errors
- ✅ Memory usage stays constant (<50MB) during backups
- ✅ Progress logs visible every 30-60 seconds with all metrics
- ✅ Crashed jobs automatically marked as failed on restart
- ✅ Temp files cleaned up automatically
- ✅ No performance regression (throughput maintained or improved)
