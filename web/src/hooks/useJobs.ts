import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '../api/client'

interface UseJobsFilters {
  status?: string
  target?: string
}

// Fetch all jobs with optional filters
export function useJobs(filters?: UseJobsFilters) {
  return useQuery({
    queryKey: ['jobs', filters],
    queryFn: () => apiClient.getJobs(filters),
    refetchInterval: 3000, // Auto-refresh every 3 seconds for real-time updates
    staleTime: 1000,
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

// Cancel job mutation
export function useCancelJob() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => apiClient.cancelJob(id),
    onSuccess: (_, id) => {
      // Invalidate specific job and job list
      queryClient.invalidateQueries({ queryKey: ['jobs', id] })
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
    },
  })
}
