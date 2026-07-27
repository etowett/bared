import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../api/client'

/**
 * Is the daemon answering?
 *
 * `/api/health` is unauthenticated and cheap, so this is a true reachability
 * probe rather than a proxy for one particular page's data. The header uses it
 * to tell an operator that the dashboard has gone stale because the process is
 * down — not because nothing happened to be running.
 */
export function useDaemonStatus() {
  const query = useQuery({
    queryKey: ['health'],
    queryFn: () => apiClient.health(),
    refetchInterval: 15000,
    // A failed probe *is* the answer; retrying just delays showing it.
    retry: false,
    staleTime: 10000,
  })

  return {
    ...query,
    reachable: query.isSuccess,
    checking: query.isPending || query.isFetching,
    version: query.data?.version,
  }
}
