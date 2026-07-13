'use client'

import { useQuery } from '@tanstack/react-query'
import { serversApi } from '@/lib/api'
import type { RestStatus } from '@/types/server'

// Shared availability probe for the Palworld REST API. Overview / Players /
// Operations all depend on the same status, so they consume this single hook to
// decide whether to render live data or the RestUnavailableNotice guidance.
//
// It polls /rest/status every 5s (matching the panel's existing cadence). The
// query is only enabled for a valid numeric id. isAvailable collapses the three
// gating flags into the one boolean sections branch on.
export function useRestStatus(serverId: number) {
  const query = useQuery({
    queryKey: ['rest-status', serverId],
    queryFn: async () => (await serversApi.restStatus(serverId)).data,
    enabled: Number.isFinite(serverId),
    refetchInterval: 5000,
  })

  const status: RestStatus | undefined = query.data
  const isAvailable = Boolean(status?.enabled && status?.running && status?.reachable)

  return { status, isAvailable, isLoading: query.isLoading }
}
