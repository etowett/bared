import type { SortDirection, TableDensity } from '@/components/ui/table'

/**
 * The columns the job tables can order by.
 *
 * `/api/jobs` has no sort parameter — it always returns `created_at DESC` — so
 * this only ever reorders the page already fetched (#141). It still belongs in
 * the URL: a link to a sorted view has to survive a reload like every other
 * control on the page.
 */
export type JobSortKey = 'target' | 'status' | 'created_at' | 'duration'

/**
 * Everything the job tables keep in the URL.
 *
 * Every field is optional even though `validateJobSearch` always fills `page`
 * and `limit`: TanStack Router derives "what a `<Link to="/backup">` must
 * supply" from this type, and a required field would force every link in the
 * app — including breadcrumbs — to spell out a page number.
 */
export interface JobSearch {
  page?: number
  limit?: number
  status?: string
  type?: string
  target?: string
  sort?: JobSortKey
  order?: SortDirection
  density?: TableDensity
}

export const JOB_STATUSES = [
  'queued',
  'running',
  'completed',
  'failed',
  'cancelling',
  'cancelled',
] as const

const SORT_KEYS: readonly string[] = ['target', 'status', 'created_at', 'duration']
const JOB_TYPES: readonly string[] = ['backup', 'restore']

/** Page sizes offered by the tables. Also the whitelist for the URL. */
export const PAGE_SIZES = [10, 20, 50, 100] as const

function oneOf<T extends string>(value: unknown, allowed: readonly string[]): T | undefined {
  return typeof value === 'string' && allowed.includes(value) ? (value as T) : undefined
}

/**
 * Turns raw URL search params into the table's state.
 *
 * Every value is whitelisted rather than cast: a hand-edited `?status=foo`
 * would otherwise be forwarded to the API, which answers a bad filter with an
 * empty list and no error — an empty table that looks like "no jobs" rather
 * than "bad link". Unknown values fall back to "no filter".
 *
 * This runs before the route's component is resolved, so it has to live in an
 * eager route module. Each `routes/**\/*.tsx` sibling of a job table calls it.
 */
export function validateJobSearch(search: Record<string, unknown>): JobSearch {
  const page = Number(search.page)
  const limit = Number(search.limit)

  return {
    page: Number.isInteger(page) && page > 0 ? page : 1,
    limit: PAGE_SIZES.includes(limit as (typeof PAGE_SIZES)[number]) ? limit : 20,
    status: oneOf(search.status, JOB_STATUSES),
    type: oneOf(search.type, JOB_TYPES),
    target: typeof search.target === 'string' && search.target ? search.target : undefined,
    sort: oneOf<JobSortKey>(search.sort, SORT_KEYS),
    order: oneOf<SortDirection>(search.order, ['asc', 'desc']),
    density: oneOf<TableDensity>(search.density, ['comfortable', 'compact']),
  }
}
