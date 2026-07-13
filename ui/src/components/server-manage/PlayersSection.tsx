'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { UserX, Ban, RotateCcw } from 'lucide-react'
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

  const { data: playersData } = useQuery({
    queryKey: ['rest-players', serverId],
    queryFn: async () => (await serversApi.restPlayers(serverId)).data,
    enabled: isAvailable,
    refetchInterval: 5000,
  })
  const players = playersData?.players ?? []

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

      {tab !== 'players' ? (
        <Placeholder className="min-h-[280px]">{t('comingSoonDesc')}</Placeholder>
      ) : !isAvailable ? (
        <RestUnavailableNotice status={status} />
      ) : (
        <div className="space-y-4">
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
      )}

      {/* Confirmation dialog for the destructive actions. */}
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
