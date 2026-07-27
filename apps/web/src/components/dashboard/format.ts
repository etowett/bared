import { formatBytes, formatDuration } from '@/lib/utils'

/**
 * `formatBytes`/`formatDuration` in `lib/utils` guard with a falsy check, so
 * they answer "N/A" for a real, measured zero as readily as for an absent
 * field. The Overview draws that distinction everywhere else, so it cannot
 * borrow the conflation here: these two take the value only once the caller has
 * established it is a number, and render zero as zero.
 */
export function formatSize(bytes: number): string {
  return bytes === 0 ? '0 B' : formatBytes(bytes)
}

export function formatRuntime(seconds: number): string {
  return seconds === 0 ? '0s' : formatDuration(seconds)
}
