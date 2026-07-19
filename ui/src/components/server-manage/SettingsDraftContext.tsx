'use client'

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { serversApi } from '@/lib/api'
import type { ConfigParamDef, LaunchArgs, Server } from '@/types/server'

// SettingsDraftContext lifts the server-config editing state (previously local
// to ServerSettingsDialog) up to the manage page, so the inline config sub-pages
// (Basics / Game / Launch / Raw) share ONE draft and ONE save bar. Editing any
// field on any config page marks the draft dirty; saving commits everything
// atomically, mirroring the old dialog's save semantics.

interface SettingsDraft {
  // Server metadata (editable regardless of install state).
  name: string
  installPath: string
  // INI OptionSettings (structured mode).
  settings: Record<string, string>
  launchArgs: LaunchArgs
  // Raw OptionSettings text (raw mode).
  rawText: string
  rawMode: boolean

  // Schema for the structured editors.
  params: ConfigParamDef[]
  paramByKey: Record<string, ConfigParamDef>

  // Derived state.
  installed: boolean
  isRunning: boolean
  loading: boolean
  isDirty: boolean
  dirtyCount: number
  saving: boolean
  error: string | null

  // Setters (any of these marks the draft dirty).
  setName: (v: string) => void
  setInstallPath: (v: string) => void
  setSetting: (key: string, value: string) => void
  setLaunch: (patch: Partial<LaunchArgs>) => void
  setRaw: (text: string) => void

  // Actions.
  save: () => void
  discard: () => void

  // Convenience: current INI value with schema-default fallback.
  iniValue: (key: string) => string
}

const Ctx = createContext<SettingsDraft | null>(null)

// Compare two settings maps over the union of their keys.
function countChangedSettings(a: Record<string, string>, b: Record<string, string>): number {
  let n = 0
  const keys = new Set([...Object.keys(a), ...Object.keys(b)])
  for (const k of keys) {
    if ((a[k] ?? '') !== (b[k] ?? '')) n++
  }
  return n
}

function countChangedLaunch(a: LaunchArgs, b: LaunchArgs): number {
  let n = 0
  const keys = new Set([...Object.keys(a), ...Object.keys(b)]) as Set<keyof LaunchArgs>
  for (const k of keys) {
    if (a[k] !== b[k]) n++
  }
  return n
}

export function SettingsDraftProvider({
  server,
  children,
}: {
  server: Server | null
  children: React.ReactNode
}) {
  const queryClient = useQueryClient()
  const serverId = server?.id

  const { data: schemaData } = useQuery({
    queryKey: ['configSchema'],
    queryFn: async () => (await serversApi.configSchema()).data,
    staleTime: Infinity,
  })

  const { data: config, isLoading } = useQuery({
    queryKey: ['serverConfig', serverId],
    queryFn: async () => (await serversApi.getConfig(serverId!)).data,
    enabled: serverId !== undefined,
    // Fetch once and keep; the save flow resets the baseline manually so a
    // refetch after invalidation never clobbers in-flight edits.
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  })

  // Draft (edited) state.
  const [name, setNameState] = useState('')
  const [installPath, setInstallPathState] = useState('')
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [launchArgs, setLaunchArgs] = useState<LaunchArgs>({})
  const [rawText, setRawText] = useState('')
  const [rawMode, setRawMode] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Baseline (last-saved / loaded) snapshot, for dirty comparison. Kept in state
  // (not a ref) so the dirty memo recomputes when it changes; the seed effect is
  // guarded by `seededFor` so setters never re-trigger seeding.
  const [baseline, setBaseline] = useState({
    name: '',
    installPath: '',
    settings: {} as Record<string, string>,
    launchArgs: {} as LaunchArgs,
    raw: '',
  })

  // Seed draft + baseline once per server. A ref guards against re-seeding on
  // background refetches so user edits are never clobbered.
  const seededFor = useRef<number | null>(null)
  useEffect(() => {
    if (!server || serverId === undefined) return
    if (!config) return
    if (seededFor.current === serverId) return
    seededFor.current = serverId

    setNameState(server.name)
    setInstallPathState(server.install_path)
    setSettings({ ...config.settings })
    setLaunchArgs({ ...config.launchArgs })
    setRawText(config.raw)
    setRawMode(false)
    setError(null)

    setBaseline({
      name: server.name,
      installPath: server.install_path,
      settings: { ...config.settings },
      launchArgs: { ...config.launchArgs },
      raw: config.raw,
    })
  }, [config, server, serverId])

  const params = useMemo(() => schemaData?.params ?? [], [schemaData])
  const paramByKey = useMemo(() => {
    const map: Record<string, ConfigParamDef> = {}
    for (const p of params) map[p.key] = p
    return map
  }, [params])

  const iniValue = useCallback(
    (key: string) => settings[key] ?? paramByKey[key]?.default ?? '',
    [settings, paramByKey],
  )

  // Dirty accounting. In raw mode the raw text is authoritative, so we count the
  // raw edit (plus any metadata edits) rather than the structured settings.
  const { isDirty, dirtyCount } = useMemo(() => {
    let count = 0
    if (name !== baseline.name) count++
    if (installPath !== baseline.installPath) count++
    count += countChangedLaunch(launchArgs, baseline.launchArgs)
    if (rawMode) {
      if (rawText !== baseline.raw) count++
    } else {
      count += countChangedSettings(settings, baseline.settings)
    }
    return { isDirty: count > 0, dirtyCount: count }
  }, [name, installPath, launchArgs, rawMode, rawText, settings, baseline])

  const setName = useCallback((v: string) => setNameState(v), [])
  const setInstallPath = useCallback((v: string) => setInstallPathState(v), [])
  const setSetting = useCallback((key: string, value: string) => {
    setSettings((prev) => ({ ...prev, [key]: value }))
    setRawMode(false)
  }, [])
  const setLaunch = useCallback(
    (patch: Partial<LaunchArgs>) => setLaunchArgs((prev) => ({ ...prev, ...patch })),
    [],
  )
  const setRaw = useCallback((text: string) => {
    setRawText(text)
    setRawMode(true)
  }, [])

  const saveMutation = useMutation({
    mutationFn: async () => {
      if (!server) return
      // Structured mode: sync the outward-facing name into INI ServerName.
      const outSettings = rawMode ? settings : { ...settings, ServerName: name }
      // 1) Metadata: name / install path / launch args.
      await serversApi.update(server.id, { name, installPath, launchArgs })
      // 2) INI config only makes sense once installed (backend rejects otherwise).
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
        queryClient.invalidateQueries({ queryKey: ['server', server.id] })
      }
      queryClient.invalidateQueries({ queryKey: ['servers'] })
      // Reset the baseline to the just-saved draft so the save bar clears.
      setBaseline({
        name,
        installPath,
        settings: { ...settings },
        launchArgs: { ...launchArgs },
        raw: rawText,
      })
      setError(null)
    },
    onError: (err: unknown) => {
      const e = err as { response?: { data?: { error?: string } } }
      setError(e.response?.data?.error ?? 'Save failed')
    },
  })

  const save = useCallback(() => {
    setError(null)
    saveMutation.mutate()
  }, [saveMutation])

  const discard = useCallback(() => {
    setNameState(baseline.name)
    setInstallPathState(baseline.installPath)
    setSettings({ ...baseline.settings })
    setLaunchArgs({ ...baseline.launchArgs })
    setRawText(baseline.raw)
    setRawMode(false)
    setError(null)
  }, [baseline])

  const value: SettingsDraft = {
    name,
    installPath,
    settings,
    launchArgs,
    rawText,
    rawMode,
    params,
    paramByKey,
    installed: !!server?.installed,
    isRunning: server?.status === 'running',
    loading: isLoading,
    isDirty,
    dirtyCount,
    saving: saveMutation.isPending,
    error,
    setName,
    setInstallPath,
    setSetting,
    setLaunch,
    setRaw,
    save,
    discard,
    iniValue,
  }

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useSettingsDraft(): SettingsDraft {
  const ctx = useContext(Ctx)
  if (!ctx) {
    throw new Error('useSettingsDraft must be used within a SettingsDraftProvider')
  }
  return ctx
}
