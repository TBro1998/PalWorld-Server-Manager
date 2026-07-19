'use client'

import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { Badge } from '@/components/ui/badge'
import { Select } from '@/components/ui/select'
import { Label } from '@/components/ui/label'
import { serversApi } from '@/lib/api'
import { getApiErrorMessage } from '@/lib/apiError'
import { useTranslations } from '@/contexts/LanguageContext'
import type { SaveGuild, SavePal, SaveItem } from '@/types/server'
import { SectionShell, Placeholder, useServerId } from './shared'

type TabKey = 'guilds' | 'pals' | 'inventory'
const TABS: TabKey[] = ['guilds', 'pals', 'inventory']

export function SaveDataSection() {
  const t = useTranslations('serverManage')
  const serverId = useServerId()
  const [tab, setTab] = useState<TabKey>('guilds')

  return (
    <SectionShell title={t('saveData.title')} desc={t('saveData.desc')} comingSoon={false}>
      {/* Tab bar */}
      <div className="flex flex-wrap gap-1.5">
        {TABS.map((tb) => {
          const active = tb === tab
          return (
            <button
              key={tb}
              type="button"
              onClick={() => setTab(tb)}
              className={
                'rounded-full px-3.5 py-1.5 text-sm font-semibold transition-colors ' +
                (active
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-secondary text-muted-foreground hover:text-foreground')
              }
            >
              {t(`players.tabs.${tb}`)}
            </button>
          )
        })}
      </div>

      {tab === 'guilds' && <GuildsView serverId={serverId} />}
      {tab === 'pals' && <PalsView serverId={serverId} />}
      {tab === 'inventory' && <InventoryView serverId={serverId} />}
    </SectionShell>
  )
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

type QueryLike = { isLoading: boolean; isError: boolean; error: unknown }

function saveStatus(q: QueryLike, t: (k: string) => string): React.ReactNode | null {
  if (q.isLoading) return <Placeholder className="min-h-[160px]">{t('players.save.loading')}</Placeholder>
  if (q.isError) {
    const notFound = q.error instanceof AxiosError && q.error.response?.status === 404
    const msg = notFound
      ? t('players.save.noSave')
      : getApiErrorMessage(q.error, t('players.save.loadError'))
    return <Placeholder className="min-h-[160px]">{msg}</Placeholder>
  }
  return null
}

// Unreal FDateTime ticks (100ns since 0001-01-01 UTC) -> localized date string.
function formatTicks(ticks: number): string {
  if (!ticks) return '—'
  const unixMs = ticks / 10000 - 62135596800000
  const d = new Date(unixMs)
  const year = d.getFullYear()
  if (Number.isNaN(unixMs) || year < 2020 || year > 2100) return '—'
  return d.toLocaleString()
}

function useSavePlayersQuery(serverId: number) {
  return useQuery({
    queryKey: ['save-players', serverId],
    queryFn: async () => (await serversApi.savePlayers(serverId)).data.players,
    enabled: Number.isFinite(serverId),
    refetchOnWindowFocus: false,
  })
}

// Player picker shared by the pals and inventory tabs.
function PlayerSelect({
  serverId,
  value,
  onChange,
}: {
  serverId: number
  value: string
  onChange: (uid: string) => void
}) {
  const t = useTranslations('serverManage')
  const q = useSavePlayersQuery(serverId)
  const players = useMemo(() => q.data ?? [], [q.data])

  useEffect(() => {
    if (!value && players.length) onChange(players[0].uid)
  }, [players, value, onChange])

  const state = saveStatus(q, t)
  if (state) return state
  if (!players.length) return <Placeholder className="min-h-[160px]">{t('players.save.empty')}</Placeholder>

  return (
    <div className="space-y-1.5">
      <Label htmlFor="save-player-select">{t('players.save.selectPlayer')}</Label>
      <Select
        id="save-player-select"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="sm:max-w-xs"
      >
        <option value="" disabled>
          {t('players.save.selectPlayerPlaceholder')}
        </option>
        {players.map((p) => (
          <option key={p.uid} value={p.uid}>
            {p.name || p.uid} {p.level ? `(Lv.${p.level})` : ''}
          </option>
        ))}
      </Select>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Guilds view
// ---------------------------------------------------------------------------

function GuildsView({ serverId }: { serverId: number }) {
  const t = useTranslations('serverManage')
  const q = useQuery({
    queryKey: ['save-guilds', serverId],
    queryFn: async () => (await serversApi.saveGuilds(serverId)).data.guilds,
    enabled: Number.isFinite(serverId),
    refetchOnWindowFocus: false,
  })
  const state = saveStatus(q, t)
  if (state) return state
  const guilds = q.data ?? []
  if (!guilds.length) return <Placeholder className="min-h-[160px]">{t('players.save.guild.empty')}</Placeholder>

  return (
    <div className="space-y-4">
      {guilds.map((g) => (
        <GuildCard key={g.guildId} guild={g} />
      ))}
    </div>
  )
}

function GuildCard({ guild }: { guild: SaveGuild }) {
  const t = useTranslations('serverManage')
  return (
    <div className="overflow-hidden rounded-2xl border-2 shadow-pal">
      <div className="flex flex-wrap items-center gap-3 bg-muted/40 px-4 py-3">
        <span className="font-bold text-foreground">{guild.name || '—'}</span>
        <Badge variant="secondary">
          {t('players.save.guild.baseCamp')} {guild.baseCampLevel}
        </Badge>
        <span className="font-mono text-xs text-muted-foreground">
          {t('players.save.guild.admin')}: {guild.adminUid || '—'}
        </span>
      </div>
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b bg-muted/20 text-left text-xs font-bold uppercase text-muted-foreground">
            <th className="px-4 py-2">{t('players.save.guild.memberName')}</th>
            <th className="px-4 py-2">{t('players.save.guild.role')}</th>
            <th className="px-4 py-2">{t('players.save.guild.lastOnline')}</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border/60">
          {guild.members.map((m) => (
            <tr key={m.uid} className="text-foreground">
              <td className="px-4 py-2 font-semibold">
                {m.name || '—'}
                <span className="ml-2 font-mono text-xs text-muted-foreground">{m.uid}</span>
              </td>
              <td className="px-4 py-2 text-muted-foreground">{m.role}</td>
              <td className="px-4 py-2 text-muted-foreground">{formatTicks(m.lastOnline)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Pals view
// ---------------------------------------------------------------------------

function PalsView({ serverId }: { serverId: number }) {
  const [uid, setUid] = useState('')

  const palsQ = useQuery({
    queryKey: ['save-pals', serverId, uid],
    queryFn: async () => (await serversApi.savePals(serverId, uid)).data.pals,
    enabled: Number.isFinite(serverId) && !!uid,
    refetchOnWindowFocus: false,
  })

  return (
    <div className="space-y-4">
      <PlayerSelect serverId={serverId} value={uid} onChange={setUid} />
      {uid && <PalsTable q={palsQ} />}
    </div>
  )
}

function PalsTable({ q }: { q: QueryLike & { data?: SavePal[] } }) {
  const t = useTranslations('serverManage')
  const state = saveStatus(q, t)
  if (state) return state
  const pals = q.data ?? []
  if (!pals.length) return <Placeholder className="min-h-[160px]">{t('players.save.pal.empty')}</Placeholder>

  return (
    <div className="overflow-x-auto rounded-2xl border-2 shadow-pal">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b-2 bg-muted/40 text-left text-xs font-bold uppercase text-muted-foreground">
            <th className="px-4 py-2.5">{t('players.save.pal.name')}</th>
            <th className="px-4 py-2.5">{t('players.save.pal.species')}</th>
            <th className="px-4 py-2.5">{t('players.save.pal.level')}</th>
            <th className="px-4 py-2.5">{t('players.save.pal.gender')}</th>
            <th className="px-4 py-2.5">{t('players.save.pal.rank')}</th>
            <th className="px-4 py-2.5">{t('players.save.pal.talent')}</th>
            <th className="px-4 py-2.5">{t('players.save.pal.passives')}</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border/60">
          {pals.map((p) => (
            <tr key={p.instanceId} className="text-foreground">
              <td className="px-4 py-2.5 font-semibold">{p.name || p.species}</td>
              <td className="px-4 py-2.5 text-muted-foreground">{p.species}</td>
              <td className="px-4 py-2.5">
                <Badge variant="secondary">Lv.{p.level}</Badge>
              </td>
              <td className="px-4 py-2.5 text-muted-foreground">{p.gender || '—'}</td>
              <td className="px-4 py-2.5 text-muted-foreground">{p.rank}</td>
              <td className="px-4 py-2.5 font-mono text-xs text-muted-foreground">
                {p.talent.hp}/{p.talent.melee}/{p.talent.shot}/{p.talent.defense}
              </td>
              <td className="px-4 py-2.5 text-xs text-muted-foreground">
                {p.passives?.length ? p.passives.join(', ') : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Inventory view
// ---------------------------------------------------------------------------

function InventoryView({ serverId }: { serverId: number }) {
  const t = useTranslations('serverManage')
  const [uid, setUid] = useState('')

  const invQ = useQuery({
    queryKey: ['save-inventory', serverId, uid],
    queryFn: async () => (await serversApi.saveInventory(serverId, uid)).data.inventory,
    enabled: Number.isFinite(serverId) && !!uid,
    refetchOnWindowFocus: false,
  })

  const state = saveStatus(invQ, t)
  const inventory = invQ.data ?? {}
  const containers = Object.entries(inventory).filter(([, items]) => items.length > 0)

  return (
    <div className="space-y-4">
      <PlayerSelect serverId={serverId} value={uid} onChange={setUid} />
      {uid &&
        (state ? (
          state
        ) : containers.length === 0 ? (
          <Placeholder className="min-h-[160px]">{t('players.save.item.empty')}</Placeholder>
        ) : (
          <div className="space-y-4">
            {containers.map(([container, items]) => (
              <ContainerTable key={container} container={container} items={items} />
            ))}
          </div>
        ))}
    </div>
  )
}

function ContainerTable({ container, items }: { container: string; items: SaveItem[] }) {
  const t = useTranslations('serverManage')
  return (
    <div className="overflow-hidden rounded-2xl border-2 shadow-pal">
      <div className="bg-muted/40 px-4 py-2 font-mono text-xs font-bold text-muted-foreground">
        {container}
      </div>
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b bg-muted/20 text-left text-xs font-bold uppercase text-muted-foreground">
            <th className="px-4 py-2">{t('players.save.item.slot')}</th>
            <th className="px-4 py-2">{t('players.save.item.staticId')}</th>
            <th className="px-4 py-2">{t('players.save.item.count')}</th>
            <th className="px-4 py-2">{t('players.save.item.durability')}</th>
            <th className="px-4 py-2">{t('players.save.item.passives')}</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border/60">
          {items.map((it, i) => (
            <tr key={`${it.slot}-${i}`} className="text-foreground">
              <td className="px-4 py-2 text-muted-foreground">{it.slot}</td>
              <td className="px-4 py-2 font-semibold">{it.staticId}</td>
              <td className="px-4 py-2">{it.count}</td>
              <td className="px-4 py-2 text-muted-foreground">
                {it.durability ? it.durability.toFixed(1) : '—'}
              </td>
              <td className="px-4 py-2 text-xs text-muted-foreground">
                {it.passives?.length ? it.passives.join(', ') : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
