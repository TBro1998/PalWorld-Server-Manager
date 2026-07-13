'use client'

import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Megaphone, Save, Power, PowerOff } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
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
import { SectionShell, PanelCard, useServerId } from './shared'
import { RestUnavailableNotice } from './RestUnavailableNotice'

type Feedback = { kind: 'success' | 'error'; text: string } | null

// Broadcast / save / graceful shutdown / immediate stop. The two shutdown paths
// are destructive and gated behind a confirmation dialog.
export function OperationsSection() {
  const t = useTranslations('serverManage')
  const serverId = useServerId()
  const queryClient = useQueryClient()
  const { status, isAvailable } = useRestStatus(serverId)

  const [announceMsg, setAnnounceMsg] = useState('')
  const [waitTime, setWaitTime] = useState('30')
  const [shutdownMsg, setShutdownMsg] = useState('')
  const [confirm, setConfirm] = useState<'shutdown' | 'stop' | null>(null)
  const [feedback, setFeedback] = useState<Feedback>(null)

  const onError = (fallback: string) => (err: unknown) =>
    setFeedback({ kind: 'error', text: getApiErrorMessage(err, fallback) })

  const announceMut = useMutation({
    mutationFn: (message: string) => serversApi.restAnnounce(serverId, { message }),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('operations.feedback.announceOk') })
      setAnnounceMsg('')
    },
    onError: onError(t('operations.feedback.announceFail')),
  })

  const saveMut = useMutation({
    mutationFn: () => serversApi.restSave(serverId),
    onSuccess: () => setFeedback({ kind: 'success', text: t('operations.feedback.saveOk') }),
    onError: onError(t('operations.feedback.saveFail')),
  })

  const shutdownMut = useMutation({
    mutationFn: ({ waittime, message }: { waittime: number; message: string }) =>
      serversApi.restShutdown(serverId, { waittime, message }),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('operations.feedback.shutdownOk') })
      queryClient.invalidateQueries({ queryKey: ['rest-status', serverId] })
    },
    onError: onError(t('operations.feedback.shutdownFail')),
  })

  const stopMut = useMutation({
    mutationFn: () => serversApi.restStop(serverId),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('operations.feedback.stopOk') })
      queryClient.invalidateQueries({ queryKey: ['rest-status', serverId] })
    },
    onError: onError(t('operations.feedback.stopFail')),
  })

  const confirmAction = () => {
    if (confirm === 'shutdown') {
      const secs = Number.parseInt(waitTime, 10)
      shutdownMut.mutate({ waittime: Number.isFinite(secs) ? secs : 0, message: shutdownMsg })
    } else if (confirm === 'stop') {
      stopMut.mutate()
    }
    setConfirm(null)
  }

  const disabled = !isAvailable

  return (
    <SectionShell title={t('operations.title')} desc={t('operations.desc')} comingSoon={false}>
      {!isAvailable && <RestUnavailableNotice status={status} />}

      {feedback && (
        <p
          className={
            'text-sm ' + (feedback.kind === 'success' ? 'text-success' : 'text-destructive')
          }
        >
          {feedback.text}
        </p>
      )}

      <div className="grid gap-4 sm:grid-cols-2">
        {/* Broadcast */}
        <PanelCard icon={<Megaphone className="h-4 w-4" />} title={t('operations.broadcast')}>
          <div className="space-y-3">
            <Textarea
              value={announceMsg}
              onChange={(e) => setAnnounceMsg(e.target.value)}
              placeholder={t('operations.broadcastPlaceholder')}
              disabled={disabled}
              className="min-h-[72px]"
            />
            <Button
              size="sm"
              className="w-full"
              onClick={() => announceMut.mutate(announceMsg)}
              disabled={disabled || !announceMsg.trim() || announceMut.isPending}
            >
              {t('operations.broadcastSend')}
            </Button>
          </div>
        </PanelCard>

        {/* Save */}
        <PanelCard icon={<Save className="h-4 w-4" />} title={t('operations.save')}>
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">{t('operations.saveDesc')}</p>
            <Button
              size="sm"
              variant="outline"
              className="w-full"
              onClick={() => saveMut.mutate()}
              disabled={disabled || saveMut.isPending}
            >
              {t('operations.saveAction')}
            </Button>
          </div>
        </PanelCard>

        {/* Graceful shutdown */}
        <PanelCard icon={<Power className="h-4 w-4" />} title={t('operations.shutdown')}>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="shutdown-wait">{t('operations.shutdownWait')}</Label>
              <Input
                id="shutdown-wait"
                type="number"
                min={0}
                value={waitTime}
                onChange={(e) => setWaitTime(e.target.value)}
                disabled={disabled}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="shutdown-msg">{t('operations.shutdownMessage')}</Label>
              <Input
                id="shutdown-msg"
                value={shutdownMsg}
                onChange={(e) => setShutdownMsg(e.target.value)}
                placeholder={t('operations.shutdownMessagePlaceholder')}
                disabled={disabled}
              />
            </div>
            <Button
              size="sm"
              variant="outline"
              className="w-full"
              onClick={() => setConfirm('shutdown')}
              disabled={disabled || shutdownMut.isPending}
            >
              {t('operations.shutdownAction')}
            </Button>
          </div>
        </PanelCard>

        {/* Immediate stop */}
        <PanelCard icon={<PowerOff className="h-4 w-4" />} title={t('operations.stop')}>
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">{t('operations.stopDesc')}</p>
            <Button
              size="sm"
              variant="destructive"
              className="w-full"
              onClick={() => setConfirm('stop')}
              disabled={disabled || stopMut.isPending}
            >
              {t('operations.stopAction')}
            </Button>
          </div>
        </PanelCard>
      </div>

      {/* Confirmation for the destructive shutdown/stop actions. */}
      <Dialog open={confirm !== null} onOpenChange={(o) => !o && setConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {confirm === 'shutdown'
                ? t('operations.confirm.shutdownTitle')
                : t('operations.confirm.stopTitle')}
            </DialogTitle>
            <DialogDescription>
              {confirm === 'shutdown'
                ? t('operations.confirm.shutdownDesc')
                : t('operations.confirm.stopDesc')}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setConfirm(null)}>
              {t('operations.confirm.cancel')}
            </Button>
            <Button type="button" variant="destructive" onClick={confirmAction}>
              {t('operations.confirm.confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SectionShell>
  )
}
