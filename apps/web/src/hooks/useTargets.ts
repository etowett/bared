import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../api/client'

// Fetch all targets
export function useTargets() {
  return useQuery({
    queryKey: ['targets'],
    queryFn: () => apiClient.getTargets(),
    refetchInterval: 5000, // Auto-refresh every 5 seconds
    staleTime: 2000,
  })
}
