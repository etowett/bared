import { validateJobSearch } from '@/lib/job-search'
import { createFileRoute } from '@tanstack/react-router'

// The job history on this page filters and pages server-side, and its state
// lives in the URL. `validateSearch` runs before the lazy component resolves,
// so it has to be declared here rather than in `index.lazy.tsx`.
export const Route = createFileRoute('/backup/')({
  validateSearch: validateJobSearch,
})
