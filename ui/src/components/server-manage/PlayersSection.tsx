'use client'

import { useEffect, useMemo, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { UserX, Ban, RotateCcw, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Select } from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog'
import { serversApi } from '@/lib/api'
import { getApiErrorMessage } from '@/lib/apiError'
import { useTranslations } from '@/contexts/LanguageContext'
import { useRestStatus } from '@/hooks/useRestStatus'
import type { PalPlayer, SaveGuild, SavePal, SaveItem } from '@/types/server'
import { SectionShell, Placeholder, useServerId } from './shared'
import { RestUnavailableNotice } from './RestUnavailableNotice'

type TabKey = 'players' | 'guilds' | 'pals' | 'inventory'
const TABS: TabKey[] = ['players', 'guilds', 'pals', 'inventory']

// Feedback shown inline after a mutation (no toast library in this project).
type Feedback = { kind: 'success' | 'error'; text: string } | null

// A pending destructive action awaiting confirmation.
type PendingAction =
  | { type: 'kick'; userid: string; name: string }
  | { type: 'ban'; userid: string; name: string }
  | { type: 'unban'; userid: string; name: string }
  | null

export function PlayersSection() {
  const t = useTranslations('serverManage')
  const serverId = useServerId()
  const queryClient = useQueryClient()
  const { status, isAvailable } = useRestStatus(serverId)

  const [tab, setTab] = useState<TabKey>('players')
  const [unbanId, setUnbanId] = useState('')
  const [pending, setPending] = useState<PendingAction>(null)
  const [banMessage, setBanMessage] = useState('')
  const [feedback, setFeedback] = useState<Feedback>(null)

  // The player list does NOT auto-poll; it fetches once when the section becomes
  // available and afterwards refreshes only when the user clicks "Refresh".
  const playersQuery = useQuery({
    queryKey: ['rest-players', serverId],
    queryFn: async () => (await serversApi.restPlayers(serverId)).data,
    enabled: isAvailable,
    refetchOnWindowFocus: false,
  })
  const players = playersQuery.data?.players ?? []

  const invalidatePlayers = () =>
    queryClient.invalidateQueries({ queryKey: ['rest-players', serverId] })

  const kickMut = useMutation({
    mutationFn: (userid: string) => serversApi.restKick(serverId, { userid, message: '' }),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('players.actions.kickOk') })
      invalidatePlayers()
    },
    onError: (err) =>
      setFeedback({ kind: 'error', text: getApiErrorMessage(err, t('players.actions.kickFail')) }),
  })

  const banMut = useMutation({
    mutationFn: ({ userid, message }: { userid: string; message: string }) =>
      serversApi.restBan(serverId, { userid, message }),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('players.actions.banOk') })
      invalidatePlayers()
    },
    onError: (err) =>
      setFeedback({ kind: 'error', text: getApiErrorMessage(err, t('players.actions.banFail')) }),
  })

  const unbanMut = useMutation({
    mutationFn: (userid: string) => serversApi.restUnban(serverId, { userid }),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('players.actions.unbanOk') })
      setUnbanId('')
      invalidatePlayers()
    },
    onError: (err) =>
      setFeedback({ kind: 'error', text: getApiErrorMessage(err, t('players.actions.unbanFail')) }),
  })

  const closeDialog = () => {
    setPending(null)
    setBanMessage('')
  }

  const confirmAction = () => {
    if (!pending) return
    if (pending.type === 'kick') kickMut.mutate(pending.userid)
    else if (pending.type === 'ban') banMut.mutate({ userid: pending.userid, message: banMessage })
    else if (pending.type === 'unban') unbanMut.mutate(pending.userid)
    closeDialog()
  }

  const openUnban = () => {
    const id = unbanId.trim()
    if (!id) return
    setPending({ type: 'unban', userid: id, name: id })
  }

  const dialogPending =
    (pending?.type === 'kick' && kickMut.isPending) ||
    (pending?.type === 'ban' && banMut.isPending) ||
    (pending?.type === 'unban' && unbanMut.isPending)

  return (
    <SectionShell title={t('players.title')} desc={t('players.desc')} comingSoon={false}>
      {/* Inner tab bar for the four data domains. */}
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

      {/* --- Live online players (via REST) --- */}
      {tab === 'players' &&
        (!isAvailable ? (
          <RestUnavailableNotice status={status} />
        ) : (
          <div className="space-y-4">
            {/* Manual refresh: the player list does not auto-poll. */}
            <div className="flex justify-end">
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="gap-2 rounded-xl border-2 shadow-pal"
                onClick={() => playersQuery.refetch()}
                disabled={!isAvailable || playersQuery.isFetching}
              >
                <RefreshCw className={`h-4 w-4 ${playersQuery.isFetching ? 'animate-spin' : ''}`} />
                {t('players.refresh')}
              </Button>
            </div>

            {feedback && (
              <p
                className={
                  'text-sm ' +
                  (feedback.kind === 'success' ? 'text-success' : 'text-destructive')
                }
              >
                {feedback.text}
              </p>
            )}

            {/* Unban by userId. */}
            <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
              <div className="flex-1 space-y-1.5">
                <Label htmlFor="unban-userid">{t('players.unban.label')}</Label>
                <Input
                  id="unban-userid"
                  value={unbanId}
                  onChange={(e) => setUnbanId(e.target.value)}
                  placeholder={t('players.unban.placeholder')}
                />
              </div>
              <Button
                type="button"
                variant="outline"
                onClick={openUnban}
                disabled={!unbanId.trim() || unbanMut.isPending}
              >
                <RotateCcw className="h-4 w-4" />
                {t('players.unban.action')}
              </Button>
            </div>

            {/* Online player table. */}
            {players.length === 0 ? (
              <Placeholder className="min-h-[160px]">{t('overview.noPlayers')}</Placeholder>
            ) : (
              <div className="overflow-x-auto rounded-2xl border-2 shadow-pal">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b-2 bg-muted/40 text-left text-xs font-bold uppercase text-muted-foreground">
                      <th className="px-4 py-2.5">{t('players.table.name')}</th>
                      <th className="px-4 py-2.5">{t('players.table.level')}</th>
                      <th className="px-4 py-2.5">{t('players.table.ping')}</th>
                      <th className="px-4 py-2.5">{t('players.table.userId')}</th>
                      <th className="px-4 py-2.5 text-right">{t('players.table.actions')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border/60">
                    {players.map((p) => (
                      <PlayerRow
                        key={p.userId || p.playerId || p.name}
                        player={p}
                        onKick={() => setPending({ type: 'kick', userid: p.userId, name: p.name })}
                        onBan={() => {
                          setBanMessage('')
                          setPending({ type: 'ban', userid: p.userId, name: p.name })
                        }}
                        kickLabel={t('players.table.kick')}
                        banLabel={t('players.table.ban')}
                      />
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        ))}

      {/* --- Parsed save data (offline-capable, independent of live REST) --- */}
      {tab === 'guilds' && <GuildsView serverId={serverId} />}
      {tab === 'pals' && <PalsView serverId={serverId} />}
      {tab === 'inventory' && <InventoryView serverId={serverId} />}

      {/* Confirmation dialog for the destructive actions (players tab only). */}
      <Dialog open={pending !== null} onOpenChange={(o) => !o && closeDialog()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {pending?.type === 'kick' && t('players.confirm.kickTitle')}
              {pending?.type === 'ban' && t('players.confirm.banTitle')}
              {pending?.type === 'unban' && t('players.confirm.unbanTitle')}
            </DialogTitle>
            <DialogDescription>
              {pending ? `${t('players.confirm.target')}: ${pending.name}` : ''}
            </DialogDescription>
          </DialogHeader>

          {pending?.type === 'ban' && (
            <div className="space-y-1.5">
              <Label htmlFor="ban-message">{t('players.confirm.banMessage')}</Label>
              <Input
                id="ban-message"
                value={banMessage}
                onChange={(e) => setBanMessage(e.target.value)}
                placeholder={t('players.confirm.banMessagePlaceholder')}
              />
            </div>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={closeDialog} disabled={dialogPending}>
              {t('players.confirm.cancel')}
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={confirmAction}
              disabled={dialogPending}
            >
              {t('players.confirm.confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SectionShell>
  )
}

function PlayerRow({
  player,
  onKick,
  onBan,
  kickLabel,
  banLabel,
}: {
  player: PalPlayer
  onKick: () => void
  onBan: () => void
  kickLabel: string
  banLabel: string
}) {
  return (
    <tr className="text-foreground">
      <td className="px-4 py-2.5 font-semibold">{player.name}</td>
      <td className="px-4 py-2.5">
        <Badge variant="secondary">Lv.{player.level}</Badge>
      </td>
      <td className="px-4 py-2.5 text-muted-foreground">{Math.round(player.ping)} ms</td>
      <td className="px-4 py-2.5 font-mono text-xs text-muted-foreground">{player.userId || '—'}</td>
      <td className="px-4 py-2.5">
        <div className="flex justify-end gap-2">
          <Button type="button" size="sm" variant="outline" onClick={onKick}>
            <UserX className="h-4 w-4" />
            {kickLabel}
          </Button>
          <Button type="button" size="sm" variant="destructive" onClick={onBan}>
            <Ban className="h-4 w-4" />
            {banLabel}
          </Button>
        </div>
      </td>
    </tr>
  )
}

// --- Save-data views ---------------------------------------------------------

type QueryLike = { isLoading: boolean; isError: boolean; error: unknown }

// Renders loading / not-found / error placeholders for a save query, or null
// when the data is ready. A 404 means "no save on disk" (distinct from a real
// failure) and gets a dedicated hint.
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
// Returns '—' for zero or out-of-range values (guards against unexpected units).
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

// Player picker shared by the pals and inventory tabs. Auto-selects the first
// player once the list loads and nothing is selected yet.
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

function PalsTable({
  q,
}: {
  q: QueryLike & { data?: SavePal[] }
}) {
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
