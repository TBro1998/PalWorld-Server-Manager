'use client'

import { Gauge, Timer, Signal, Clock, Server as ServerIcon, RefreshCw } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { serversApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'
import { useRestStatus } from '@/hooks/useRestStatus'
import { SectionShell, PanelCard, Placeholder, useServerId } from './shared'
import { RestUnavailableNotice } from './RestUnavailableNotice'

// Server info + runtime metrics, driven by the game server's REST API (/info,
// /metrics). The online player list lives in the Players section instead. Reads
// do NOT auto-poll; they fetch once when the section becomes available and
// afterwards refresh only when the user clicks the "Refresh" button.
export function OverviewSection() {
  const t = useTranslations('serverManage')
  const serverId = useServerId()
  const { status, isAvailable } = useRestStatus(serverId)

  const infoQuery = useQuery({
    queryKey: ['rest-info', serverId],
    queryFn: async () => (await serversApi.restInfo(serverId)).data,
    enabled: isAvailable,
    refetchOnWindowFocus: false,
  })

  const metricsQuery = useQuery({
    queryKey: ['rest-metrics', serverId],
    queryFn: async () => (await serversApi.restMetrics(serverId)).data,
    enabled: isAvailable,
    refetchOnWindowFocus: false,
  })

  const info = infoQuery.data
  const metrics = metricsQuery.data

  // Manual refresh: pull both read queries at once. Disabled while the REST API
  // is unavailable or while either query is already fetching.
  const isFetching = infoQuery.isFetching || metricsQuery.isFetching
  const handleRefresh = () => {
    infoQuery.refetch()
    metricsQuery.refetch()
  }

  // Metric tiles: filled from /metrics when available, "—" placeholder otherwise.
  const dash = '—'
  const tiles = [
    {
      icon: Signal,
      label: t('overview.online'),
      value: metrics ? `${metrics.currentplayernum}/${metrics.maxplayernum}` : dash,
    },
    {
      icon: Gauge,
      label: t('overview.fps'),
      value: metrics ? String(metrics.serverfps) : dash,
    },
    {
      icon: Clock,
      label: t('overview.uptime'),
      value: metrics ? formatUptime(metrics.uptime) : dash,
    },
    {
      icon: Timer,
      label: t('overview.frametime'),
      value: metrics ? `${metrics.serverframetime.toFixed(1)} ms` : dash,
    },
  ]

  return (
    <SectionShell title={t('overview.title')} desc={t('overview.desc')} comingSoon={false}>
      {!isAvailable && <RestUnavailableNotice status={status} />}

      {/* Manual refresh: overview REST data does not auto-poll. */}
      <div className="flex justify-end">
        <Button
          variant="outline"
          size="sm"
          className="gap-2 rounded-xl border-2 shadow-pal"
          onClick={handleRefresh}
          disabled={!isAvailable || isFetching}
        >
          <RefreshCw className={`h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} />
          {t('overview.refresh')}
        </Button>
      </div>

      {/* Metric tiles */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {tiles.map((m) => {
          const Icon = m.icon
          return (
            <Card key={m.label} className="rounded-2xl border-2 shadow-pal">
              <CardContent className="flex items-center gap-3 p-4">
                <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <Icon className="h-5 w-5" />
                </span>
                <div className="min-w-0">
                  <div className="truncate text-2xl font-extrabold leading-none text-foreground">
                    {m.value}
                  </div>
                  <div className="mt-1 text-xs font-medium text-muted-foreground">{m.label}</div>
                </div>
              </CardContent>
            </Card>
          )
        })}
      </div>

      <PanelCard icon={<ServerIcon className="h-4 w-4" />} title={t('overview.info')}>
        {info ? (
          <dl className="space-y-2 text-sm">
            <InfoRow label={t('overview.serverName')} value={info.servername} />
            <InfoRow label={t('overview.version')} value={info.version} />
            <InfoRow label={t('overview.description')} value={info.description} />
          </dl>
        ) : (
          <Placeholder className="min-h-[140px]">{t('rest.noData')}</Placeholder>
        )}
      </PanelCard>
    </SectionShell>
  )
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <dt className="shrink-0 text-muted-foreground">{label}</dt>
      <dd className="min-w-0 break-words text-right font-medium text-foreground">
        {value || '—'}
      </dd>
    </div>
  )
}

// formatUptime turns a real-time uptime in seconds into a compact d/h/m/s label.
// The units are locale-neutral, avoiding interpolation the i18n helper lacks.
function formatUptime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}
