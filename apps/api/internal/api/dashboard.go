package api

import (
	"math"
	"time"

	"github.com/etowett/bared/apps/api/internal/app"
	"github.com/etowett/bared/apps/api/internal/jobs"
)

// Outcome labels reported in TargetSummary.LastBackupStatus.
const (
	backupOutcomeSuccess = "success"
	backupOutcomeFailed  = "failed"
	backupOutcomeNever   = "never"
)

// dashboardHistoryLimit caps how many backup jobs the rollups scan. It exists
// so the endpoint stays bounded on installs with a long history; when the cap
// is hit, windows that the retrieved slice cannot fully cover report no rate
// instead of one computed from a truncated sample.
const dashboardHistoryLimit = 5000

// Rollup windows.
const (
	window24h = 24 * time.Hour
	window7d  = 7 * 24 * time.Hour
)

// targetHealth is the per-target rollup derived from backup job history.
type targetHealth struct {
	// lastSuccessAt is when the most recent successful backup finished.
	lastSuccessAt *time.Time
	// lastSuccessBytes is the artifact size that backup recorded, if the job
	// still carries its result.
	lastSuccessBytes *int64
	// lastSuccessSeconds is how long that backup job ran.
	lastSuccessSeconds *float64
	// outcome is the most recent finished outcome, or backupOutcomeNever.
	outcome string
	// consecutiveFailures counts failures newer than the last success.
	consecutiveFailures int
	// oldestJobAt is the creation time of the oldest backup job we know of for
	// the target. It is the only evidence of when a target started running.
	oldestJobAt *time.Time
}

// jobFinishedAt returns when a job reached a terminal state. Persisted rows
// always carry completed_at; the fallback keeps a job whose timestamps were
// never written from being silently dropped from the rollups.
//
// Only call this once GetStatus() has reported a terminal state — see the note
// on computeTargetHealth.
func jobFinishedAt(job *jobs.Job) time.Time {
	if job.CompletedAt != nil {
		return *job.CompletedAt
	}
	return job.CreatedAt
}

// backupArtifactSize returns the size a completed backup job recorded. Jobs
// restored from the store before results were decoded, and jobs that failed
// before producing an artifact, have no result to read.
func backupArtifactSize(job *jobs.Job) (int64, bool) {
	result, ok := job.Result.(*app.BackupResult)
	if !ok || result == nil || !result.Success {
		return 0, false
	}
	return result.Size, true
}

// computeTargetHealth rolls backup job history up per target. history must be
// ordered newest first, which is what jobs.Manager.ListJobsFiltered guarantees.
//
// Concurrency: history can contain live jobs a worker is still mutating.
// StartedAt, CompletedAt and Result are only read after GetStatus() has
// reported a terminal state — MarkCompleted and MarkFailed write those fields
// and the status under one lock, so the locked read establishes the
// happens-before edge and the fields are frozen from then on. Do not hoist any
// of those reads out of the terminal-status branches.
func computeTargetHealth(history []*jobs.Job) map[string]*targetHealth {
	health := make(map[string]*targetHealth)

	for _, job := range history {
		if job.Type != jobs.JobTypeBackup {
			continue
		}

		entry, ok := health[job.TargetName]
		if !ok {
			entry = &targetHealth{outcome: backupOutcomeNever}
			health[job.TargetName] = entry
		}

		// history runs newest first, so the last assignment is the oldest job.
		createdAt := job.CreatedAt
		entry.oldestJobAt = &createdAt

		switch job.GetStatus() {
		case jobs.JobStatusCompleted:
			if entry.outcome == backupOutcomeNever {
				entry.outcome = backupOutcomeSuccess
			}
			if entry.lastSuccessAt == nil {
				finishedAt := jobFinishedAt(job)
				entry.lastSuccessAt = &finishedAt

				if size, ok := backupArtifactSize(job); ok {
					entry.lastSuccessBytes = &size
				}
				if job.StartedAt != nil && job.CompletedAt != nil {
					seconds := job.CompletedAt.Sub(*job.StartedAt).Seconds()
					entry.lastSuccessSeconds = &seconds
				}
			}

		case jobs.JobStatusFailed:
			if entry.outcome == backupOutcomeNever {
				entry.outcome = backupOutcomeFailed
			}
			// Only failures newer than the last success are consecutive.
			if entry.lastSuccessAt == nil {
				entry.consecutiveFailures++
			}

		default:
			// Queued, running, cancelling and cancelled jobs report neither
			// success nor failure, so they do not move any of these counters.
		}
	}

	return health
}

// isOverdue reports whether a scheduled target has missed a run.
//
// The reference point is the last successful backup: the schedule's next fire
// after it is when a fresh backup was due. With no success on record we fall
// back to the oldest known job, which is the earliest moment we can prove the
// target existed. With no history at all we return false — nothing records when
// a target was configured, and claiming a brand-new target is overdue would be
// a guess.
func isOverdue(schedule string, entry *targetHealth, now time.Time) bool {
	if schedule == "" || entry == nil {
		return false
	}

	var since time.Time
	switch {
	case entry.lastSuccessAt != nil:
		since = *entry.lastSuccessAt
	case entry.oldestJobAt != nil:
		since = *entry.oldestJobAt
	default:
		return false
	}

	parsed, err := parseCronSchedule(schedule)
	if err != nil {
		return false
	}

	return parsed.Next(since).Before(now)
}

// backupWindowStats returns the percentage of backup jobs finishing within the
// window that succeeded, and how many failed.
//
// Both are nil when history was truncated by dashboardHistoryLimit before
// reaching the start of the window: the sample would then be missing older jobs
// from the same window, and a success rate that is quietly wrong is worse on a
// backup dashboard than no success rate at all. The rate alone is nil when the
// window holds no finished backup job, because a rate over an empty sample is
// unknown, not 0%. A failure count of zero, in contrast, is a real answer.
func backupWindowStats(history []*jobs.Job, truncated bool, window time.Duration, now time.Time) (rate *float64, failed *int) {
	cutoff := now.Add(-window)

	if truncated && len(history) > 0 && history[len(history)-1].CreatedAt.After(cutoff) {
		return nil, nil
	}

	succeeded, failures := 0, 0
	for _, job := range history {
		if job.Type != jobs.JobTypeBackup {
			continue
		}

		status := job.GetStatus()
		if status != jobs.JobStatusCompleted && status != jobs.JobStatusFailed {
			continue
		}
		if jobFinishedAt(job).Before(cutoff) {
			continue
		}

		if status == jobs.JobStatusCompleted {
			succeeded++
		} else {
			failures++
		}
	}

	total := succeeded + failures
	if total == 0 {
		return nil, &failures
	}

	// One decimal place: further digits are noise on a sample this small.
	percent := math.Round(float64(succeeded)/float64(total)*1000) / 10
	return &percent, &failures
}
