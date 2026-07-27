import { validateJobSearch, type JobSearch } from '@/lib/job-search'
import { createFileRoute } from '@tanstack/react-router'

export type JobsSearch = JobSearch

// `validateSearch` runs before the route's component is resolved, so it has to
// stay in the eager route file. The page itself lives in `index.lazy.tsx` and
// is fetched on demand.
export const Route = createFileRoute('/jobs/')({
  validateSearch: validateJobSearch,
})
