'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { UserX, Ban, RotateCcw, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
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
import type { PalPlayer } from '@/types/server'
import { SectionShell, Placeholder, useServerId } from './shared'
import { RestUnavailableNotice } from './RestUnavailableNotice'

// ---------------------------------------------------------------------------
// World-map helpers (Palpagos Island, Unreal Engine units)
// ---------------------------------------------------------------------------
const MAP_X_MIN = -582000
const MAP_X_MAX = 582000
const MAP_Y_MIN = -582000
const MAP_Y_MAX = 582000

function worldToPercent(x: number, y: number) {
  const px = ((x - MAP_X_MIN) / (MAP_X_MAX - MAP_X_MIN)) * 100
  // Screen Y is inverted relative to the game world Y axis.
  const py = (1 - (y - MAP_Y_MIN) / (MAP_Y_MAX - MAP_Y_MIN)) * 100
  return {
    px: Math.max(0, Math.min(100, px)),
    py: Math.max(0, Math.min(100, py)),
  }
}

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

  const [hovered, setHovered] = useState<PalPlayer | null>(null)
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
      {!isAvailable ? (
        <RestUnavailableNotice status={status} />
      ) : (
        <div className="space-y-4">
          {/* Manual refresh */}
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

          {/* Unban by userId */}
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

          {/* Map + online player table */}
          {players.length === 0 ? (
            <Placeholder className="min-h-[160px]">{t('overview.noPlayers')}</Placeholder>
          ) : (
            <div className="space-y-4">
              {/* World map with player positions - full width on top */}
              <InlinePlayerMap
                players={players}
                hovered={hovered}
                onHover={setHovered}
              />

              {/* Player table - full width below */}
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
                        isHovered={hovered?.userId === p.userId}
                        onHover={() => setHovered(p)}
                        onLeave={() => setHovered(null)}
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
            </div>
          )}
        </div>
      )}

      {/* Confirmation dialog */}
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

// ---------------------------------------------------------------------------
// Inline player map
// ---------------------------------------------------------------------------

function InlinePlayerMap({
  players,
  hovered,
  onHover,
}: {
  players: PalPlayer[]
  hovered: PalPlayer | null
  onHover: (p: PalPlayer | null) => void
}) {
  return (
    <div className="space-y-2">
      <div
        className="relative overflow-hidden rounded-xl border-2 border-border/60 bg-[#1b2a3b]"
        style={{ aspectRatio: '1 / 1' }}
      >
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src="/World_Map.webp"
          alt="Palpagos Island"
          className="pointer-events-none absolute inset-0 h-full w-full object-cover select-none"
          draggable={false}
        />

        {players.map((p) => {
          const { px, py } = worldToPercent(p.location_x, p.location_y)
          const isHov = hovered?.userId === p.userId
          return (
            <button
              key={p.userId || p.playerId || p.name}
              type="button"
              className="absolute -translate-x-1/2 -translate-y-1/2 focus:outline-none"
              style={{ left: `${px}%`, top: `${py}%` }}
              onMouseEnter={() => onHover(p)}
              onMouseLeave={() => onHover(null)}
            >
              <span
                className={
                  'block rounded-full border-2 border-white/70 transition-all duration-150 ' +
                  (isHov
                    ? 'h-4 w-4 bg-primary shadow-[0_0_8px_2px_hsl(var(--primary)/0.6)]'
                    : 'h-3 w-3 bg-emerald-400')
                }
              />
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
      </div>

      {/* Legend chips */}
      <div className="flex flex-wrap gap-1">
        {players.map((p) => (
          <button
            key={p.userId || p.name}
            type="button"
            className={
              'flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs transition-colors ' +
              (hovered?.userId === p.userId
                ? 'border-primary bg-primary/10 text-foreground'
                : 'border-transparent bg-secondary text-muted-foreground hover:text-foreground')
            }
            onMouseEnter={() => onHover(p)}
            onMouseLeave={() => onHover(null)}
          >
            <span className="inline-block h-2 w-2 rounded-full bg-emerald-400" />
            {p.name}
          </button>
        ))}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Player table row
// ---------------------------------------------------------------------------

function PlayerRow({
  player,
  isHovered,
  onHover,
  onLeave,
  onKick,
  onBan,
  kickLabel,
  banLabel,
}: {
  player: PalPlayer
  isHovered?: boolean
  onHover?: () => void
  onLeave?: () => void
  onKick: () => void
  onBan: () => void
  kickLabel: string
  banLabel: string
}) {
  return (
    <tr
      className={'text-foreground transition-colors ' + (isHovered ? 'bg-primary/5' : '')}
      onMouseEnter={onHover}
      onMouseLeave={onLeave}
    >
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
