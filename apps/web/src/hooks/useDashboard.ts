import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../api/client'

// Fetch dashboard summary
export function useDashboard() {
  return useQuery({
    queryKey: ['dashboard'],
    queryFn: () => apiClient.getDashboard(),
    refetchInterval: 5000, // Auto-refresh every 5 seconds
    staleTime: 2000,
  })
}
