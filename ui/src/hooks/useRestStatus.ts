'use client'

import { useQuery } from '@tanstack/react-query'
import { serversApi } from '@/lib/api'
import type { RestStatus } from '@/types/server'

// Shared availability probe for the Palworld REST API. Overview / Players /
// Operations all depend on the same status, so they consume this single hook to
// decide whether to render live data or the RestUnavailableNotice guidance.
//
// It does NOT auto-poll: the backend's /rest/status implicitly hits the game
// server's /v1/api/info as its connectivity check, so a periodic poll would keep
// that endpoint under constant load. Instead it probes once when a section
// mounts (each tab switch remounts its section) and afterwards only when the
// caller invokes refetch() — typically wired to the section's Refresh button.
// The query is only enabled for a valid numeric id. isAvailable collapses the
// three gating flags into the one boolean sections branch on.
export function useRestStatus(serverId: number) {
  const query = useQuery({
    queryKey: ['rest-status', serverId],
    queryFn: async () => (await serversApi.restStatus(serverId)).data,
    enabled: Number.isFinite(serverId),
    refetchOnWindowFocus: false,
  })

  const status: RestStatus | undefined = query.data
  const isAvailable = Boolean(status?.enabled && status?.running && status?.reachable)

  return { status, isAvailable, isLoading: query.isLoading, refetch: query.refetch }
}
