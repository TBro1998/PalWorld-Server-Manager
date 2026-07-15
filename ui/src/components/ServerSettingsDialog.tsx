'use client'

import React, { useEffect, useMemo, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from './ui/dialog'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { Button } from './ui/button'
import { Switch } from './ui/switch'
import { Select } from './ui/select'
import { Textarea } from './ui/textarea'
import { Eye, EyeOff, Trash2, AlertTriangle, CheckCircle2, LogIn } from 'lucide-react'
import { serversApi, modsApi, steamApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'
import type { Server, ConfigParamDef, LaunchArgs, Mod } from '@/types/server'
import { ServerLogsDialog } from './ServerLogsDialog'

interface ServerSettingsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  server: Server | null
}

const CATEGORIES = ['performances', 'serverManagement', 'features', 'gameBalances'] as const

// OptionSettings keys promoted into the Basics tab. They are edited there (and
// surfaced on the server card) instead of the generic serverManagement list, so
// they are filtered out of that category to avoid duplicate editors.
const BASICS_INI_KEYS = new Set<string>([
  'ServerPassword',
  'AdminPassword',
  'ServerDescription',
  'RESTAPIEnabled',
  'RESTAPIPort',
])

export function ServerSettingsDialog({ open, onOpenChange, server }: ServerSettingsDialogProps) {
  const t = useTranslations('serverConfig')
  const queryClient = useQueryClient()

  const [tab, setTab] = useState<string>('basics')
  const [name, setName] = useState('')
  const [installPath, setInstallPath] = useState('')
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [launchArgs, setLaunchArgs] = useState<LaunchArgs>({})
  const [rawText, setRawText] = useState('')
  const [rawMode, setRawMode] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const { data: schemaData } = useQuery({
    queryKey: ['configSchema'],
    queryFn: async () => (await serversApi.configSchema()).data,
    staleTime: Infinity,
  })

  const { data: config, isLoading } = useQuery({
    queryKey: ['serverConfig', server?.id],
    queryFn: async () => (await serversApi.getConfig(server!.id)).data,
    enabled: open && !!server,
  })

  // Initialize local editing state when config arrives / dialog opens.
  useEffect(() => {
    if (config && open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- seed edit state from fetched config on open
      setSettings({ ...config.settings })
      setLaunchArgs({ ...config.launchArgs })
      setRawText(config.raw)
      setRawMode(false)
      setError(null)
    }
  }, [config, open])

  // Prefill name / install path from the selected server whenever the dialog opens.
  useEffect(() => {
    if (server && open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- seed edit state from selected server on open
      setName(server.name)
      setInstallPath(server.install_path)
      setTab('basics')
      setError(null)
    }
  }, [server, open])

  const paramsByCategory = useMemo(() => {
    const map: Record<string, ConfigParamDef[]> = {}
    for (const p of schemaData?.params ?? []) {
      ;(map[p.category] ??= []).push(p)
    }
    return map
  }, [schemaData])

  const paramByKey = useMemo(() => {
    const map: Record<string, ConfigParamDef> = {}
    for (const p of schemaData?.params ?? []) map[p.key] = p
    return map
  }, [schemaData])

  const iniValue = (key: string) => settings[key] ?? paramByKey[key]?.default ?? ''

  // R7 save orchestration: metadata (update) then INI config (updateConfig).
  const saveMutation = useMutation({
    mutationFn: async () => {
      if (!server) return
      // 1) Structured mode: sync the outward-facing name into INI ServerName.
      let outSettings = settings
      if (!rawMode) {
        outSettings = { ...settings, ServerName: name }
      }
      // 2) Metadata: name / install path / launch args.
      await serversApi.update(server.id, { name, installPath, launchArgs })
      // 3) INI config only makes sense once installed (backend rejects otherwise).
      if (server.installed) {
        await serversApi.updateConfig(
          server.id,
          rawMode ? { raw: rawText } : { settings: outSettings, launchArgs },
        )
      }
    },
    onSuccess: () => {
      if (server) {
        queryClient.invalidateQueries({ queryKey: ['serverConfig', server.id] })
      }
      queryClient.invalidateQueries({ queryKey: ['servers'] })
      onOpenChange(false)
    },
    onError: (err: unknown) => {
      const e = err as { response?: { data?: { error?: string } } }
      setError(e.response?.data?.error ?? 'Save failed')
    },
  })

  const handleSave = () => {
    setError(null)
    saveMutation.mutate()
  }

  const paramLabel = (key: string) => {
    const l = t(`params.${key}.label`)
    return l.includes('params.') ? key : l
  }
  const paramDesc = (key: string) => {
    const d = t(`params.${key}.desc`)
    return d.includes('params.') ? '' : d
  }

  const setSetting = (key: string, value: string) => {
    setSettings((prev) => ({ ...prev, [key]: value }))
    setRawMode(false)
  }

  const renderControl = (p: ConfigParamDef) => {
    const value = settings[p.key] ?? p.default
    switch (p.type) {
      case 'bool':
        return (
          <Switch
            checked={value === 'True'}
            onCheckedChange={(c) => setSetting(p.key, c ? 'True' : 'False')}
          />
        )
      case 'enum':
        return (
          <Select value={value} onChange={(e) => setSetting(p.key, e.target.value)} className="max-w-[220px]">
            {(p.options ?? []).map((opt) => (
              <option key={opt} value={opt}>
                {opt}
              </option>
            ))}
          </Select>
        )
      case 'int':
      case 'float':
        return (
          <Input
            type="number"
            step={p.type === 'float' ? 'any' : '1'}
            value={value}
            onChange={(e) => setSetting(p.key, e.target.value)}
            className="max-w-[220px]"
          />
        )
      default:
        return (
          <Input
            value={value}
            onChange={(e) => setSetting(p.key, e.target.value)}
            className="max-w-[220px]"
          />
        )
    }
  }

  const setLA = (patch: Partial<LaunchArgs>) => setLaunchArgs((prev) => ({ ...prev, ...patch }))
  const numOrUndef = (v: string) => (v === '' ? undefined : Number(v))

  const pathChanged = server ? installPath !== server.install_path : false

  const tabs = ['basics', 'mods', ...CATEGORIES, 'launch', 'raw']

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>
            {t('title')}
            {server ? ` — ${name || server.name}` : ''}
          </DialogTitle>
        </DialogHeader>

        {!server?.installed && (
          <p className="text-sm text-warning">{t('notInstalledHint')}</p>
        )}

        {/* Tab bar */}
        <div className="flex flex-wrap gap-1 border-b">
          {tabs.map((tb) => (
            <button
              key={tb}
              type="button"
              onClick={() => setTab(tb)}
              className={
                'px-3 py-1.5 text-sm rounded-t-md ' +
                (tab === tb
                  ? 'bg-primary text-primary-foreground'
                  : 'text-muted-foreground hover:bg-muted')
              }
            >
              {t(`tabs.${tb}`)}
            </button>
          ))}
        </div>

        <div className="max-h-[55vh] overflow-y-auto pr-1">
          {tab === 'basics' ? (
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="settings-name">{t('basics.name')}</Label>
                <Input id="settings-name" value={name} onChange={(e) => setName(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="settings-path">{t('basics.path')}</Label>
                <Input
                  id="settings-path"
                  value={installPath}
                  onChange={(e) => setInstallPath(e.target.value)}
                />
                {pathChanged && (
                  <p className="text-sm text-warning">
                    {t('basics.pathChangedHint')}
                  </p>
                )}
              </div>
              <LaunchNumber
                label={t('basics.port')}
                value={launchArgs.port}
                onChange={(v) => setLA({ port: v })}
                numOrUndef={numOrUndef}
              />

              <div className="space-y-2">
                <Label htmlFor="settings-serverpassword">{paramLabel('ServerPassword')}</Label>
                <PasswordInput
                  id="settings-serverpassword"
                  value={iniValue('ServerPassword')}
                  onChange={(v) => setSetting('ServerPassword', v)}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="settings-adminpassword">{paramLabel('AdminPassword')}</Label>
                <PasswordInput
                  id="settings-adminpassword"
                  value={iniValue('AdminPassword')}
                  onChange={(v) => setSetting('AdminPassword', v)}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="settings-serverdesc">{paramLabel('ServerDescription')}</Label>
                <Textarea
                  id="settings-serverdesc"
                  value={iniValue('ServerDescription')}
                  onChange={(e) => setSetting('ServerDescription', e.target.value)}
                  className="min-h-[72px]"
                />
              </div>

              <div className="flex items-center justify-between gap-4 border-b border-dashed pb-2">
                <Label>{paramLabel('RESTAPIEnabled')}</Label>
                <Switch
                  checked={iniValue('RESTAPIEnabled') === 'True'}
                  onCheckedChange={(c) => setSetting('RESTAPIEnabled', c ? 'True' : 'False')}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="settings-restport">{paramLabel('RESTAPIPort')}</Label>
                <Input
                  id="settings-restport"
                  type="number"
                  step="1"
                  value={iniValue('RESTAPIPort')}
                  onChange={(e) => setSetting('RESTAPIPort', e.target.value)}
                  className="max-w-[220px]"
                />
              </div>
            </div>
          ) : tab === 'mods' ? (
            server ? <ModsSection server={server} /> : null
          ) : isLoading ? (
            <p className="text-sm text-muted-foreground py-6 text-center">{t('loading')}</p>
          ) : CATEGORIES.includes(tab as (typeof CATEGORIES)[number]) ? (
            <div className="space-y-3">
              {(paramsByCategory[tab] ?? [])
                .filter((p) => p.key !== 'ServerName' && !BASICS_INI_KEYS.has(p.key))
                .map((p) => (
                <div key={p.key} className="flex items-start justify-between gap-4 border-b border-dashed pb-2">
                  <div className="min-w-0">
                    <div className="text-sm font-medium">{paramLabel(p.key)}</div>
                    <div className="text-xs text-muted-foreground font-mono">{p.key}</div>
                    {paramDesc(p.key) && (
                      <div className="text-xs text-muted-foreground mt-0.5">{paramDesc(p.key)}</div>
                    )}
                  </div>
                  <div className="shrink-0">{renderControl(p)}</div>
                </div>
              ))}
            </div>
          ) : tab === 'launch' ? (
            <div className="space-y-3">
              <LaunchNumber label={t('launch.players')} value={launchArgs.players} onChange={(v) => setLA({ players: v })} numOrUndef={numOrUndef} />
              <LaunchToggle label={t('launch.usePerfThreads')} checked={!!launchArgs.usePerfThreads} onChange={(c) => setLA({ usePerfThreads: c })} />
              <LaunchToggle label={t('launch.noAsyncLoadingThread')} checked={!!launchArgs.noAsyncLoadingThread} onChange={(c) => setLA({ noAsyncLoadingThread: c })} />
              <LaunchToggle label={t('launch.useMultithreadForDS')} checked={!!launchArgs.useMultithreadForDS} onChange={(c) => setLA({ useMultithreadForDS: c })} />
              <LaunchNumber label={t('launch.numberOfWorkerThreadsServer')} value={launchArgs.numberOfWorkerThreadsServer} onChange={(v) => setLA({ numberOfWorkerThreadsServer: v })} numOrUndef={numOrUndef} />
              <LaunchNumber label={t('launch.queryPort')} value={launchArgs.queryPort} onChange={(v) => setLA({ queryPort: v })} numOrUndef={numOrUndef} placeholder="27015" />
              <LaunchToggle label={t('launch.publicLobby')} checked={!!launchArgs.publicLobby} onChange={(c) => setLA({ publicLobby: c })} />
            </div>
          ) : (
            <div className="space-y-2">
              <p className="text-xs text-muted-foreground">{t('rawHint')}</p>
              <Textarea
                value={rawText}
                onChange={(e) => {
                  setRawText(e.target.value)
                  setRawMode(true)
                }}
                className="min-h-[300px] font-mono text-xs"
                spellCheck={false}
              />
            </div>
          )}
        </div>

        {error && <p className="text-sm text-destructive">{error}</p>}
        {rawMode && <p className="text-xs text-warning">{t('rawModeActive')}</p>}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={saveMutation.isPending}>
            {t('cancel')}
          </Button>
          <Button type="button" onClick={handleSave} disabled={saveMutation.isPending}>
            {saveMutation.isPending ? t('saving') : t('save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ModsSection is the Mods tab: manual mod-list CRUD plus the "update mods"
// action that runs SteamCMD (download + deploy + config write). It reuses the
// steamcmd log stream via ServerLogsDialog for progress. Changes only take
// effect after a server restart, so a hint is shown while the server runs.
function ModsSection({ server }: { server: Server }) {
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
      // Metadata (version / package name) is backfilled asynchronously; refetch
      // shortly after so the list reflects the results once downloads finish.
      setTimeout(invalidate, 3000)
    },
    onError,
  })

  const busy = addMutation.isPending || updateMutation.isPending

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">{t('mods.hint')}</p>
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
        <p className="text-xs text-warning">{t('steam.status.needLogin')}</p>
      )}

      {server.status === 'running' && dirty && (
        <p className="text-sm text-warning">{t('mods.restartNeeded')}</p>
      )}

      {/* Add form */}
      <div className="flex flex-wrap items-end gap-2 border-b border-dashed pb-3">
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
        <p className="text-sm text-muted-foreground py-4 text-center">{t('loading')}</p>
      ) : mods.length === 0 ? (
        <p className="text-sm text-muted-foreground py-4 text-center">{t('mods.empty')}</p>
      ) : (
        <div className="space-y-2">
          {mods.map((m) => (
            <div key={m.id} className="flex items-center justify-between gap-4 border-b border-dashed pb-2">
              <div className="min-w-0">
                <div className="flex items-center gap-1.5 text-sm font-medium">
                  {m.name || m.workshop_id}
                  {m.package_name === '' && (
                    <AlertTriangle
                      size={14}
                      className="text-warning"
                      aria-label={t('mods.notDownloaded')}
                    />
                  )}
                </div>
                <div className="text-xs text-muted-foreground font-mono">
                  {m.workshop_id}
                  {m.version ? ` · v${m.version}` : ''}
                </div>
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

      <ServerLogsDialog open={logsOpen} onOpenChange={setLogsOpen} server={server} kind="steamcmd" />
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
  const [loginLog, setLoginLog] = useState('')
  const [loggedIn, setLoggedIn] = useState(false)

  const { data: status } = useQuery({
    queryKey: ['steamStatus'],
    queryFn: async () => (await steamApi.status()).data,
  })

  const configured = !!status?.username
  const sessionReady = status?.sessionReady ?? false

  const openDialog = () => {
    setUsername(status?.username ?? '')
    setPassword('')
    setGuardCode('')
    setNeedGuard(false)
    setError(null)
    setLoginLog('')
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
      setLoginLog(data.log ?? '')
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

  return (
    <div className="rounded-md bg-muted px-3 py-2">
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="text-xs font-medium text-muted-foreground">{t('steam.title')}</div>
          {sessionReady ? (
            <div className="flex items-center gap-1.5 text-sm">
              <CheckCircle2 size={14} className="text-success shrink-0" />
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
            {loggedIn && (
              <p className="flex items-center gap-1.5 text-sm text-success">
                <CheckCircle2 size={14} className="shrink-0" />
                {t('steam.success')}
              </p>
            )}
            {error && <p className="text-sm text-destructive">{error}</p>}

            {loginLog && (
              <div className="space-y-1">
                <Label>{t('steam.output')}</Label>
                <pre className="max-h-48 overflow-auto rounded-md bg-black/90 p-2 text-xs text-green-200 whitespace-pre-wrap break-words">
                  {loginLog}
                </pre>
              </div>
            )}
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

// PasswordInput is a text input masked by default with a show/hide toggle.
function PasswordInput({
  id,
  value,
  onChange,
}: {
  id?: string
  value: string
  onChange: (v: string) => void
}) {
  const [show, setShow] = useState(false)
  return (
    <div className="relative">
      <Input
        id={id}
        type={show ? 'text' : 'password'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="pr-9"
      />
      <button
        type="button"
        tabIndex={-1}
        onClick={() => setShow((s) => !s)}
        className="absolute inset-y-0 right-0 flex items-center px-2.5 text-muted-foreground hover:text-foreground"
      >
        {show ? <EyeOff size={16} /> : <Eye size={16} />}
      </button>
    </div>
  )
}

function LaunchToggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (c: boolean) => void }) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-dashed pb-2">
      <Label>{label}</Label>
      <Switch checked={checked} onCheckedChange={onChange} />
    </div>
  )
}

function LaunchNumber({
  label,
  value,
  onChange,
  numOrUndef,
  placeholder,
}: {
  label: string
  value: number | undefined
  onChange: (v: number | undefined) => void
  numOrUndef: (v: string) => number | undefined
  placeholder?: string
}) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-dashed pb-2">
      <Label>{label}</Label>
      <Input
        type="number"
        value={value ?? ''}
        placeholder={placeholder}
        onChange={(e) => onChange(numOrUndef(e.target.value))}
        className="max-w-[220px]"
      />
    </div>
  )
}
