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
  ScrollText,
  SlidersHorizontal,
  Gamepad2,
  Terminal,
  Package,
  FileCode,
  ShieldCheck,
  Database,
  Archive,
  Server as ServerIcon,
} from 'lucide-react'
import { serversApi } from '@/lib/api'
import type { Server as ServerType } from '@/types/server'
import { useTranslations } from '@/contexts/LanguageContext'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { OverviewSection } from '@/components/server-manage/OverviewSection'
import { PlayersSection } from '@/components/server-manage/PlayersSection'
import { OperationsSection } from '@/components/server-manage/OperationsSection'
import { LogsSection } from '@/components/server-manage/LogsSection'
import { WhitelistSection } from '@/components/server-manage/WhitelistSection'
import { BackupSection } from '@/components/server-manage/BackupSection'
import { ModsSection } from '@/components/server-manage/ModsSection'
import { SaveDataSection } from '@/components/server-manage/SaveDataSection'
import { SettingsDraftProvider } from '@/components/server-manage/SettingsDraftContext'
import { SettingsSaveBar } from '@/components/server-manage/SettingsSaveBar'
import { BasicsSettings } from '@/components/server-manage/settings/BasicsSettings'
import { GameSettings } from '@/components/server-manage/settings/GameSettings'
import { LaunchSettings } from '@/components/server-manage/settings/LaunchSettings'
import { RawSettings } from '@/components/server-manage/settings/RawSettings'

// Grouped left sub-nav. `key` maps into serverManage.sections.* and drives the
// active-section switch below; `group` maps into serverManage.groups.*.
const NAV_GROUPS = [
  {
    group: 'monitor',
    items: [
      { key: 'overview', icon: LayoutDashboard },
      { key: 'players', icon: Users },
      { key: 'saveData', icon: Database },
      { key: 'operations', icon: Wrench },
      { key: 'logs', icon: ScrollText },
    ],
  },
  {
    group: 'config',
    items: [
      { key: 'basics', icon: SlidersHorizontal },
      { key: 'game', icon: Gamepad2 },
      { key: 'launch', icon: Terminal },
      { key: 'mods', icon: Package },
      { key: 'raw', icon: FileCode },
    ],
  },
  {
    group: 'more',
    items: [
      { key: 'map', icon: ShieldCheck },
      { key: 'backup', icon: Archive },
    ],
  },
] as const

type SectionKey = (typeof NAV_GROUPS)[number]['items'][number]['key']

const ALL_KEYS = NAV_GROUPS.flatMap((g) => g.items.map((i) => i.key)) as SectionKey[]

const statusBadge: Record<
  ServerType['status'],
  { variant: 'success' | 'secondary' | 'info' | 'destructive'; key: string }
> = {
  running: { variant: 'success', key: 'statusRunning' },
  stopped: { variant: 'secondary', key: 'statusStopped' },
  installing: { variant: 'info', key: 'statusInstalling' },
  error: { variant: 'destructive', key: 'statusError' },
}

// The panel reads ?id= / ?section= at runtime; static export requires
// useSearchParams to be wrapped in a Suspense boundary.
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

  // Optional deep-link into a specific section (e.g. from the list-page logs
  // shortcut). Falls back to overview when absent/invalid.
  const rawSection = searchParams.get('section')
  const initialSection: SectionKey =
    rawSection && (ALL_KEYS as string[]).includes(rawSection)
      ? (rawSection as SectionKey)
      : 'overview'
  const [active, setActive] = useState<SectionKey>(initialSection)

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

      {/* Body: left grouped sub-nav rail + section content. The settings draft
          provider wraps both so the shared save bar and config pages align. */}
      <SettingsDraftProvider server={server ?? null}>
        <div className="mt-8 grid gap-6 lg:grid-cols-[13rem_1fr]">
          <aside className="lg:sticky lg:top-8 lg:self-start">
            <nav className="flex flex-col gap-3">
              {NAV_GROUPS.map(({ group, items }) => (
                <div key={group} className="flex flex-col gap-1">
                  <span className="px-3.5 pb-0.5 text-[11px] font-bold uppercase tracking-wider text-muted-foreground/70">
                    {t(`groups.${group}`)}
                  </span>
                  {items.map(({ key, icon: Icon }) => {
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
                </div>
              ))}
            </nav>
          </aside>

          <section className="min-w-0">
            {active === 'overview' && <OverviewSection />}
            {active === 'players' && <PlayersSection />}
            {active === 'saveData' && <SaveDataSection />}
            {active === 'operations' && <OperationsSection />}
            {active === 'logs' && <LogsSection />}
            {active === 'basics' && <BasicsSettings />}
            {active === 'game' && <GameSettings />}
            {active === 'launch' && <LaunchSettings />}
            {active === 'mods' && <ModsSection />}
            {active === 'raw' && <RawSettings />}
            {active === 'map' && <WhitelistSection />}
            {active === 'backup' && <BackupSection />}

            {/* Shared sticky save bar: visible on any config page whenever the
                draft is dirty; stays across page switches. */}
            <SettingsSaveBar />
          </section>
        </div>
      </SettingsDraftProvider>
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
