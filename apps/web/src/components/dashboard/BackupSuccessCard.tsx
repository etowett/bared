import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Dashboard } from '@/types'
import { formatSize } from './format'
import { UnknownValue } from './UnknownValue'

type SuccessRates = Pick<
  Dashboard,
  'success_rate_24h' | 'success_rate_7d' | 'failed_jobs_24h' | 'total_storage_bytes'
>

/*
 * `failed_jobs_24h` has its own headline card above, so it is not repeated as a
 * row here — it is read only to tell an empty 24-hour sample apart from a
 * truncated history.
 */

/**
 * The daemon's own rollups, reported exactly as far as they go.
 *
 * Every figure here is optional in the contract, and each absence means
 * something different, so each gets its own sentence rather than a shared
 * "N/A":
 *
 * - `success_rate_24h` is null when no backup finished in the window. A rate
 *   over an empty sample is not 0%. `failed_jobs_24h` tells the two apart:
 *   present-and-zero means nothing finished, absent means history was
 *   truncated and neither figure could be established.
 * - `success_rate_7d` is null unless the daemon persists job history — in
 *   memory it is pruned well inside 7 days.
 * - `total_storage_bytes` is never populated. The backend refuses to infer it
 *   from job history (which counts backups retention has since deleted) or
 *   from live listings (slow, and billable on S3), so this reads "unavailable"
 *   permanently — and there is no storage-growth panel, because there is no
 *   honest data to draw one from.
 */
export function BackupSuccessCard({
  success_rate_24h,
  success_rate_7d,
  failed_jobs_24h,
  total_storage_bytes,
}: SuccessRates) {
  const historyTruncated = failed_jobs_24h === undefined

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle>Backup success</CardTitle>
      </CardHeader>

      <CardContent className="space-y-4">
        <div>
          <p className="text-metric font-mono font-semibold tabular-nums tracking-tight">
            {success_rate_24h === undefined ? <UnknownValue /> : `${success_rate_24h}%`}
          </p>
          <p className="mt-1 text-sm text-muted-foreground">
            {success_rate_24h === undefined
              ? historyTruncated
                ? 'Job history was truncated, so the last 24 hours cannot be measured.'
                : 'No backup finished in the last 24 hours.'
              : 'of the backups that finished in the last 24 hours'}
          </p>
        </div>

        <dl className="border-t pt-4 text-sm">
          <div className="flex items-baseline justify-between gap-4">
            <dt className="text-muted-foreground">Last 7 days</dt>
            <dd className="text-right font-mono tabular-nums">
              {success_rate_7d === undefined ? (
                <UnknownValue title="The daemon keeps no 7-day job history to measure." />
              ) : (
                `${success_rate_7d}%`
              )}
            </dd>
          </div>
          {success_rate_7d === undefined && (
            <dd className="mt-1 text-xs text-muted-foreground">
              Needs a persistent job store — in memory, history is pruned first.
            </dd>
          )}
        </dl>

        {/* Last, and quiet: a permanent footnote, not a headline figure. */}
        <dl className="border-t pt-4 text-sm">
          <div className="flex items-baseline justify-between gap-4">
            <dt className="text-muted-foreground">Storage used</dt>
            <dd className="text-right font-mono tabular-nums">
              {total_storage_bytes === undefined ? (
                <UnknownValue kind="unavailable" />
              ) : (
                formatSize(total_storage_bytes)
              )}
            </dd>
          </div>
          {total_storage_bytes === undefined && (
            <dd className="mt-1 text-xs text-muted-foreground">
              The daemon does not total stored bytes — counting them would mean listing every
              backend on each request.
            </dd>
          )}
        </dl>
      </CardContent>
    </Card>
  )
}
