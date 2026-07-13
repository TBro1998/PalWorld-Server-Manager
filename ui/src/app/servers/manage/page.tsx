'use client'

import React, { Suspense, useState } from 'react'
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
  Cpu,
  MemoryStick,
  Clock,
  Signal,
  Megaphone,
  UserX,
  Ban,
  Power,
  ListChecks,
  RefreshCw,
  Construction,
  ExternalLink,
} from 'lucide-react'
import { serversApi } from '@/lib/api'
import type { Server as ServerType } from '@/types/server'
import { useTranslations } from '@/contexts/LanguageContext'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { ServerSettingsDialog } from '@/components/ServerSettingsDialog'

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
          {active === 'settings' && (
            <SettingsSection onOpen={() => setSettingsOpen(true)} />
          )}
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

// ── Shared layout primitives ─────────────────────────────────────────────────

// Section wrapper: title + description + a "coming soon" chip so every reserved
// area reads consistently while the features are stubbed.
function SectionShell({
  title,
  desc,
  children,
}: {
  title: string
  desc: string
  children: React.ReactNode
}) {
  const t = useTranslations('serverManage')
  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-xl font-bold text-foreground">{title}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{desc}</p>
        </div>
        <Badge variant="info" className="gap-1.5">
          <Construction className="h-3.5 w-3.5" />
          {t('comingSoon')}
        </Badge>
      </div>
      {children}
    </div>
  )
}

// A dashed placeholder region standing in for not-yet-built content.
function Placeholder({
  className = '',
  children,
}: {
  className?: string
  children?: React.ReactNode
}) {
  return (
    <div
      className={
        'flex items-center justify-center rounded-2xl border-2 border-dashed border-border/70 bg-muted/30 p-6 text-center text-sm text-muted-foreground ' +
        className
      }
    >
      {children}
    </div>
  )
}

// A titled card used to frame a reserved sub-area within a section.
function PanelCard({
  icon,
  title,
  children,
}: {
  icon: React.ReactNode
  title: string
  children: React.ReactNode
}) {
  return (
    <Card className="rounded-2xl border-2 shadow-pal">
      <CardContent className="space-y-3 p-5">
        <div className="flex items-center gap-2 text-foreground">
          <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
            {icon}
          </span>
          <h3 className="font-bold">{title}</h3>
        </div>
        {children}
      </CardContent>
    </Card>
  )
}

// ── Sections (layout only — features reserved) ───────────────────────────────

function OverviewSection() {
  const t = useTranslations('serverManage')
  const metrics = [
    { icon: Cpu, label: t('overview.cpu') },
    { icon: MemoryStick, label: t('overview.memory') },
    { icon: Signal, label: t('overview.online') },
    { icon: Clock, label: t('overview.uptime') },
  ]
  return (
    <SectionShell title={t('overview.title')} desc={t('overview.desc')}>
      {/* Metric tiles */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {metrics.map((m) => {
          const Icon = m.icon
          return (
            <Card key={m.label} className="rounded-2xl border-2 shadow-pal">
              <CardContent className="flex items-center gap-3 p-4">
                <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <Icon className="h-5 w-5" />
                </span>
                <div>
                  <div className="text-2xl font-extrabold leading-none text-foreground">
                    —
                  </div>
                  <div className="mt-1 text-xs font-medium text-muted-foreground">
                    {m.label}
                  </div>
                </div>
              </CardContent>
            </Card>
          )
        })}
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <PanelCard icon={<ServerIcon className="h-4 w-4" />} title={t('overview.info')}>
          <Placeholder className="min-h-[140px]">{t('comingSoonDesc')}</Placeholder>
        </PanelCard>
        <PanelCard icon={<Users className="h-4 w-4" />} title={t('overview.onlinePlayers')}>
          <Placeholder className="min-h-[140px]">{t('comingSoonDesc')}</Placeholder>
        </PanelCard>
      </div>
    </SectionShell>
  )
}

function PlayersSection() {
  const t = useTranslations('serverManage')
  const tabs = ['players', 'guilds', 'pals', 'inventory'] as const
  return (
    <SectionShell title={t('players.title')} desc={t('players.desc')}>
      {/* Reserved inner tab bar for the four data domains. */}
      <div className="flex flex-wrap gap-1.5">
        {tabs.map((tb, i) => (
          <span
            key={tb}
            className={
              'rounded-full px-3.5 py-1.5 text-sm font-semibold ' +
              (i === 0
                ? 'bg-primary text-primary-foreground'
                : 'bg-secondary text-muted-foreground')
            }
          >
            {t(`players.tabs.${tb}`)}
          </span>
        ))}
      </div>
      <Placeholder className="min-h-[280px]">{t('comingSoonDesc')}</Placeholder>
    </SectionShell>
  )
}

function OperationsSection() {
  const t = useTranslations('serverManage')
  const ops = [
    { icon: UserX, key: 'kick' },
    { icon: Ban, key: 'ban' },
    { icon: Megaphone, key: 'broadcast' },
    { icon: Power, key: 'shutdown' },
  ] as const
  return (
    <SectionShell title={t('operations.title')} desc={t('operations.desc')}>
      <div className="grid gap-4 sm:grid-cols-2">
        {ops.map(({ icon: Icon, key }) => (
          <PanelCard key={key} icon={<Icon className="h-4 w-4" />} title={t(`operations.${key}`)}>
            <div className="space-y-3">
              <Placeholder className="min-h-[72px]">{t('comingSoonDesc')}</Placeholder>
              <Button size="sm" variant="outline" disabled className="w-full">
                {t(`operations.${key}`)}
              </Button>
            </div>
          </PanelCard>
        ))}
      </div>
    </SectionShell>
  )
}

function MapSection() {
  const t = useTranslations('serverManage')
  return (
    <SectionShell title={t('map.title')} desc={t('map.desc')}>
      <div className="grid gap-4 lg:grid-cols-[1fr_18rem]">
        <PanelCard icon={<MapIcon className="h-4 w-4" />} title={t('map.mapView')}>
          <Placeholder className="min-h-[320px]">{t('comingSoonDesc')}</Placeholder>
        </PanelCard>
        <PanelCard icon={<ListChecks className="h-4 w-4" />} title={t('map.whitelist')}>
          <Placeholder className="min-h-[320px]">{t('comingSoonDesc')}</Placeholder>
        </PanelCard>
      </div>
    </SectionShell>
  )
}

function RestApiSection() {
  const t = useTranslations('serverManage')
  // Representative endpoint groups from the official Palworld REST API, laid out
  // as reserved rows. Wiring is intentionally deferred.
  const endpoints = [
    { method: 'GET', path: '/v1/api/info' },
    { method: 'GET', path: '/v1/api/metrics' },
    { method: 'GET', path: '/v1/api/players' },
    { method: 'GET', path: '/v1/api/settings' },
    { method: 'POST', path: '/v1/api/announce' },
    { method: 'POST', path: '/v1/api/kick' },
    { method: 'POST', path: '/v1/api/ban' },
    { method: 'POST', path: '/v1/api/save' },
    { method: 'POST', path: '/v1/api/shutdown' },
  ]
  return (
    <SectionShell title={t('restapi.title')} desc={t('restapi.desc')}>
      <a
        href="https://docs.palworldgame.com/category/rest-api"
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex items-center gap-1.5 text-sm font-semibold text-primary hover:underline"
      >
        <ExternalLink className="h-4 w-4" />
        {t('restapi.docs')}
      </a>
      <Card className="rounded-2xl border-2 shadow-pal">
        <CardContent className="divide-y divide-border/60 p-0">
          {endpoints.map((e) => (
            <div
              key={e.path}
              className="flex items-center gap-3 px-4 py-2.5 opacity-70"
            >
              <span
                className={
                  'w-14 shrink-0 rounded-md px-2 py-0.5 text-center text-xs font-bold ' +
                  (e.method === 'GET'
                    ? 'bg-info/15 text-info'
                    : 'bg-success/15 text-success')
                }
              >
                {e.method}
              </span>
              <code className="truncate font-mono text-sm text-foreground">{e.path}</code>
            </div>
          ))}
        </CardContent>
      </Card>
    </SectionShell>
  )
}

function BackupSection() {
  const t = useTranslations('serverManage')
  return (
    <SectionShell title={t('backup.title')} desc={t('backup.desc')}>
      <div className="grid gap-4 lg:grid-cols-2">
        <PanelCard icon={<RefreshCw className="h-4 w-4" />} title={t('backup.sync')}>
          <Placeholder className="min-h-[120px]">{t('comingSoonDesc')}</Placeholder>
        </PanelCard>
        <PanelCard icon={<Clock className="h-4 w-4" />} title={t('backup.auto')}>
          <Placeholder className="min-h-[120px]">{t('comingSoonDesc')}</Placeholder>
        </PanelCard>
      </div>
      <PanelCard icon={<Archive className="h-4 w-4" />} title={t('backup.manage')}>
        <Placeholder className="min-h-[200px]">{t('comingSoonDesc')}</Placeholder>
      </PanelCard>
    </SectionShell>
  )
}

function SettingsSection({ onOpen }: { onOpen: () => void }) {
  const t = useTranslations('serverManage')
  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-xl font-bold text-foreground">{t('settings.title')}</h2>
        <p className="mt-1 text-sm text-muted-foreground">{t('settings.desc')}</p>
      </div>
      <PanelCard icon={<Settings className="h-4 w-4" />} title={t('settings.title')}>
        <p className="text-sm text-muted-foreground">{t('settings.hint')}</p>
        <Button onClick={onOpen} className="shadow-pal">
          <Settings className="mr-1 h-4 w-4" />
          {t('settings.open')}
        </Button>
      </PanelCard>
    </div>
  )
}
