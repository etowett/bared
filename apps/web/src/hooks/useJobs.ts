import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '../api/client'
import type { Job } from '../types'

interface UseJobsFilters {
  status?: string
  target?: string
  type?: 'backup' | 'restore'
  page?: number
  limit?: number
}

// Fetch all jobs with optional filters
export function useJobs(filters?: UseJobsFilters) {
  return useQuery({
    queryKey: ['jobs', filters],
    queryFn: () => apiClient.getJobs(filters),
    refetchInterval: 3000, // Auto-refresh every 3 seconds for real-time updates
    staleTime: 1000,
    // Changing a filter or page keeps the current rows on screen while the new
    // ones load, instead of collapsing the table to a skeleton every keystroke.
    placeholderData: keepPreviousData,
  })
}

// Fetch single job by ID
export function useJob(id: string) {
  return useQuery({
    queryKey: ['jobs', id],
    queryFn: () => apiClient.getJob(id),
    refetchInterval: 2000, // Faster refresh for individual job monitoring
    staleTime: 500,
    enabled: !!id, // Only run if ID is provided
  })
}

// Fetch job logs
export function useJobLogs(id: string) {
  return useQuery({
    queryKey: ['jobs', id, 'logs'],
    queryFn: () => apiClient.getJobLogs(id),
    refetchInterval: 5000, // Refresh logs periodically
    staleTime: 1000,
    enabled: !!id,
  })
}

// Trigger backup mutation
export function useTriggerBackup() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (target: string) => apiClient.triggerBackup(target),
    onSuccess: () => {
      // Invalidate and refetch jobs to show the new job
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
      queryClient.invalidateQueries({ queryKey: ['targets'] })
    },
  })
}

// Trigger restore mutation
export function useTriggerRestore() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      target,
      backup_path,
      dry_run,
    }: {
      target: string
      backup_path: string
      dry_run?: boolean
    }) => apiClient.triggerRestore(target, backup_path, dry_run),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
      queryClient.invalidateQueries({ queryKey: ['targets'] })
    },
  })
}

/** Every cached shape that can hold a job, so one helper can rewrite them all. */
type CachedJobs = { jobs: Job[] } | Job | undefined

function withStatus(cached: CachedJobs, id: string, status: Job['status']): CachedJobs {
  if (!cached) return cached
  if ('jobs' in cached) {
    return { ...cached, jobs: cached.jobs.map((j) => (j.id === id ? { ...j, status } : j)) }
  }
  return cached.id === id ? { ...cached, status } : cached
}

// Cancel job mutation
export function useCancelJob() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => apiClient.cancelJob(id),

    // `cancelling` is a real server state, not a spinner: the daemon sets it
    // the moment it accepts the request. Showing it straight away means the
    // row stops looking runnable without waiting up to three seconds for the
    // next poll. If the request is refused, the snapshot goes back.
    onMutate: async (id: string) => {
      await queryClient.cancelQueries({ queryKey: ['jobs'] })
      const previous = queryClient.getQueriesData<CachedJobs>({ queryKey: ['jobs'] })
      queryClient.setQueriesData<CachedJobs>({ queryKey: ['jobs'] }, (cached) =>
        withStatus(cached, id, 'cancelling')
      )
      return { previous }
    },

    onError: (_error, _id, context) => {
      context?.previous.forEach(([key, cached]) => queryClient.setQueryData(key, cached))
    },

    onSettled: (_data, _error, id) => {
      // Invalidate specific job and job list
      queryClient.invalidateQueries({ queryKey: ['jobs', id] })
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
    },
  })
}
