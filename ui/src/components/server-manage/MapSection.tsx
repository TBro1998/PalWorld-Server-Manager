'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Map as MapIcon, UserPlus, RefreshCw, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { serversApi } from '@/lib/api'
import { getApiErrorMessage } from '@/lib/apiError'
import { useTranslations } from '@/contexts/LanguageContext'
import { useRestStatus } from '@/hooks/useRestStatus'
import type { PalPlayer } from '@/types/server'
import { SectionShell, Placeholder, PanelCard, useServerId } from './shared'
import { RestUnavailableNotice } from './RestUnavailableNotice'

// Palworld world coordinate bounds (Unreal Engine units, Palpagos Island).
// These approximate the playable zone; out-of-range points are clamped to the edge.
const MAP_X_MIN = -582000
const MAP_X_MAX = 582000
const MAP_Y_MIN = -582000
const MAP_Y_MAX = 582000

/** Convert world coords to CSS percentage positions (top-left origin). */
function worldToPercent(x: number, y: number) {
  const px = ((x - MAP_X_MIN) / (MAP_X_MAX - MAP_X_MIN)) * 100
  // Screen Y is inverted relative to the game world Y axis.
  const py = (1 - (y - MAP_Y_MIN) / (MAP_Y_MAX - MAP_Y_MIN)) * 100
  return {
    px: Math.max(0, Math.min(100, px)),
    py: Math.max(0, Math.min(100, py)),
  }
}

export function MapSection() {
  const t = useTranslations('serverManage')
  const serverId = useServerId()
  const { status, isAvailable } = useRestStatus(serverId)

  return (
    <SectionShell title={t('map.title')} desc={t('map.desc')} comingSoon={false}>
      <div className="grid gap-4 lg:grid-cols-[1fr_18rem]">
        <PanelCard icon={<MapIcon className="h-4 w-4" />} title={t('map.mapView')}>
          {!isAvailable ? (
            <RestUnavailableNotice status={status} />
          ) : (
            <MapView serverId={serverId} />
          )}
        </PanelCard>

        <PanelCard icon={<UserPlus className="h-4 w-4" />} title={t('map.whitelist')}>
          <WhitelistPanel serverId={serverId} />
        </PanelCard>
      </div>
    </SectionShell>
  )
}

// --- Map view -----------------------------------------------------------------

function MapView({ serverId }: { serverId: number }) {
  const t = useTranslations('serverManage')
  const [hovered, setHovered] = useState<PalPlayer | null>(null)

  const playersQuery = useQuery({
    queryKey: ['rest-players-map', serverId],
    queryFn: async () => (await serversApi.restPlayers(serverId)).data,
    refetchInterval: 10_000,
    refetchOnWindowFocus: false,
  })
  const players = playersQuery.data?.players ?? []

  return (
    <div className="space-y-3">
      <div className="flex justify-end">
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="gap-2 rounded-xl border-2 shadow-pal"
          onClick={() => playersQuery.refetch()}
          disabled={playersQuery.isFetching}
        >
          <RefreshCw className={`h-4 w-4 ${playersQuery.isFetching ? 'animate-spin' : ''}`} />
          {t('map.refresh')}
        </Button>
      </div>

      {/* Map container — square aspect ratio with a subtle grid background */}
      <div
        className="relative overflow-hidden rounded-xl border-2 border-border/60 bg-[#1b2a3b]"
        style={{ aspectRatio: '1 / 1' }}
      >
        {/* Grid SVG overlay */}
        <svg
          className="pointer-events-none absolute inset-0 h-full w-full opacity-[0.12]"
          xmlns="http://www.w3.org/2000/svg"
        >
          <defs>
            <pattern id="pal-grid" width="10%" height="10%" patternUnits="objectBoundingBox">
              <path d="M 100 0 L 0 0 0 100" fill="none" stroke="#4a90c4" strokeWidth="0.8" />
            </pattern>
          </defs>
          <rect width="100%" height="100%" fill="url(#pal-grid)" />
        </svg>

        {/* Map label watermark */}
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <span className="select-none text-[11px] font-bold uppercase tracking-[0.35em] text-white/[0.08]">
            Palpagos Island
          </span>
        </div>

        {/* Player markers */}
        {players.map((p) => {
          const { px, py } = worldToPercent(p.location_x, p.location_y)
          const isHov = hovered?.userId === p.userId
          return (
            <button
              key={p.userId || p.playerId || p.name}
              type="button"
              className="absolute -translate-x-1/2 -translate-y-1/2 focus:outline-none"
              style={{ left: `${px}%`, top: `${py}%` }}
              onMouseEnter={() => setHovered(p)}
              onMouseLeave={() => setHovered(null)}
            >
              {/* Dot with pulse on hover */}
              <span
                className={
                  'block rounded-full border-2 border-white/70 transition-all duration-150 ' +
                  (isHov
                    ? 'h-4 w-4 bg-primary shadow-[0_0_8px_2px_hsl(var(--primary)/0.6)]'
                    : 'h-3 w-3 bg-emerald-400')
                }
              />
              {/* Hover tooltip */}
              {isHov && (
                <div className="pointer-events-none absolute bottom-full left-1/2 z-20 mb-2 -translate-x-1/2 whitespace-nowrap rounded-lg border bg-popover px-2.5 py-1.5 text-xs shadow-lg">
                  <p className="font-semibold text-foreground">{p.name}</p>
                  <p className="text-muted-foreground">Lv.{p.level}</p>
                  <p className="font-mono text-muted-foreground">
                    {Math.round(p.location_x)}, {Math.round(p.location_y)}
                  </p>
                </div>
              )}
            </button>
          )
        })}

        {/* Empty-state overlay */}
        {players.length === 0 && !playersQuery.isFetching && (
          <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
            <p className="text-sm text-white/30">{t('map.noPlayers')}</p>
          </div>
        )}
      </div>

      {/* Player legend beneath the map */}
      {players.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {players.map((p) => (
            <Badge
              key={p.userId || p.name}
              variant="secondary"
              className="cursor-default gap-1.5"
              onMouseEnter={() => setHovered(p)}
              onMouseLeave={() => setHovered(null)}
            >
              <span className="inline-block h-2 w-2 rounded-full bg-emerald-400" />
              {p.name}
            </Badge>
          ))}
        </div>
      )}
    </div>
  )
}

// --- Whitelist panel ----------------------------------------------------------

function WhitelistPanel({ serverId }: { serverId: number }) {
  const t = useTranslations('serverManage')
  const queryClient = useQueryClient()
  const [newUid, setNewUid] = useState('')
  const [feedback, setFeedback] = useState<{ kind: 'success' | 'error'; text: string } | null>(null)

  const wlQuery = useQuery({
    queryKey: ['whitelist', serverId],
    queryFn: async () => (await serversApi.getWhitelist(serverId)).data.entries,
    enabled: Number.isFinite(serverId),
    refetchOnWindowFocus: false,
  })
  const entries = wlQuery.data ?? []

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['whitelist', serverId] })

  const addMut = useMutation({
    mutationFn: (uid: string) => serversApi.addWhitelist(serverId, uid),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('map.wl.addOk') })
      setNewUid('')
      invalidate()
    },
    onError: (err) =>
      setFeedback({ kind: 'error', text: getApiErrorMessage(err, t('map.wl.addFail')) }),
  })

  const removeMut = useMutation({
    mutationFn: (uid: string) => serversApi.removeWhitelist(serverId, uid),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('map.wl.removeOk') })
      invalidate()
    },
    onError: (err) =>
      setFeedback({ kind: 'error', text: getApiErrorMessage(err, t('map.wl.removeFail')) }),
  })

  const handleAdd = () => {
    const uid = newUid.trim()
    if (!uid) return
    setFeedback(null)
    addMut.mutate(uid)
  }

  return (
    <div className="space-y-3">
      <p className="text-xs text-muted-foreground">{t('map.wl.hint')}</p>

      {/* Add entry */}
      <div className="flex gap-2">
        <Input
          value={newUid}
          onChange={(e) => setNewUid(e.target.value)}
          placeholder={t('map.wl.placeholder')}
          onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
          className="font-mono text-xs"
        />
        <Button
          type="button"
          size="sm"
          onClick={handleAdd}
          disabled={!newUid.trim() || addMut.isPending}
          className="shrink-0"
        >
          <UserPlus className="h-4 w-4" />
        </Button>
      </div>

      {feedback && (
        <p
          className={
            'text-xs ' +
            (feedback.kind === 'success' ? 'text-success' : 'text-destructive')
          }
        >
          {feedback.text}
        </p>
      )}

      {/* Entry list */}
      {wlQuery.isLoading ? (
        <p className="text-xs text-muted-foreground">{t('map.wl.loading')}</p>
      ) : entries.length === 0 ? (
        <Placeholder className="min-h-[100px] text-xs">{t('map.wl.empty')}</Placeholder>
      ) : (
        <ul className="max-h-[380px] space-y-1 overflow-y-auto">
          {entries.map((uid) => (
            <li
              key={uid}
              className="flex items-center gap-2 rounded-lg border px-3 py-1.5"
            >
              <span className="flex-1 truncate font-mono text-xs text-foreground">{uid}</span>
              <button
                type="button"
                className="shrink-0 text-muted-foreground hover:text-destructive"
                onClick={() => {
                  setFeedback(null)
                  removeMut.mutate(uid)
                }}
                disabled={removeMut.isPending}
                aria-label={t('map.wl.remove')}
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
