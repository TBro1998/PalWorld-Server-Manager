'use client'

import { useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { serversApi } from '@/lib/api'
import type { RestStatus, Server } from '@/types/server'

// Shared availability probe for the Palworld REST API. Overview / Players /
// Operations all depend on the same status, so they consume this single hook to
// decide whether to render live data or the RestUnavailableNotice guidance.
//
// Steady-state goal: once the REST API is reachable it must NOT be re-probed on
// a timer, because the backend's /rest/status hits the game server's
// /v1/api/info as its connectivity check and a periodic poll would keep that
// endpoint under constant load. Section Refresh buttons pull only their own
// live data (/metrics, /players), never this probe.
//
// But a once-only probe (the previous implementation) got stuck whenever the
// lifecycle changed after page entry: entering while the server was stopped
// cached `not_running` forever, so starting the server never flipped the
// sections out of "REST API unavailable". To fix that without reintroducing
// steady-state /v1/api/info load, the probe is driven by the server's running
// status (reused from the ['server', id] query the manage page already polls):
//   - on a running <-> stopped transition, re-probe once immediately;
//   - while running but not yet reachable (post-start warm-up / transient
//     unreachable), poll every 3s UNTIL reachable, then stop. Config problems
//     (restapi_disabled / admin_password_empty) do not self-resolve, so they
//     are not polled.
// The query is only enabled for a valid numeric id. isAvailable collapses the
// three gating flags into the one boolean sections branch on.
export function useRestStatus(serverId: number) {
  // Reuse the server record the manage page already polls (refetchInterval 5s);
  // React Query dedupes by key, so this adds no extra request.
  const { data: server } = useQuery<Server>({
    queryKey: ['server', serverId],
    queryFn: async () => (await serversApi.get(serverId)).data,
    enabled: Number.isFinite(serverId),
    refetchInterval: 5000,
  })
  const running = server?.status === 'running'

  const query = useQuery({
    queryKey: ['rest-status', serverId],
    queryFn: async () => (await serversApi.restStatus(serverId)).data,
    enabled: Number.isFinite(serverId),
    refetchOnWindowFocus: false,
    staleTime: Infinity,
    // Poll only during the running-but-not-yet-reachable window; stop once the
    // API answers or when the blocking reason is a config problem that won't
    // self-resolve. A stopped server needs no poll (not_running is stable).
    refetchInterval: (q) => {
      const data = q.state.data as RestStatus | undefined
      if (!running) return false
      if (data?.reachable) return false
      if (data?.reason === 'restapi_disabled' || data?.reason === 'admin_password_empty') {
        return false
      }
      return 3000
    },
  })

  // Re-probe immediately when the server's lifecycle flips, so a status captured
  // while the server was in the other state (e.g. `not_running` from before a
  // start, or a stale `reachable` from before a stop) does not stick.
  const { refetch } = query
  const prevRunning = useRef<boolean | undefined>(undefined)
  useEffect(() => {
    if (!Number.isFinite(serverId)) return
    if (prevRunning.current !== undefined && prevRunning.current !== running) {
      refetch()
    }
    prevRunning.current = running
  }, [running, serverId, refetch])

  const status: RestStatus | undefined = query.data
  const isAvailable = Boolean(status?.enabled && status?.running && status?.reachable)

  return {
    status,
    isAvailable,
    isLoading: query.isLoading,
    isFetching: query.isFetching,
    refetch: query.refetch,
  }
}
