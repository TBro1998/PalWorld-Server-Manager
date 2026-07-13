'use client'

import { Suspense, useState } from 'react'
import Link from 'next/link'
import { useSearchParams } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import {
  ArrowLeft,
  LayoutDashboard,
  Users,
  Wrench,
  Map as MapIcon,
  Webhook,
  Archive,
  Settings,
  Server as ServerIcon,
} from 'lucide-react'
import { serversApi } from '@/lib/api'
import type { Server as ServerType } from '@/types/server'
import { useTranslations } from '@/contexts/LanguageContext'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { ServerSettingsDialog } from '@/components/ServerSettingsDialog'
import { OverviewSection } from '@/components/server-manage/OverviewSection'
import { PlayersSection } from '@/components/server-manage/PlayersSection'
import { OperationsSection } from '@/components/server-manage/OperationsSection'
import { MapSection } from '@/components/server-manage/MapSection'
import { RestApiSection } from '@/components/server-manage/RestApiSection'
import { BackupSection } from '@/components/server-manage/BackupSection'
import { SettingsSection } from '@/components/server-manage/SettingsSection'

// Left sub-nav definition. `key` maps into the serverManage.sections.* i18n keys
// and drives the active-section switch below.
const SECTIONS = [
  { key: 'overview', icon: LayoutDashboard },
  { key: 'players', icon: Users },
  { key: 'operations', icon: Wrench },
  { key: 'map', icon: MapIcon },
  { key: 'restapi', icon: Webhook },
  { key: 'backup', icon: Archive },
  { key: 'settings', icon: Settings },
] as const

type SectionKey = (typeof SECTIONS)[number]['key']

const statusBadge: Record<
  ServerType['status'],
  { variant: 'success' | 'secondary' | 'info' | 'destructive'; key: string }
> = {
  running: { variant: 'success', key: 'statusRunning' },
  stopped: { variant: 'secondary', key: 'statusStopped' },
  installing: { variant: 'info', key: 'statusInstalling' },
  error: { variant: 'destructive', key: 'statusError' },
}

// The panel reads ?id= at runtime; static export requires useSearchParams to be
// wrapped in a Suspense boundary, so the real panel lives in ManagePanel.
export default function ServerManagePage() {
  return (
    <Suspense fallback={<div className="p-10 text-muted-foreground">…</div>}>
      <ManagePanel />
    </Suspense>
  )
}

function ManagePanel() {
  const t = useTranslations('serverManage')
  const ts = useTranslations('servers')
  const searchParams = useSearchParams()
  const rawId = searchParams.get('id')
  const serverId = rawId ? Number(rawId) : NaN

  const [active, setActive] = useState<SectionKey>('overview')
  const [settingsOpen, setSettingsOpen] = useState(false)

  const { data: server, isLoading } = useQuery({
    queryKey: ['server', serverId],
    queryFn: async () => (await serversApi.get(serverId)).data,
    enabled: Number.isFinite(serverId),
    refetchInterval: 5000,
  })

  // Guard: missing / invalid id, or a fetch that resolved to nothing.
  if (!Number.isFinite(serverId) || (!isLoading && !server)) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-10">
        <BackLink label={t('backToList')} />
        <Card className="mt-6 rounded-2xl border-2 border-dashed shadow-none">
          <CardContent className="flex flex-col items-center gap-3 py-16 text-center">
            <div className="flex h-16 w-16 items-center justify-center rounded-3xl bg-muted text-4xl">
              🐾
            </div>
            <p className="text-muted-foreground">{t('notFound')}</p>
          </CardContent>
        </Card>
      </div>
    )
  }

  const badge = server ? statusBadge[server.status] : null

  return (
    <div className="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-10">
      {/* Header: back + server identity */}
      <BackLink label={t('backToList')} />
      <div className="mt-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary">
            <ServerIcon className="h-6 w-6" />
          </div>
          <div className="min-w-0">
            <h1 className="truncate text-2xl font-extrabold tracking-tight text-foreground sm:text-3xl">
              {server?.name ?? '…'}
            </h1>
            <p
              className="truncate font-mono text-xs text-muted-foreground"
              title={server?.install_path}
            >
              {server?.install_path ?? ''}
            </p>
          </div>
        </div>
        {badge && (
          <Badge variant={badge.variant} className="shrink-0 self-start sm:self-auto">
            {ts(badge.key)}
          </Badge>
        )}
      </div>

      {/* Body: left sub-nav rail + section content */}
      <div className="mt-8 grid gap-6 lg:grid-cols-[13rem_1fr]">
        <aside className="lg:sticky lg:top-8 lg:self-start">
          <nav className="flex gap-1.5 overflow-x-auto lg:flex-col lg:overflow-visible">
            {SECTIONS.map(({ key, icon: Icon }) => {
              const isActive = active === key
              return (
                <button
                  key={key}
                  type="button"
                  onClick={() => setActive(key)}
                  className={
                    'flex shrink-0 items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-sm font-semibold transition-all ' +
                    (isActive
                      ? 'bg-primary text-primary-foreground shadow-pal'
                      : 'text-muted-foreground hover:bg-secondary hover:text-foreground')
                  }
                >
                  <Icon className="h-4 w-4 shrink-0" />
                  {t(`sections.${key}`)}
                </button>
              )
            })}
          </nav>
        </aside>

        <section className="min-w-0">
          {active === 'overview' && <OverviewSection />}
          {active === 'players' && <PlayersSection />}
          {active === 'operations' && <OperationsSection />}
          {active === 'map' && <MapSection />}
          {active === 'restapi' && <RestApiSection />}
          {active === 'backup' && <BackupSection />}
          {active === 'settings' && <SettingsSection onOpen={() => setSettingsOpen(true)} />}
        </section>
      </div>

      {/* Reused, still-functional server config editor. The panel now owns the
          entry that previously lived on the server card. */}
      <ServerSettingsDialog
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        server={server ?? null}
      />
    </div>
  )
}

function BackLink({ label }: { label: string }) {
  return (
    <Link
      href="/servers"
      prefetch={false}
      className="inline-flex items-center gap-1 text-sm font-semibold text-primary hover:underline"
    >
      <ArrowLeft className="h-4 w-4" />
      {label}
    </Link>
  )
}
