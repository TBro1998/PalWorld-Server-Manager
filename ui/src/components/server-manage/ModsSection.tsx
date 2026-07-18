'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Trash2, AlertTriangle, RefreshCw, Plus, CheckCircle2 } from 'lucide-react'
import { modsApi, globalModsApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'
import type { ServerMod, Mod } from '@/types/server'
import { ServerLogsDialog } from '@/components/ServerLogsDialog'
import { SectionShell, Placeholder, useServer } from './shared'

// ModsSection: server-scoped mod management. Shows which global library mods
// are linked to this server, lets the user add/remove links and toggle enabled
// state, and deploys the staged files into the server directory.
//
// Workshop search and global downloads live in /mods (the global library page).
// This section intentionally has no Workshop browser.
export function ModsSection() {
  const t = useTranslations('serverManage')
  const { data: server } = useServer()

  if (!server) {
    return (
      <SectionShell title={t('sections.mods')} desc={t('modsSection.desc')} comingSoon={false}>
        <Placeholder className="min-h-[160px]">…</Placeholder>
      </SectionShell>
    )
  }

  return (
    <SectionShell title={t('sections.mods')} desc={t('modsSection.desc')} comingSoon={false}>
      <ModsBody serverId={server.id} serverStatus={server.status} />
    </SectionShell>
  )
}

function ModsBody({ serverId, serverStatus }: { serverId: number; serverStatus: string }) {
  const t = useTranslations('serverConfig')
  const queryClient = useQueryClient()
  const [logsOpen, setLogsOpen] = useState(false)
  const [addOpen, setAddOpen] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['serverMods', serverId] })
  const onError = (err: unknown) => {
    const e = err as { response?: { data?: { error?: string } } }
    setError(e.response?.data?.error ?? 'Request failed')
  }

  const { data, isLoading } = useQuery({
    queryKey: ['serverMods', serverId],
    queryFn: async () => (await modsApi.list(serverId)).data,
  })
  const serverMods: ServerMod[] = data?.mods ?? []

  const toggleMutation = useMutation({
    mutationFn: async (serverModId: number) => modsApi.toggle(serverId, serverModId),
    onSuccess: () => { setError(null); setDirty(true); invalidate() },
    onError,
  })

  const unlinkMutation = useMutation({
    mutationFn: async (serverModId: number) => modsApi.unlink(serverId, serverModId),
    onSuccess: () => { setError(null); setDirty(true); invalidate() },
    onError,
  })

  const deployMutation = useMutation({
    mutationFn: async () => modsApi.deploy(serverId),
    onSuccess: () => {
      setError(null)
      setDirty(true)
      setLogsOpen(true)
    },
    onError,
  })

  const hasVersionMismatch = serverMods.some((m) => m.version_mismatch)
  const busy = toggleMutation.isPending || deployMutation.isPending

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between gap-2">
        <p className="text-sm text-muted-foreground">{t('mods.hint')}</p>
        <div className="flex gap-2">
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => setAddOpen(true)}
          >
            <Plus className="mr-1.5 h-3.5 w-3.5" />
            {t('mods.addFromLibrary')}
          </Button>
          <Button
            type="button"
            size="sm"
            onClick={() => deployMutation.mutate()}
            disabled={busy || serverMods.length === 0}
          >
            <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
            {deployMutation.isPending ? t('mods.deploying') : t('mods.deploy')}
          </Button>
        </div>
      </div>

      {hasVersionMismatch && (
        <p className="flex items-center gap-1.5 text-sm text-warning">
          <AlertTriangle size={14} className="shrink-0" />
          {t('mods.versionMismatch')}
        </p>
      )}

      {serverStatus === 'running' && dirty && (
        <p className="text-sm text-warning">{t('mods.restartNeeded')}</p>
      )}

      {error && <p className="text-sm text-destructive">{error}</p>}

      {/* Mod list */}
      {isLoading ? (
        <Placeholder className="min-h-[120px]">{t('loading')}</Placeholder>
      ) : serverMods.length === 0 ? (
        <Placeholder className="min-h-[120px]">{t('mods.empty')}</Placeholder>
      ) : (
        <div className="space-y-2">
          {serverMods.map((sm) => (
            <ServerModRow
              key={sm.id}
              sm={sm}
              busy={busy}
              onToggle={() => toggleMutation.mutate(sm.id)}
              onUnlink={() => unlinkMutation.mutate(sm.id)}
              t={t}
            />
          ))}
        </div>
      )}

      {/* Deploy logs dialog */}
      <ServerLogsDialog
        open={logsOpen}
        onOpenChange={setLogsOpen}
        server={{ id: serverId } as never}
        kind="steamcmd"
        onDone={(success) => {
          invalidate()
          if (success) setLogsOpen(false)
        }}
      />

      {/* Add from library dialog */}
      <AddFromLibraryDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        serverId={serverId}
        linkedModIds={new Set(serverMods.map((sm) => sm.mod_id))}
        onLinked={() => { invalidate(); setDirty(true) }}
      />
    </div>
  )
}

function ServerModRow({
  sm,
  busy,
  onToggle,
  onUnlink,
  t,
}: {
  sm: ServerMod
  busy: boolean
  onToggle: () => void
  onUnlink: () => void
  t: (key: string) => string
}) {
  return (
    <div className="flex items-center justify-between gap-4 rounded-xl border border-border/60 bg-background/60 px-4 py-2.5">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-1.5 text-sm font-medium">
          {sm.mod_name || sm.name || sm.workshop_id}
          {!sm.downloaded && (
            <span title={t('mods.notDownloaded')}>
              <AlertTriangle size={14} className="text-warning shrink-0" />
            </span>
          )}
          {sm.version_mismatch && (
            <span title={t('mods.versionMismatch')}>
              <RefreshCw size={14} className="text-warning shrink-0" />
            </span>
          )}
        </div>
        <div className="font-mono text-xs text-muted-foreground">
          {sm.workshop_id}
          {sm.package_name ? ` · ${sm.package_name}` : ''}
          {sm.version ? ` · v${sm.version}` : ''}
          {sm.deployed_version && sm.deployed_version !== sm.version && (
            <span className="ml-1 text-warning">
              ({t('mods.deployedVersion')}: v{sm.deployed_version})
            </span>
          )}
        </div>
        {(sm.tags ?? []).length > 0 && (
          <div className="mt-1 flex flex-wrap gap-1">
            {(sm.tags ?? []).map((tag) => (
              <span
                key={tag}
                className="rounded-full border border-border/60 bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground"
              >
                {tag}
              </span>
            ))}
          </div>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-3">
        <Switch checked={sm.enabled} onCheckedChange={onToggle} disabled={busy} />
        <button
          type="button"
          onClick={onUnlink}
          disabled={busy}
          className="text-muted-foreground hover:text-destructive disabled:opacity-50"
          aria-label={t('mods.remove')}
        >
          <Trash2 size={16} />
        </button>
      </div>
    </div>
  )
}

// AddFromLibraryDialog shows all downloaded global mods not yet linked to
// this server and lets the user pick one or more to link.
function AddFromLibraryDialog({
  open,
  onOpenChange,
  serverId,
  linkedModIds,
  onLinked,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  serverId: number
  linkedModIds: Set<number>
  onLinked: () => void
}) {
  const t = useTranslations('serverConfig')
  const queryClient = useQueryClient()
  const [error, setError] = useState<string | null>(null)

  const { data } = useQuery({
    queryKey: ['globalMods'],
    queryFn: async () => (await globalModsApi.list()).data,
    enabled: open,
  })
  const allMods: Mod[] = (data?.mods ?? []).filter(
    (m) => m.downloaded && !linkedModIds.has(m.id),
  )

  const linkMutation = useMutation({
    mutationFn: async (modId: number) => modsApi.link(serverId, { modId }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['serverMods', serverId] })
      onLinked()
      setError(null)
    },
    onError: (err: unknown) => {
      const e = err as { response?: { data?: { error?: string } } }
      setError(e.response?.data?.error ?? 'Failed to link mod')
    },
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('mods.addFromLibrary')}</DialogTitle>
        </DialogHeader>

        {allMods.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {t('mods.noDownloadedMods')}
          </p>
        ) : (
          <div className="max-h-80 space-y-1.5 overflow-y-auto">
            {allMods.map((mod) => (
              <div
                key={mod.id}
                className="flex items-center justify-between gap-3 rounded-xl border border-border/50 px-3 py-2"
              >
                <div className="min-w-0">
                  <div className="flex items-center gap-1.5 text-sm font-medium">
                    {mod.mod_name || mod.name || mod.workshop_id}
                    <CheckCircle2 size={13} className="text-success shrink-0" />
                  </div>
                  <div className="font-mono text-xs text-muted-foreground">
                    {mod.workshop_id}
                    {mod.version ? ` · v${mod.version}` : ''}
                  </div>
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  className="h-7 shrink-0 px-2.5 text-xs"
                  disabled={linkMutation.isPending}
                  onClick={() => linkMutation.mutate(mod.id)}
                >
                  <Plus size={12} className="mr-1" />
                  {t('mods.addFromLibrary')}
                </Button>
              </div>
            ))}
          </div>
        )}

        {error && <p className="text-sm text-destructive">{error}</p>}

        <div className="flex justify-end">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t('cancel')}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
