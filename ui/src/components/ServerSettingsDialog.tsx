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
import { Eye, EyeOff } from 'lucide-react'
import { serversApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'
import type { Server, ConfigParamDef, LaunchArgs } from '@/types/server'

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

  const tabs = ['basics', ...CATEGORIES, 'launch', 'raw']

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
          <p className="text-sm text-amber-600 dark:text-amber-500">{t('notInstalledHint')}</p>
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
                  <p className="text-sm text-amber-600 dark:text-amber-500">
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
        {rawMode && <p className="text-xs text-amber-600 dark:text-amber-500">{t('rawModeActive')}</p>}

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
