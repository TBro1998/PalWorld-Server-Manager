'use client'

import React, { useEffect, useRef, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Trash2, AlertTriangle, CheckCircle2, LogIn } from 'lucide-react'
import { modsApi, steamApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'
import type { Server, Mod } from '@/types/server'
import { ServerLogsDialog } from '@/components/ServerLogsDialog'
import { SectionShell, PasswordInput, Placeholder, useServer } from './shared'

// Mods nav item for the manage page. Manual mod-list CRUD plus the "update mods"
// action that runs SteamCMD (download + deploy + config write). Reuses the
// steamcmd log stream via ServerLogsDialog for progress. Mod changes only take
// effect after a server restart, so a hint is shown while the server runs.
//
// NOTE: this section is intentionally self-saving (its mutations persist
// immediately); it does NOT participate in the settings draft / save bar.
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
      <ModsBody server={server} />
    </SectionShell>
  )
}

function ModsBody({ server }: { server: Server }) {
  const t = useTranslations('serverConfig')
  const queryClient = useQueryClient()
  const [workshopId, setWorkshopId] = useState('')
  const [name, setName] = useState('')
  const [logsOpen, setLogsOpen] = useState(false)
  // Set once any mod change is made this session; drives the restart hint.
  const [dirty, setDirty] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['mods', server.id] })
  const onError = (err: unknown) => {
    const e = err as { response?: { data?: { error?: string } } }
    setError(e.response?.data?.error ?? 'Request failed')
  }

  const { data, isLoading } = useQuery({
    queryKey: ['mods', server.id],
    queryFn: async () => (await modsApi.list(server.id)).data,
  })
  const mods: Mod[] = data?.mods ?? []

  // A cached Steam session is required before downloads can succeed; gate the
  // "update mods" action on it.
  const { data: steamStatus } = useQuery({
    queryKey: ['steamStatus'],
    queryFn: async () => (await steamApi.status()).data,
  })
  const sessionReady = steamStatus?.sessionReady ?? false

  const addMutation = useMutation({
    mutationFn: async () => {
      await modsApi.add(server.id, { workshopId: workshopId.trim(), name: name.trim() || undefined })
    },
    onSuccess: () => {
      setWorkshopId('')
      setName('')
      setError(null)
      setDirty(true)
      invalidate()
    },
    onError,
  })

  const toggleMutation = useMutation({
    mutationFn: async (modId: number) => {
      await modsApi.toggle(server.id, modId)
    },
    onSuccess: () => {
      setError(null)
      setDirty(true)
      invalidate()
    },
    onError,
  })

  const removeMutation = useMutation({
    mutationFn: async (modId: number) => {
      await modsApi.remove(server.id, modId)
    },
    onSuccess: () => {
      setError(null)
      setDirty(true)
      invalidate()
    },
    onError,
  })

  const updateMutation = useMutation({
    mutationFn: async () => {
      await modsApi.update(server.id)
    },
    onSuccess: () => {
      setError(null)
      setDirty(true)
      setLogsOpen(true)
      // The run is async; completion arrives via the log dialog's `done` event
      // (see onDone below), which refreshes the list and closes on full success.
    },
    onError,
  })

  const busy = addMutation.isPending || updateMutation.isPending

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-2">
        <p className="text-sm text-muted-foreground">{t('mods.hint')}</p>
        <Button
          type="button"
          size="sm"
          onClick={() => updateMutation.mutate()}
          disabled={busy || mods.length === 0 || !sessionReady}
        >
          {updateMutation.isPending ? t('mods.updating') : t('mods.update')}
        </Button>
      </div>

      <SteamAccountSection />

      {!sessionReady && (
        <p className="text-sm text-warning">{t('steam.status.needLogin')}</p>
      )}

      {server.status === 'running' && dirty && (
        <p className="text-sm text-warning">{t('mods.restartNeeded')}</p>
      )}

      {/* Add form */}
      <div className="flex flex-wrap items-end gap-2 rounded-2xl border-2 p-4 shadow-pal">
        <div className="space-y-1">
          <Label htmlFor="mod-workshop-id">{t('mods.workshopId')}</Label>
          <Input
            id="mod-workshop-id"
            value={workshopId}
            onChange={(e) => setWorkshopId(e.target.value)}
            placeholder="1234567890"
            className="max-w-[200px]"
          />
        </div>
        <div className="space-y-1">
          <Label htmlFor="mod-name">{t('mods.name')}</Label>
          <Input
            id="mod-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t('mods.namePlaceholder')}
            className="max-w-[200px]"
          />
        </div>
        <Button
          type="button"
          size="sm"
          onClick={() => addMutation.mutate()}
          disabled={busy || workshopId.trim() === ''}
        >
          {t('mods.add')}
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {/* Mod list */}
      {isLoading ? (
        <Placeholder className="min-h-[120px]">{t('loading')}</Placeholder>
      ) : mods.length === 0 ? (
        <Placeholder className="min-h-[120px]">{t('mods.empty')}</Placeholder>
      ) : (
        <div className="space-y-2">
          {mods.map((m) => (
            <div
              key={m.id}
              className="flex items-center justify-between gap-4 rounded-xl border border-border/60 bg-background/60 px-4 py-2.5"
            >
              <div className="min-w-0">
                <div className="flex items-center gap-1.5 text-sm font-medium">
                  {/* Prefer the Info.json ModName once downloaded; fall back to
                      the user-supplied name, then the workshop id. */}
                  {m.mod_name || m.name || m.workshop_id}
                  {m.package_name === '' && (
                    <AlertTriangle
                      size={14}
                      className="text-warning"
                      aria-label={t('mods.notDownloaded')}
                    />
                  )}
                </div>
                <div className="font-mono text-xs text-muted-foreground">
                  {m.workshop_id}
                  {m.package_name ? ` · ${m.package_name}` : ''}
                  {m.version ? ` · v${m.version}` : ''}
                </div>
                {(m.tags ?? []).length > 0 && (
                  <div className="mt-1 flex flex-wrap gap-1">
                    {(m.tags ?? []).map((tag) => (
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
                <Switch
                  checked={m.enabled}
                  onCheckedChange={() => toggleMutation.mutate(m.id)}
                  disabled={busy}
                />
                <button
                  type="button"
                  onClick={() => removeMutation.mutate(m.id)}
                  disabled={busy}
                  className="text-muted-foreground hover:text-destructive disabled:opacity-50"
                  aria-label={t('mods.remove')}
                >
                  <Trash2 size={16} />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      <ServerLogsDialog
        open={logsOpen}
        onOpenChange={setLogsOpen}
        server={server}
        kind="steamcmd"
        onDone={(success) => {
          // Async update finished: refresh the list (metadata backfilled) and,
          // when every mod succeeded, close the log dialog automatically.
          invalidate()
          if (success) setLogsOpen(false)
        }}
      />
    </div>
  )
}

// SteamAccountSection shows the configured Steam account status and drives the
// app-in login flow. Login runs `steamcmd +login` server-side; a Steam Guard
// code is requested in a second step only if needed. The password is only sent
// for the login request and is never stored by the tool.
function SteamAccountSection() {
  const t = useTranslations('serverConfig')
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [guardCode, setGuardCode] = useState('')
  const [needGuard, setNeedGuard] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [logLines, setLogLines] = useState<string[]>([])
  const [live, setLive] = useState(false)
  const [loggedIn, setLoggedIn] = useState(false)
  const logScrollRef = useRef<HTMLDivElement | null>(null)

  const { data: status } = useQuery({
    queryKey: ['steamStatus'],
    queryFn: async () => (await steamApi.status()).data,
  })

  const configured = !!status?.username
  const sessionReady = status?.sessionReady ?? false

  // Open the live steamcmd login SSE stream while the dialog is open, before the
  // user submits, so the first output lines are not missed. Backend emits named
  // `log` events (one per line) on the global login stream. The dialog is not
  // auto-closed on success, so the full run stays visible.
  useEffect(() => {
    if (!open) return

    // eslint-disable-next-line react-hooks/set-state-in-effect -- reset log UI state on (re)open
    setLogLines([])
    setLive(false)

    const es = new EventSource(steamApi.loginStreamUrl())
    es.addEventListener('log', (e) => {
      setLogLines((prev) => [...prev, (e as MessageEvent).data])
    })
    es.onopen = () => setLive(true)
    es.onerror = () => setLive(false)

    return () => {
      es.close()
      setLive(false)
    }
  }, [open])

  // Auto-scroll the log view to the bottom as new lines arrive.
  useEffect(() => {
    const el = logScrollRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [logLines])

  const openDialog = () => {
    setUsername(status?.username ?? '')
    setPassword('')
    setGuardCode('')
    setNeedGuard(false)
    setError(null)
    setLoggedIn(false)
    setOpen(true)
  }

  const loginMutation = useMutation({
    mutationFn: async () =>
      (
        await steamApi.login({
          username: username.trim(),
          password,
          guardCode: needGuard ? guardCode.trim() || undefined : undefined,
        })
      ).data,
    onSuccess: (data) => {
      switch (data.result) {
        case 'success':
          setPassword('')
          setGuardCode('')
          setError(null)
          setLoggedIn(true)
          queryClient.invalidateQueries({ queryKey: ['steamStatus'] })
          break
        case 'needGuard':
          setNeedGuard(true)
          setError(null)
          break
        case 'badCredentials':
          setError(t('steam.badCredentials'))
          break
        default:
          setError(t('steam.error'))
      }
    },
    onError: () => setError(t('steam.error')),
  })

  // Steam Guard mobile-authenticator accounts block on the user approving the
  // login in the phone app ("Waiting for confirmation..."); surface a clear hint
  // while that is pending so the user knows to check their phone.
  const awaitingConfirm =
    loginMutation.isPending &&
    !loggedIn &&
    logLines.some((l) => /confirm the login|waiting for confirmation/i.test(l))

  return (
    <div className="rounded-xl border border-border/60 bg-muted/40 px-4 py-3">
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="text-xs font-medium text-muted-foreground">{t('steam.title')}</div>
          {sessionReady ? (
            <div className="flex items-center gap-1.5 text-sm">
              <CheckCircle2 size={14} className="shrink-0 text-success" />
              <span className="truncate">{t('steam.status.loggedIn')}: {status?.username}</span>
            </div>
          ) : configured ? (
            <div className="flex items-center gap-1.5 text-sm text-warning">
              <AlertTriangle size={14} className="shrink-0" />
              <span className="truncate">{t('steam.status.needLogin')}</span>
            </div>
          ) : (
            <div className="text-sm text-muted-foreground">{t('steam.status.notConfigured')}</div>
          )}
        </div>
        <Button type="button" size="sm" variant="outline" onClick={openDialog} className="shrink-0">
          <LogIn size={14} className="mr-1" />
          {sessionReady ? t('steam.relogin') : t('steam.login')}
        </Button>
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('steam.title')}</DialogTitle>
          </DialogHeader>

          <div className="space-y-3">
            <div className="space-y-1">
              <Label htmlFor="steam-username">{t('steam.username')}</Label>
              <Input
                id="steam-username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoComplete="off"
                disabled={needGuard}
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="steam-password">{t('steam.password')}</Label>
              <PasswordInput id="steam-password" value={password} onChange={setPassword} />
            </div>

            {needGuard && (
              <div className="space-y-1">
                <Label htmlFor="steam-guard">{t('steam.guardCode')}</Label>
                <Input
                  id="steam-guard"
                  value={guardCode}
                  onChange={(e) => setGuardCode(e.target.value)}
                  autoComplete="off"
                />
                <p className="text-xs text-muted-foreground">{t('steam.guardHint')}</p>
              </div>
            )}

            <p className="text-xs text-muted-foreground">{t('steam.passwordNotStored')}</p>
            {awaitingConfirm && (
              <p className="flex items-center gap-1.5 rounded-md bg-warning/10 px-2 py-1.5 text-sm text-warning">
                <AlertTriangle size={14} className="shrink-0" />
                {t('steam.confirmOnPhone')}
              </p>
            )}
            {loggedIn && (
              <p className="flex items-center gap-1.5 text-sm text-success">
                <CheckCircle2 size={14} className="shrink-0" />
                {t('steam.success')}
              </p>
            )}
            {error && <p className="text-sm text-destructive">{error}</p>}

            <div className="space-y-1">
              <div className="flex items-center gap-2">
                <Label>{t('steam.output')}</Label>
                {live && (
                  <span className="flex items-center gap-1 text-xs font-normal text-success">
                    <span className="h-2 w-2 animate-pulse rounded-full bg-success" />
                    {t('steam.live')}
                  </span>
                )}
              </div>
              <div
                ref={logScrollRef}
                className="max-h-48 overflow-auto rounded-md bg-black/90 p-2 font-mono text-xs text-green-200"
              >
                {logLines.length === 0 ? (
                  <div className="text-zinc-500">{t('steam.outputEmpty')}</div>
                ) : (
                  logLines.map((line, i) => (
                    <div key={i} className="whitespace-pre-wrap break-all">
                      {line}
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpen(false)}
              disabled={loginMutation.isPending}
            >
              {t('cancel')}
            </Button>
            <Button
              type="button"
              onClick={() => loginMutation.mutate()}
              disabled={
                loginMutation.isPending ||
                loggedIn ||
                username.trim() === '' ||
                password === '' ||
                (needGuard && guardCode.trim() === '')
              }
            >
              {loginMutation.isPending ? t('steam.loggingIn') : t('steam.login')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
