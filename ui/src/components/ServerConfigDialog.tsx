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
import { serversApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'
import type { Server, ConfigParamDef, LaunchArgs, UpdateServerConfigData } from '@/types/server'

interface ServerConfigDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  server: Server | null
}

const CATEGORIES = ['performances', 'serverManagement', 'features', 'gameBalances'] as const

export function ServerConfigDialog({ open, onOpenChange, server }: ServerConfigDialogProps) {
  const t = useTranslations('serverConfig')
  const queryClient = useQueryClient()

  const [tab, setTab] = useState<string>('performances')
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
      setSettings({ ...config.settings })
      setLaunchArgs({ ...config.launchArgs })
      setRawText(config.raw)
      setRawMode(false)
      setError(null)
    }
  }, [config, open])

  const paramsByCategory = useMemo(() => {
    const map: Record<string, ConfigParamDef[]> = {}
    for (const p of schemaData?.params ?? []) {
      ;(map[p.category] ??= []).push(p)
    }
    return map
  }, [schemaData])

  const saveMutation = useMutation({
    mutationFn: (data: UpdateServerConfigData) => serversApi.updateConfig(server!.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['serverConfig', server?.id] })
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
    const data: UpdateServerConfigData = { launchArgs }
    if (rawMode) {
      data.raw = rawText
    } else {
      data.settings = settings
    }
    saveMutation.mutate(data)
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

  const tabs = [...CATEGORIES, 'launch', 'raw']

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>
            {t('title')}
            {server ? ` — ${server.name}` : ''}
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
          {isLoading ? (
            <p className="text-sm text-muted-foreground py-6 text-center">{t('loading')}</p>
          ) : CATEGORIES.includes(tab as (typeof CATEGORIES)[number]) ? (
            <div className="space-y-3">
              {(paramsByCategory[tab] ?? []).map((p) => (
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
              <LaunchNumber label={t('launch.port')} value={launchArgs.port} onChange={(v) => setLA({ port: v })} numOrUndef={numOrUndef} />
              <LaunchNumber label={t('launch.players')} value={launchArgs.players} onChange={(v) => setLA({ players: v })} numOrUndef={numOrUndef} />
              <LaunchToggle label={t('launch.usePerfThreads')} checked={!!launchArgs.usePerfThreads} onChange={(c) => setLA({ usePerfThreads: c })} />
              <LaunchToggle label={t('launch.noAsyncLoadingThread')} checked={!!launchArgs.noAsyncLoadingThread} onChange={(c) => setLA({ noAsyncLoadingThread: c })} />
              <LaunchToggle label={t('launch.useMultithreadForDS')} checked={!!launchArgs.useMultithreadForDS} onChange={(c) => setLA({ useMultithreadForDS: c })} />
              <LaunchNumber label={t('launch.numberOfWorkerThreadsServer')} value={launchArgs.numberOfWorkerThreadsServer} onChange={(v) => setLA({ numberOfWorkerThreadsServer: v })} numOrUndef={numOrUndef} />
              <LaunchToggle label={t('launch.publicLobby')} checked={!!launchArgs.publicLobby} onChange={(c) => setLA({ publicLobby: c })} />
              <div className="flex items-center justify-between gap-4 border-b border-dashed pb-2">
                <Label>{t('launch.publicIP')}</Label>
                <Input
                  value={launchArgs.publicIP ?? ''}
                  onChange={(e) => setLA({ publicIP: e.target.value || undefined })}
                  className="max-w-[220px]"
                />
              </div>
              <LaunchNumber label={t('launch.publicPort')} value={launchArgs.publicPort} onChange={(v) => setLA({ publicPort: v })} numOrUndef={numOrUndef} />
              <div className="flex items-center justify-between gap-4 border-b border-dashed pb-2">
                <Label>{t('launch.logFormat')}</Label>
                <Select
                  value={launchArgs.logFormat ?? ''}
                  onChange={(e) => setLA({ logFormat: e.target.value || undefined })}
                  className="max-w-[220px]"
                >
                  <option value="">--</option>
                  <option value="text">text</option>
                  <option value="json">json</option>
                </Select>
              </div>
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
          <Button type="button" onClick={handleSave} disabled={saveMutation.isPending || !server?.installed}>
            {saveMutation.isPending ? t('saving') : t('save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
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
}: {
  label: string
  value: number | undefined
  onChange: (v: number | undefined) => void
  numOrUndef: (v: string) => number | undefined
}) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-dashed pb-2">
      <Label>{label}</Label>
      <Input
        type="number"
        value={value ?? ''}
        onChange={(e) => onChange(numOrUndef(e.target.value))}
        className="max-w-[220px]"
      />
    </div>
  )
}
