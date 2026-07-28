import { validateJobSearch } from '@/lib/job-search'
import { createFileRoute } from '@tanstack/react-router'

// See `backup/index.tsx` — search validation cannot live in a lazy module.
export const Route = createFileRoute('/restore/jobs')({
  validateSearch: validateJobSearch,
})
