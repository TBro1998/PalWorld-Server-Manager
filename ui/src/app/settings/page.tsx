'use client'

import { useState, useEffect, useRef, useCallback } from 'react'
import { RefreshCw, Download, Save, CheckCircle, AlertCircle, Loader2 } from 'lucide-react'
import { useTranslations } from '@/contexts/LanguageContext'
import { systemApi } from '@/lib/api'
import type { VersionInfo, CheckResult } from '@/types/system'

// Preset GitHub download mirrors.  value='' means direct (no proxy).
// value=null means "custom" — the user fills in their own URL.
const PRESET_MIRRORS: Array<{ id: string; value: string | null }> = [
  { id: 'direct',     value: '' },
  { id: 'ghfast',     value: 'https://ghfast.top' },
  { id: 'ghproxynet', value: 'https://ghproxy.net' },
  { id: 'ghproxyme',  value: 'https://gh-proxy.com' },
  { id: 'custom',     value: null },
]

function detectPreset(mirrorValue: string): string {
  const match = PRESET_MIRRORS.find(p => p.value === mirrorValue)
  return match ? match.id : 'custom'
}

export default function SettingsPage() {
  const t = useTranslations('settingsPage')

  // --- Version info ---
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null)

  // --- Update state ---
  const [checkResult, setCheckResult] = useState<CheckResult | null>(null)
  const [checking, setChecking] = useState(false)
  const [applying, setApplying] = useState(false)
  const [applyProgress, setApplyProgress] = useState(0)
  const [applyMsg, setApplyMsg] = useState('')
  const [restarting, setRestarting] = useState(false)
  const [restartVersion, setRestartVersion] = useState('')
  const [updateError, setUpdateError] = useState('')

  // --- Mirror settings ---
  const [mirror, setMirror] = useState('')
  const [selectedPreset, setSelectedPreset] = useState('direct')
  const [mirrorSaving, setMirrorSaving] = useState(false)
  const [mirrorSaved, setMirrorSaved] = useState(false)
  const [mirrorError, setMirrorError] = useState('')

  const eventSourceRef = useRef<EventSource | null>(null)

  // Check for updates
  const handleCheck = useCallback(async () => {
    setChecking(true)
    setUpdateError('')
    try {
      const r = await systemApi.checkUpdate(false)
      setCheckResult(r.data)
      if (r.data.err) setUpdateError(r.data.err)
    } catch {
      setUpdateError(t('update.checkFailed'))
    } finally {
      setChecking(false)
    }
  }, [t])

  // Poll /version after restart until new version appears
  const pollForNewVersion = useCallback((expected: string) => {
    let attempts = 0
    const maxAttempts = 60 // 60 × 2s = 2 minutes

    const poll = async () => {
      attempts++
      try {
        const r = await systemApi.version()
        if (r.data.version === expected) {
          setVersionInfo(r.data)
          setRestartVersion(expected)
          setRestarting(false)
          return
        }
      } catch {
        // server is still down — expected during restart
      }
      if (attempts < maxAttempts) {
        setTimeout(poll, 2000)
      } else {
        setRestarting(false)
      }
    }

    setTimeout(poll, 3000) // give new process time to bind port
  }, [])

  // Open the SSE update stream and wire all event handlers.  Call before
  // triggering applyUpdate, or on mount to re-attach to an in-progress
  // download without re-triggering the backend operation.
  const connectStream = useCallback((expectedVersion: string, initialPct = 0, initialMsg = '') => {
    setApplying(true)
    setApplyProgress(initialPct)
    setApplyMsg(initialMsg)
    setUpdateError('')

    const es = new EventSource(systemApi.updateStreamUrl())
    eventSourceRef.current = es

    es.addEventListener('progress', (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data) as { pct: number; msg: string }
        setApplyProgress(data.pct)
        setApplyMsg(data.msg)
      } catch {}
    })
    es.addEventListener('log', (e: MessageEvent) => { setApplyMsg(e.data) })
    es.addEventListener('error', (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data) as { error: string }
        setUpdateError(data.error || t('update.applyFailed'))
      } catch {
        setUpdateError(t('update.applyFailed'))
      }
      setApplying(false)
      es.close()
    })
    es.addEventListener('restarting', () => {
      setRestarting(true)
      setApplying(false)
      es.close()
      pollForNewVersion(expectedVersion)
    })
  }, [t, pollForNewVersion])

  // Apply update — open the SSE stream first, then trigger the backend.
  const handleApply = useCallback(async () => {
    if (!checkResult?.hasUpdate) return
    connectStream(checkResult.latestVersion ?? '')
    try {
      await systemApi.applyUpdate()
    } catch {
      setUpdateError(t('update.applyFailed'))
      setApplying(false)
      eventSourceRef.current?.close()
    }
  }, [checkResult, t, connectStream])

  // Load version + cached check result + update status + settings on mount.
  // When an update is already running (page refresh / navigating away and back),
  // recover: re-attach to the SSE stream or show the restarting spinner.
  useEffect(() => {
    Promise.all([
      systemApi.version().catch(() => null),
      systemApi.checkUpdate(true).catch(() => null),
      systemApi.updateStatus().catch(() => null),
      systemApi.getSettings().catch(() => null),
    ]).then(([ver, check, status, sett]) => {
      if (ver) setVersionInfo(ver.data)
      if (check) setCheckResult(check.data)
      if (sett) {
        const v = sett.data.download_mirror ?? ''
        setMirror(v)
        setSelectedPreset(detectPreset(v))
      }
      if (!status) return
      const s = status.data
      const lv = check?.data.latestVersion ?? ''
      if (s.phase === 'downloading') {
        // An update was already in progress; re-attach to the SSE stream
        // without re-triggering applyUpdate on the backend.
        connectStream(lv, s.pct, s.msg)
      } else if (s.phase === 'restarting') {
        setRestarting(true)
        pollForNewVersion(lv)
      } else if (s.phase === 'error' && s.err) {
        setUpdateError(s.err)
      }
    })
    // connectStream and pollForNewVersion are stable useCallback refs.
    // [] is intentional — this effect runs only on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Select a preset mirror
  const handleSelectPreset = useCallback((preset: { id: string; value: string | null }) => {
    setSelectedPreset(preset.id)
    if (preset.value !== null) {
      setMirror(preset.value)
    }
    setMirrorSaved(false)
  }, [])

  // Save mirror
  const handleSaveMirror = useCallback(async () => {
    setMirrorSaving(true)
    setMirrorSaved(false)
    setMirrorError('')
    try {
      await systemApi.setSettings({ download_mirror: mirror })
      setMirrorSaved(true)
      setTimeout(() => setMirrorSaved(false), 3000)
    } catch {
      setMirrorError(t('mirror.saveFailed'))
    } finally {
      setMirrorSaving(false)
    }
  }, [mirror, t])

  // Cleanup SSE on unmount
  useEffect(() => () => { eventSourceRef.current?.close() }, [])

  return (
    <div className="p-6 max-w-2xl mx-auto space-y-8">
      <h1 className="text-2xl font-extrabold tracking-tight text-foreground">{t('title')}</h1>

      {/* ── About ── */}
      <section className="rounded-2xl border border-border bg-card p-6 space-y-4">
        <h2 className="text-lg font-bold text-foreground">{t('about.title')}</h2>
        <dl className="space-y-2 text-sm">
          <div className="flex justify-between">
            <dt className="text-muted-foreground">{t('about.version')}</dt>
            <dd className="font-mono font-semibold text-foreground">{versionInfo?.version ?? '—'}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-muted-foreground">{t('about.buildTime')}</dt>
            <dd className="font-mono text-foreground">{versionInfo?.buildTime ?? '—'}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-muted-foreground">{t('about.gitCommit')}</dt>
            <dd className="font-mono text-foreground">{versionInfo?.gitCommit ?? '—'}</dd>
          </div>
        </dl>
      </section>

      {/* ── Update ── */}
      <section className="rounded-2xl border border-border bg-card p-6 space-y-4">
        <h2 className="text-lg font-bold text-foreground">{t('update.title')}</h2>

        {checkResult?.isDev ? (
          <p className="text-sm text-muted-foreground">{t('update.devBuild')}</p>
        ) : (
          <>
            {/* Check button */}
            {!applying && !restarting && (
              <button
                type="button"
                onClick={handleCheck}
                disabled={checking}
                className="inline-flex items-center gap-2 rounded-xl bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground shadow-pal hover:opacity-90 disabled:opacity-50"
              >
                {checking
                  ? <Loader2 className="h-4 w-4 animate-spin" />
                  : <RefreshCw className="h-4 w-4" />}
                {checking ? t('update.checking') : t('update.checkButton')}
              </button>
            )}

            {/* Error */}
            {updateError && (
              <div className="flex items-center gap-2 text-sm text-destructive">
                <AlertCircle className="h-4 w-4 shrink-0" />
                {updateError}
              </div>
            )}

            {/* Up to date */}
            {checkResult && !checkResult.hasUpdate && !checkResult.err && !checkResult.isDev && (
              <div className="flex items-center gap-2 text-sm text-success">
                <CheckCircle className="h-4 w-4 shrink-0" />
                {t('update.upToDate')}
              </div>
            )}

            {/* New version available */}
            {checkResult?.hasUpdate && !applying && !restarting && (
              <div className="space-y-3">
                <p className="text-sm font-semibold text-primary">
                  {t('update.newVersion').replace('{{version}}', checkResult.latestVersion ?? '')}
                </p>
                {checkResult.releaseNotes && (
                  <pre className="max-h-40 overflow-y-auto rounded-xl bg-secondary p-3 text-xs text-foreground whitespace-pre-wrap">
                    {checkResult.releaseNotes}
                  </pre>
                )}
                <button
                  type="button"
                  onClick={handleApply}
                  className="inline-flex items-center gap-2 rounded-xl bg-success px-4 py-2 text-sm font-semibold text-white shadow-pal hover:opacity-90"
                >
                  <Download className="h-4 w-4" />
                  {t('update.applyButton')}
                </button>
              </div>
            )}

            {/* Applying progress */}
            {applying && (
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {t('update.applying')}
                </div>
                {applyProgress > 0 && (
                  <div className="h-2 w-full overflow-hidden rounded-full bg-secondary">
                    <div
                      className="h-full rounded-full bg-primary transition-all"
                      style={{ width: `${applyProgress}%` }}
                    />
                  </div>
                )}
                {applyMsg && <p className="text-xs text-muted-foreground">{applyMsg}</p>}
              </div>
            )}

            {/* Restarting */}
            {restarting && (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t('update.restarting')}
              </div>
            )}

            {/* Restart success */}
            {restartVersion && !restarting && (
              <div className="flex items-center gap-2 text-sm text-success">
                <CheckCircle className="h-4 w-4 shrink-0" />
                {t('update.success').replace('{{version}}', restartVersion)}
              </div>
            )}
          </>
        )}
      </section>

      {/* ── Mirror ── */}
      <section className="rounded-2xl border border-border bg-card p-6 space-y-4">
        <h2 className="text-lg font-bold text-foreground">{t('mirror.title')}</h2>

        {/* Preset options */}
        <div className="flex flex-wrap gap-2">
          {PRESET_MIRRORS.map(preset => (
            <button
              key={preset.id}
              type="button"
              onClick={() => handleSelectPreset(preset)}
              className={`flex flex-col items-start rounded-xl border px-3 py-2 text-left text-sm transition-colors ${
                selectedPreset === preset.id
                  ? 'border-primary bg-primary/10 text-primary'
                  : 'border-border bg-background text-foreground hover:border-primary/40'
              }`}
            >
              <span className="font-semibold">{t(`mirror.preset.${preset.id}`)}</span>
              {preset.value && (
                <span className="mt-0.5 text-xs text-muted-foreground">{preset.value}</span>
              )}
            </button>
          ))}
        </div>

        {/* Custom URL input — only shown when "custom" preset is selected */}
        {selectedPreset === 'custom' && (
          <div className="space-y-1.5">
            <label className="block text-sm text-muted-foreground" htmlFor="mirror-input">
              {t('mirror.customLabel')}
            </label>
            <input
              id="mirror-input"
              type="url"
              value={mirror}
              onChange={e => {
                setMirror(e.target.value)
                setMirrorSaved(false)
              }}
              placeholder={t('mirror.placeholder')}
              className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
            />
          </div>
        )}

        {/* Save */}
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={handleSaveMirror}
            disabled={mirrorSaving}
            className="inline-flex items-center gap-2 rounded-xl bg-secondary px-4 py-2 text-sm font-semibold text-foreground hover:bg-secondary/80 disabled:opacity-50"
          >
            {mirrorSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            {mirrorSaved ? t('mirror.saved') : t('mirror.save')}
          </button>
          {mirrorError && <p className="text-sm text-destructive">{mirrorError}</p>}
        </div>
      </section>
    </div>
  )
}
