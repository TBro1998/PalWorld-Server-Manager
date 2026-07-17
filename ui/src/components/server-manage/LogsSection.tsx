'use client'

import { useEffect, useRef, useState } from 'react'
import { serversApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'
import { SectionShell, useServer } from './shared'

type StreamState = 'connecting' | 'live' | 'closed'

// Inline runtime-log viewer for the manage page (replaces the old runtime-log
// dialog). Fetches history then follows the live SSE stream. The SteamCMD
// install/update log stays a dialog (triggered from list-page/Mods actions).
export function LogsSection() {
  const t = useTranslations('serverManage')
  const ts = useTranslations('servers')
  const { data: server } = useServer()
  const serverId = server?.id

  const [lines, setLines] = useState<string[]>([])
  const [streamState, setStreamState] = useState<StreamState>('closed')
  const scrollRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (serverId === undefined) return

    let cancelled = false
    // eslint-disable-next-line react-hooks/set-state-in-effect -- reset stream UI state on server change
    setStreamState('connecting')
    setLines([])

    // 1. Historical logs for the running process.
    serversApi
      .getLogs(serverId, 'server')
      .then((res) => {
        if (!cancelled) setLines(res.data.logs ?? [])
      })
      .catch(() => {
        /* ignore: SSE will still deliver new lines */
      })

    // 2. Live SSE stream. Backend emits named `log` events (one per line).
    const es = new EventSource(serversApi.logStreamUrl(serverId, 'server'))
    es.addEventListener('log', (e) => {
      setLines((prev) => [...prev, (e as MessageEvent).data])
    })
    es.onopen = () => setStreamState('live')
    es.onerror = () => setStreamState('connecting')

    return () => {
      cancelled = true
      es.close()
      setStreamState('closed')
    }
  }, [serverId])

  // Auto-scroll to the bottom as new lines arrive.
  useEffect(() => {
    const el = scrollRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [lines])

  return (
    <SectionShell title={t('sections.logs')} desc={t('logsSection.desc')} comingSoon={false}>
      <div className="flex items-center gap-2">
        {streamState === 'live' && (
          <span className="flex items-center gap-1 text-xs font-medium text-success">
            <span className="h-2 w-2 animate-pulse rounded-full bg-success" />
            {ts('logsLive')}
          </span>
        )}
      </div>
      <div
        ref={scrollRef}
        className="max-h-[65vh] min-h-[320px] overflow-y-auto rounded-2xl border-2 border-border bg-zinc-950 p-3 font-mono text-xs text-zinc-100 shadow-pal"
      >
        {lines.length === 0 ? (
          <div className="text-zinc-500">{ts('logsEmpty')}</div>
        ) : (
          lines.map((line, i) => (
            <div key={i} className="whitespace-pre-wrap break-all">
              {line}
            </div>
          ))
        )}
      </div>
    </SectionShell>
  )
}
