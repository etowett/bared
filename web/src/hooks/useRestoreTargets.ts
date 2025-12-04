import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../api/client'

// Fetch all restore targets
export function useRestoreTargets() {
  return useQuery({
    queryKey: ['restore-targets'],
    queryFn: () => apiClient.getRestoreTargets(),
    staleTime: 30000, // Cache for 30 seconds
  })
}
