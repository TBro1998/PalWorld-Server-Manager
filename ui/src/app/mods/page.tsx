'use client'

import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Package,
  Plus,
  Download,
  RefreshCw,
  Trash2,
  CheckCircle2,
  AlertTriangle,
  Loader2,
  X,
  ChevronDown,
  ChevronUp,
} from 'lucide-react'
import { globalModsApi, steamApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'
import type { Mod } from '@/types/server'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { WorkshopBrowserDialog } from '@/components/server-manage/WorkshopBrowserDialog'

export default function ModsPage() {
  const t = useTranslations('modLibrary')
  const tc = useTranslations('serverConfig')
  const queryClient = useQueryClient()

  const [addOpen, setAddOpen] = useState(false)
  const [browseOpen, setBrowseOpen] = useState(false)
  const [workshopId, setWorkshopId] = useState('')
  const [modName, setModName] = useState('')
  const [addError, setAddError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Mod | null>(null)
  // Support concurrent downloads: track all downloading mod IDs.
  const [downloadingIds, setDownloadingIds] = useState<Set<number>>(new Set())

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['globalMods'] })

  const { data, isLoading } = useQuery({
    queryKey: ['globalMods'],
    queryFn: async () => (await globalModsApi.list()).data,
    refetchInterval: downloadingIds.size > 0 ? 3000 : false,
  })
  const mods: Mod[] = data?.mods ?? []

  const { data: steamStatus } = useQuery({
    queryKey: ['steamStatus'],
    queryFn: async () => (await steamApi.status()).data,
  })
  const webApiKeyConfigured = steamStatus?.webApiKeyConfigured ?? false

  const addMutation = useMutation({
    mutationFn: async () =>
      globalModsApi.add({ workshopId: workshopId.trim(), name: modName.trim() || undefined }),
    onSuccess: () => {
      setWorkshopId('')
      setModName('')
      setAddError(null)
      setAddOpen(false)
      invalidate()
    },
    onError: (err: unknown) => {
      const e = err as { response?: { data?: { error?: string } } }
      setAddError(e.response?.data?.error ?? 'Failed to add mod')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (modId: number) => globalModsApi.remove(modId),
    onSuccess: () => {
      setDeleteTarget(null)
      invalidate()
    },
  })

  const downloadMutation = useMutation({
    mutationFn: async (modId: number) => {
      setDownloadingIds((prev) => new Set([...prev, modId]))
      return globalModsApi.download(modId)
    },
    onError: (_err, modId) => {
      setDownloadingIds((prev) => {
        const next = new Set(prev)
        next.delete(modId)
        return next
      })
    },
  })

  const handleDownloadDone = (modId: number) => {
    setDownloadingIds((prev) => {
      const next = new Set(prev)
      next.delete(modId)
      return next
    })
    invalidate()
  }

  return (
    <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-10">
      {/* Header */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary">
            <Package className="h-6 w-6" />
          </div>
          <div>
            <h1 className="text-2xl font-extrabold tracking-tight text-foreground sm:text-3xl">
              {t('title')}
            </h1>
            <p className="text-sm text-muted-foreground">{t('desc')}</p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => setBrowseOpen(true)}
            disabled={!webApiKeyConfigured}
            title={webApiKeyConfigured ? undefined : tc('workshop.noKey')}
          >
            <RefreshCw className="mr-1.5 h-4 w-4" />
            {t('browse')}
          </Button>
          <Button type="button" size="sm" onClick={() => setAddOpen(true)}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t('add')}
          </Button>
        </div>
      </div>

      {/* Mod list */}
      <div className="mt-8">
        {isLoading ? (
          <div className="flex items-center justify-center py-20 text-muted-foreground">
            <Loader2 className="mr-2 h-5 w-5 animate-spin" />
          </div>
        ) : mods.length === 0 ? (
          <Card className="rounded-2xl border-2 border-dashed shadow-none">
            <CardContent className="flex flex-col items-center gap-3 py-16 text-center">
              <div className="flex h-16 w-16 items-center justify-center rounded-3xl bg-muted text-4xl">
                📦
              </div>
              <p className="text-muted-foreground">{t('empty')}</p>
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-2">
            {mods.map((mod) => (
              <ModRow
                key={mod.id}
                mod={mod}
                downloading={downloadingIds.has(mod.id)}
                onDownload={() => downloadMutation.mutate(mod.id)}
                onDelete={() => setDeleteTarget(mod)}
                onDownloadDone={() => handleDownloadDone(mod.id)}
                t={t}
              />
            ))}
          </div>
        )}
      </div>

      {/* Add by ID dialog */}
      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t('addByWorkshopId')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1">
              <Label htmlFor="lib-workshop-id">{t('workshopIdLabel')}</Label>
              <Input
                id="lib-workshop-id"
                value={workshopId}
                onChange={(e) => setWorkshopId(e.target.value)}
                placeholder={t('workshopIdPlaceholder')}
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="lib-mod-name">{t('nameLabel')}</Label>
              <Input
                id="lib-mod-name"
                value={modName}
                onChange={(e) => setModName(e.target.value)}
                placeholder={t('namePlaceholder')}
              />
            </div>
            {addError && <p className="text-sm text-destructive">{addError}</p>}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setAddOpen(false)}>
              {t('cancel')}
            </Button>
            <Button
              type="button"
              onClick={() => addMutation.mutate()}
              disabled={addMutation.isPending || workshopId.trim() === ''}
            >
              {addMutation.isPending ? '...' : t('add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirm dialog */}
      <Dialog open={deleteTarget !== null} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t('remove')}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">{t('removeConfirm')}</p>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDeleteTarget(null)}>
              {t('cancel')}
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? '...' : t('confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Workshop browser */}
      <WorkshopBrowserDialog
        open={browseOpen}
        onOpenChange={setBrowseOpen}
        onAdded={() => invalidate()}
      />
    </div>
  )
}

// ── ModRow ────────────────────────────────────────────────────────────────────

interface ModRowProps {
  mod: Mod
  downloading: boolean
  onDownload: () => void
  onDelete: () => void
  onDownloadDone: () => void
  t: (key: string, params?: Record<string, string | number>) => string
}

function ModRow({ mod, downloading, onDownload, onDelete, onDownloadDone, t }: ModRowProps) {
  // showLog: visible when a download is active or just finished (until dismissed).
  const [showLog, setShowLog] = useState(false)
  const [collapsed, setCollapsed] = useState(false)
  const [logLines, setLogLines] = useState<string[]>([])
  const [logDone, setLogDone] = useState<'ok' | 'error' | null>(null)
  const logEndRef = useRef<HTMLDivElement>(null)

  // Open the log panel automatically when download starts; reset state.
  useEffect(() => {
    if (downloading) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setShowLog(true)
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setCollapsed(false)
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setLogLines([])
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setLogDone(null)
    }
  }, [downloading])

  // Connect SSE while a download is in progress.
  useEffect(() => {
    if (!downloading) return
    const es = new EventSource(globalModsApi.logStreamUrl(mod.id))
    es.addEventListener('log', (e: MessageEvent) => {
      setLogLines((prev) => [...prev, (e as MessageEvent).data])
    })
    es.addEventListener('done', (e: MessageEvent) => {
      const result = (e as MessageEvent).data === 'ok' ? 'ok' : 'error'
      setLogDone(result)
      onDownloadDone()
      es.close()
    })
    es.onerror = () => {
      setLogDone('error')
      onDownloadDone()
      es.close()
    }
    return () => es.close()
  }, [downloading, mod.id, onDownloadDone])

  // Auto-scroll to the latest log line.
  useEffect(() => {
    if (!collapsed) {
      logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
    }
  }, [logLines, collapsed])

  return (
    <div className="rounded-xl border border-border/60 bg-background/60 overflow-hidden">
      {/* Main row */}
      <div className="flex items-center justify-between gap-4 px-4 py-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-1.5 text-sm font-semibold">
            {mod.mod_name || mod.name || mod.workshop_id}
            {mod.downloaded ? (
              <CheckCircle2 size={14} className="text-success shrink-0" />
            ) : (
              <AlertTriangle size={14} className="text-warning shrink-0" />
            )}
          </div>
          <div className="font-mono text-xs text-muted-foreground">
            {mod.workshop_id}
            {mod.package_name ? ` · ${mod.package_name}` : ''}
            {mod.version ? ` · v${mod.version}` : ''}
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-1.5">
            <Badge
              variant={mod.downloaded ? 'success' : 'secondary'}
              className="text-[10px]"
            >
              {mod.downloaded ? t('status.downloaded') : t('status.notDownloaded')}
            </Badge>
            {typeof mod.server_count === 'number' && (
              <span className="text-[10px] text-muted-foreground">
                {mod.server_count > 0
                  ? t('serverCount', { count: mod.server_count })
                  : t('noServers')}
              </span>
            )}
            {(mod.tags ?? []).slice(0, 3).map((tag) => (
              <span
                key={tag}
                className="rounded-full border border-border/60 bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground"
              >
                {tag}
              </span>
            ))}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {/* Toggle log panel when a log is available */}
          {showLog && (
            <button
              type="button"
              onClick={() => setCollapsed((v) => !v)}
              className="text-muted-foreground hover:text-foreground"
              aria-label={collapsed ? t('log.expand') : t('log.collapse')}
            >
              {collapsed ? <ChevronDown size={15} /> : <ChevronUp size={15} />}
            </button>
          )}
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={onDownload}
            disabled={downloading}
            className="h-8 px-2.5 text-xs"
          >
            {downloading ? (
              <Loader2 size={13} className="animate-spin" />
            ) : (
              <Download size={13} />
            )}
            <span className="ml-1.5">
              {downloading ? t('downloading') : mod.downloaded ? t('redownload') : t('download')}
            </span>
          </Button>
          <button
            type="button"
            onClick={onDelete}
            className="text-muted-foreground hover:text-destructive"
            aria-label={t('remove')}
          >
            <Trash2 size={16} />
          </button>
        </div>
      </div>

      {/* Inline log panel */}
      {showLog && !collapsed && (
        <div className="border-t border-border/40 bg-muted/30">
          {/* Log header */}
          <div className="flex items-center justify-between gap-2 px-4 py-1.5">
            <div className="flex items-center gap-2">
              {downloading && !logDone && (
                <span className="flex items-center gap-1 text-[10px] font-medium text-primary">
                  <span className="h-1.5 w-1.5 rounded-full bg-primary animate-pulse" />
                  {t('log.live')}
                </span>
              )}
              {logDone === 'ok' && (
                <span className="flex items-center gap-1 text-[10px] font-medium text-success">
                  <CheckCircle2 size={11} />
                  {t('log.done')}
                </span>
              )}
              {logDone === 'error' && (
                <span className="flex items-center gap-1 text-[10px] font-medium text-destructive">
                  <AlertTriangle size={11} />
                  {t('log.failed')}
                </span>
              )}
            </div>
            <button
              type="button"
              onClick={() => setShowLog(false)}
              className="text-muted-foreground hover:text-foreground"
              aria-label={t('log.close')}
            >
              <X size={13} />
            </button>
          </div>
          {/* Log lines */}
          <div className="max-h-48 overflow-y-auto px-4 pb-3">
            {logLines.length === 0 ? (
              <p className="py-2 text-[11px] text-muted-foreground italic">{t('log.waiting')}</p>
            ) : (
              <pre className="whitespace-pre-wrap break-all font-mono text-[11px] leading-relaxed text-foreground/80">
                {logLines.join('\n')}
              </pre>
            )}
            <div ref={logEndRef} />
          </div>
        </div>
      )}
    </div>
  )
}
