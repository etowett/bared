import { createFileRoute } from '@tanstack/react-router'

export type JobsSearch = {
  page?: number
  limit?: number
  status?: string
  type?: string
  target?: string
}

// `validateSearch` runs before the route's component is resolved, so it has to
// stay in the eager route file. The page itself lives in `index.lazy.tsx` and
// is fetched on demand.
export const Route = createFileRoute('/jobs/')({
  validateSearch: (search: Record<string, unknown>): JobsSearch => ({
    page: Number(search.page) || 1,
    limit: Number(search.limit) || 20,
    status: search.status === 'all' ? undefined : (search.status as string) || undefined,
    type: search.type === 'all' ? undefined : (search.type as string) || undefined,
    target: (search.target as string) || undefined,
  }),
})
