'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Clock, Archive, Download, Trash2, RotateCcw, Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { backupsApi } from '@/lib/api'
import { getApiErrorMessage } from '@/lib/apiError'
import { useTranslations } from '@/contexts/LanguageContext'
import type { Backup, BackupScope, BackupSchedule } from '@/types/server'
import { useServer } from './shared'
import { SectionShell, Placeholder, PanelCard, useServerId } from './shared'

const SCOPES: BackupScope[] = ['all', 'save', 'config']

export function BackupSection() {
  const t = useTranslations('serverManage')
  const serverId = useServerId()
  return (
    <SectionShell title={t('backup.title')} desc={t('backup.desc')} comingSoon={false}>
      <PanelCard icon={<Clock className="h-4 w-4" />} title={t('backup.auto')}>
        <SchedulePanel serverId={serverId} />
      </PanelCard>
      <PanelCard icon={<Archive className="h-4 w-4" />} title={t('backup.manage')}>
        <ManagePanel serverId={serverId} />
      </PanelCard>
    </SectionShell>
  )
}

// --- helpers -----------------------------------------------------------------

function formatSize(bytes: number): string {
  if (bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let n = bytes
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

type Feedback = { kind: 'success' | 'error'; text: string } | null

function FeedbackLine({ feedback }: { feedback: Feedback }) {
  if (!feedback) return null
  return (
    <p
      className={
        'text-sm ' + (feedback.kind === 'success' ? 'text-emerald-600' : 'text-destructive')
      }
    >
      {feedback.text}
    </p>
  )
}

// --- Automatic-backup schedule -----------------------------------------------

function SchedulePanel({ serverId }: { serverId: number }) {
  const t = useTranslations('serverManage')
  const queryClient = useQueryClient()
  const [feedback, setFeedback] = useState<Feedback>(null)
  const [draft, setDraft] = useState<Omit<BackupSchedule, 'server_id'> | null>(null)

  const query = useQuery({
    queryKey: ['backupSchedule', serverId],
    queryFn: async () => (await backupsApi.getSchedule(serverId)).data,
    enabled: Number.isFinite(serverId),
    refetchOnWindowFocus: false,
  })

  // Initialize the editable draft once the schedule loads.
  const sched = draft ?? (query.data
    ? {
        enabled: query.data.enabled,
        interval_minutes: query.data.interval_minutes,
        scope: query.data.scope,
        keep_count: query.data.keep_count,
        keep_days: query.data.keep_days,
      }
    : null)

  const saveMut = useMutation({
    mutationFn: (data: Omit<BackupSchedule, 'server_id'>) =>
      backupsApi.updateSchedule(serverId, data),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('backup.form.saveOk') })
      queryClient.invalidateQueries({ queryKey: ['backupSchedule', serverId] })
    },
    onError: (err) =>
      setFeedback({ kind: 'error', text: getApiErrorMessage(err, t('backup.form.saveFail')) }),
  })

  if (query.isLoading || !sched) {
    return <Placeholder className="min-h-[120px]">{t('backup.loading')}</Placeholder>
  }

  const update = (patch: Partial<Omit<BackupSchedule, 'server_id'>>) =>
    setDraft({ ...sched, ...patch })

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-4 border-b border-dashed pb-3">
        <Label>{t('backup.form.enabled')}</Label>
        <Switch checked={sched.enabled} onCheckedChange={(v) => update({ enabled: v })} />
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label>{t('backup.form.interval')}</Label>
          <Input
            type="number"
            min={1}
            value={sched.interval_minutes}
            onChange={(e) => update({ interval_minutes: Number(e.target.value) })}
          />
        </div>
        <div className="space-y-1.5">
          <Label>{t('backup.form.scope')}</Label>
          <Select
            value={sched.scope}
            onChange={(e) => update({ scope: e.target.value as BackupScope })}
          >
            {SCOPES.map((s) => (
              <option key={s} value={s}>
                {t(`backup.scope.${s}`)}
              </option>
            ))}
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label>{t('backup.form.keepCount')}</Label>
          <Input
            type="number"
            min={0}
            value={sched.keep_count}
            onChange={(e) => update({ keep_count: Number(e.target.value) })}
          />
          <p className="text-xs text-muted-foreground">{t('backup.form.zeroUnlimited')}</p>
        </div>
        <div className="space-y-1.5">
          <Label>{t('backup.form.keepDays')}</Label>
          <Input
            type="number"
            min={0}
            value={sched.keep_days}
            onChange={(e) => update({ keep_days: Number(e.target.value) })}
          />
          <p className="text-xs text-muted-foreground">{t('backup.form.zeroUnlimited')}</p>
        </div>
      </div>

      <div className="flex items-center gap-3">
        <Button onClick={() => saveMut.mutate(sched)} disabled={saveMut.isPending}>
          {saveMut.isPending ? t('backup.form.saving') : t('backup.form.save')}
        </Button>
        <FeedbackLine feedback={feedback} />
      </div>
    </div>
  )
}

// --- Backup list / create / restore / download / delete ----------------------

function ManagePanel({ serverId }: { serverId: number }) {
  const t = useTranslations('serverManage')
  const queryClient = useQueryClient()
  const server = useServer()
  const [scope, setScope] = useState<BackupScope>('all')
  const [feedback, setFeedback] = useState<Feedback>(null)

  const isRunning = server.data?.status === 'running'

  const query = useQuery({
    queryKey: ['backups', serverId],
    queryFn: async () => (await backupsApi.list(serverId)).data.backups,
    enabled: Number.isFinite(serverId),
    refetchOnWindowFocus: false,
  })
  const backups = query.data ?? []

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['backups', serverId] })

  const createMut = useMutation({
    mutationFn: (s: BackupScope) => backupsApi.create(serverId, s),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('backup.list.createOk') })
      invalidate()
    },
    onError: (err) =>
      setFeedback({ kind: 'error', text: getApiErrorMessage(err, t('backup.list.createFail')) }),
  })

  const deleteMut = useMutation({
    mutationFn: (id: number) => backupsApi.remove(serverId, id),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('backup.list.deleteOk') })
      invalidate()
    },
    onError: (err) =>
      setFeedback({ kind: 'error', text: getApiErrorMessage(err, t('backup.list.deleteFail')) }),
  })

  const restoreMut = useMutation({
    mutationFn: (id: number) => backupsApi.restore(serverId, id),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('backup.list.restoreOk') })
      invalidate()
    },
    onError: (err) =>
      setFeedback({ kind: 'error', text: getApiErrorMessage(err, t('backup.list.restoreFail')) }),
  })

  const handleDownload = async (b: Backup) => {
    try {
      const res = await backupsApi.download(serverId, b.id)
      const url = window.URL.createObjectURL(res.data as Blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `server-${serverId}-backup-${b.id}.zip`
      document.body.appendChild(a)
      a.click()
      a.remove()
      window.URL.revokeObjectURL(url)
    } catch (err) {
      setFeedback({ kind: 'error', text: getApiErrorMessage(err, t('backup.list.downloadFail')) })
    }
  }

  const handleRestore = (b: Backup) => {
    if (isRunning) {
      setFeedback({ kind: 'error', text: t('backup.list.mustStop') })
      return
    }
    if (window.confirm(t('backup.list.restoreConfirm'))) {
      setFeedback(null)
      restoreMut.mutate(b.id)
    }
  }

  const handleDelete = (b: Backup) => {
    if (window.confirm(t('backup.list.deleteConfirm'))) {
      setFeedback(null)
      deleteMut.mutate(b.id)
    }
  }

  return (
    <div className="space-y-4">
      {/* Create bar */}
      <div className="flex flex-wrap items-end gap-3">
        <div className="space-y-1.5">
          <Label>{t('backup.form.scope')}</Label>
          <Select value={scope} onChange={(e) => setScope(e.target.value as BackupScope)}>
            {SCOPES.map((s) => (
              <option key={s} value={s}>
                {t(`backup.scope.${s}`)}
              </option>
            ))}
          </Select>
        </div>
        <Button onClick={() => createMut.mutate(scope)} disabled={createMut.isPending}>
          <Plus className="mr-1.5 h-4 w-4" />
          {createMut.isPending ? t('backup.list.creating') : t('backup.list.create')}
        </Button>
      </div>

      <FeedbackLine feedback={feedback} />

      {/* List */}
      {query.isLoading ? (
        <Placeholder className="min-h-[120px]">{t('backup.loading')}</Placeholder>
      ) : backups.length === 0 ? (
        <Placeholder className="min-h-[120px]">{t('backup.list.empty')}</Placeholder>
      ) : (
        <div className="space-y-2">
          {backups.map((b) => (
            <div
              key={b.id}
              className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border/70 bg-muted/20 px-4 py-3"
            >
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-foreground">
                    {new Date(b.created_at).toLocaleString()}
                  </span>
                  <Badge variant="secondary">{t(`backup.scope.${b.scope}`)}</Badge>
                  <Badge variant="info">{t(`backup.source.${b.source}`)}</Badge>
                  {b.hot && <Badge variant="warning">{t('backup.list.hot')}</Badge>}
                </div>
                <p className="text-xs text-muted-foreground">{formatSize(b.size_bytes)}</p>
              </div>
              <div className="flex items-center gap-1.5">
                <Button variant="outline" size="sm" onClick={() => handleDownload(b)}>
                  <Download className="h-4 w-4" />
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleRestore(b)}
                  disabled={restoreMut.isPending}
                  title={isRunning ? t('backup.list.mustStop') : t('backup.list.restore')}
                >
                  <RotateCcw className="h-4 w-4" />
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleDelete(b)}
                  disabled={deleteMut.isPending}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
