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
	// backupOutcomeUnknown means the daemon could not establish the target's
	// history — the job store was unreachable, or the scan hit its cap before
	// reaching this target. It is never a claim about backups; "never" is.
	backupOutcomeUnknown = "unknown"
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

// overdueGrace caps how long after a due run a target is given before it counts
// as late. See isOverdue.
const overdueGrace = time.Hour

// historySample is the backup job history every rollup is derived from,
// together with what is missing from it.
//
// The two flags exist because an incomplete sample and a quiet install look
// identical once the jobs are in a slice, and on a backup dashboard the
// difference between "nothing failed" and "I cannot tell" is the whole point.
type historySample struct {
	// jobs are backup jobs ordered newest first by CreatedAt.
	jobs []*jobs.Job

	// truncated is set when the fetch returned dashboardHistoryLimit rows, so
	// older jobs — possibly a whole target's history — were cut off.
	truncated bool

	// degraded is set when the job store could not be read. Whatever is here
	// is in-memory only: everything from before the current process is absent.
	degraded bool
}

// complete reports whether a target's absence from the sample can be trusted
// to mean the target has never been backed up.
func (h historySample) complete() bool {
	return !h.truncated && !h.degraded
}

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
	// lastFinishedAt is when the newest finished job — success or failure —
	// finished. It decides outcome; it is not reported directly.
	lastFinishedAt *time.Time
}

// jobFinishedAt returns when a job reached a terminal state. Persisted rows
// always carry completed_at; the fallback keeps a job whose timestamps were
// never written from being silently dropped from the rollups.
//
// PRECONDITION: job.GetStatus() has already returned a terminal status
// (completed, failed or cancelled) on this goroutine. CompletedAt is guarded by
// the job's mutex and is written under the same Lock that publishes the
// terminal status, so the status read is what freezes it. Calling this on a job
// that may still be running is a data race — see the note on
// computeTargetHealth.
func jobFinishedAt(job *jobs.Job) time.Time {
	if job.CompletedAt != nil {
		return *job.CompletedAt
	}
	return job.CreatedAt
}

// backupArtifactSize returns the size a completed backup job recorded. Jobs
// restored from the store before results were decoded, and jobs that failed
// before producing an artifact, have no result to read.
//
// PRECONDITION: as jobFinishedAt — Result is mutex-guarded and only frozen once
// GetStatus() has reported a terminal status. A torn read of the interface
// would crash the type assertion below, not merely return a stale size.
func backupArtifactSize(job *jobs.Job) (int64, bool) {
	result, ok := job.Result.(*app.BackupResult)
	if !ok || result == nil || !result.Success {
		return 0, false
	}
	return result.Size, true
}

// computeTargetHealth rolls backup job history up per target.
//
// Ordering: jobs.Manager sorts history by CreatedAt, not by completion. A job
// created earlier can finish later — an overlapping or long-running backup does
// exactly that — so "most recent" is resolved with jobFinishedAt on every field
// here rather than by taking the first match in the slice. Nothing in this
// function depends on the order it is given.
//
// Concurrency: history can contain live jobs a worker is still mutating.
// StartedAt, CompletedAt and Result are only read after GetStatus() has
// reported a terminal state — MarkCompleted and MarkFailed write those fields
// and the status under one lock, so the locked read establishes the
// happens-before edge and the fields are frozen from then on. Do not hoist any
// of those reads out of the terminal-status branches.
func computeTargetHealth(history []*jobs.Job) map[string]*targetHealth {
	health := make(map[string]*targetHealth)

	// Failures are replayed once lastSuccessAt has settled, so each job's
	// status is read exactly once: a job that finishes between two passes would
	// otherwise be counted by one and missed by the other.
	type failedJob struct {
		entry      *targetHealth
		finishedAt time.Time
	}
	failures := make([]failedJob, 0, len(history))

	for _, job := range history {
		if job.Type != jobs.JobTypeBackup {
			continue
		}

		entry, ok := health[job.TargetName]
		if !ok {
			entry = &targetHealth{outcome: backupOutcomeNever}
			health[job.TargetName] = entry
		}

		if createdAt := job.CreatedAt; entry.oldestJobAt == nil || createdAt.Before(*entry.oldestJobAt) {
			entry.oldestJobAt = &createdAt
		}

		status := job.GetStatus()
		if status != jobs.JobStatusCompleted && status != jobs.JobStatusFailed {
			// Queued, running, cancelling and cancelled jobs report neither
			// success nor failure, so they do not move any of these counters.
			continue
		}

		finishedAt := jobFinishedAt(job)

		if entry.lastFinishedAt == nil || finishedAt.After(*entry.lastFinishedAt) {
			entry.lastFinishedAt = &finishedAt
			if status == jobs.JobStatusCompleted {
				entry.outcome = backupOutcomeSuccess
			} else {
				entry.outcome = backupOutcomeFailed
			}
		}

		if status == jobs.JobStatusFailed {
			failures = append(failures, failedJob{entry: entry, finishedAt: finishedAt})
			continue
		}

		if entry.lastSuccessAt != nil && !finishedAt.After(*entry.lastSuccessAt) {
			continue
		}

		entry.lastSuccessAt = &finishedAt
		entry.lastSuccessBytes = nil
		entry.lastSuccessSeconds = nil

		if size, ok := backupArtifactSize(job); ok {
			entry.lastSuccessBytes = &size
		}
		if job.StartedAt != nil && job.CompletedAt != nil {
			seconds := job.CompletedAt.Sub(*job.StartedAt).Seconds()
			entry.lastSuccessSeconds = &seconds
		}
	}

	// Only failures newer than the last success are consecutive, and the loop
	// above only settles lastSuccessAt once it has seen every job.
	for _, failure := range failures {
		if failure.entry.lastSuccessAt == nil || failure.finishedAt.After(*failure.entry.lastSuccessAt) {
			failure.entry.consecutiveFailures++
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
//
// The due time alone is not enough. A backup that takes four minutes on an
// hourly schedule is "due" from the top of every hour until it finishes, so
// flagging the moment the slot opens paints a healthy target red for the whole
// duration of its own run — while is_running says it is working. A target with
// a job in flight is therefore never overdue, and on top of that we allow
// overdueGrace: the seconds between a slot opening and the scheduler actually
// starting the job, plus a run that was skipped because the previous one was
// still going.
//
// The grace is capped rather than "the next fire" so lateness is still caught
// within an hour of a daily, monthly or yearly target going quiet. Waiting for
// the second fire of an @yearly schedule would mean noticing in two years.
func isOverdue(schedule string, entry *targetHealth, running bool, now time.Time) bool {
	if schedule == "" || entry == nil || running {
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

	due := parsed.Next(since)

	// One whole period, for schedules short enough that a period is the natural
	// grace — an every-5-minutes target is not late at 5 minutes and one second.
	grace := parsed.Next(due).Sub(due)
	if grace > overdueGrace {
		grace = overdueGrace
	}

	return due.Add(grace).Before(now)
}

// backupWindowStats returns the percentage of backup jobs finishing within the
// window that succeeded, and how many failed.
//
// Both are nil when the sample cannot cover the window: the store read failed,
// or dashboardHistoryLimit cut history off after the window started. The sample
// is then missing jobs from inside the window, and a success rate that is
// quietly wrong is worse on a backup dashboard than no success rate at all. The
// rate alone is nil when the window holds no finished backup job, because a
// rate over an empty sample is unknown, not 0%. A failure count of zero, in
// contrast, is a real answer.
func backupWindowStats(sample historySample, window time.Duration, now time.Time) (rate *float64, failed *int) {
	cutoff := now.Add(-window)

	if sample.degraded {
		return nil, nil
	}
	if sample.truncated && len(sample.jobs) > 0 && sample.jobs[len(sample.jobs)-1].CreatedAt.After(cutoff) {
		return nil, nil
	}

	succeeded, failures := 0, 0
	for _, job := range sample.jobs {
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
