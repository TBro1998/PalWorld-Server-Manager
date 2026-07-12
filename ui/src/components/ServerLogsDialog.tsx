'use client'

import React, { useEffect, useRef, useState } from 'react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from './ui/dialog'
import { Button } from './ui/button'
import { serversApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'
import type { LogKind, Server } from '@/types/server'

interface ServerLogsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  server: Server | null
  // Which log stream to show: 'server' (running process) or 'steamcmd'
  // (install/update output). Defaults to 'server'.
  kind?: LogKind
}

type StreamState = 'connecting' | 'live' | 'closed'

export function ServerLogsDialog({
  open,
  onOpenChange,
  server,
  kind = 'server',
}: ServerLogsDialogProps) {
  const t = useTranslations('servers')
  const [lines, setLines] = useState<string[]>([])
  const [streamState, setStreamState] = useState<StreamState>('closed')
  const scrollRef = useRef<HTMLDivElement | null>(null)

  const serverId = server?.id
  const title = kind === 'steamcmd' ? t('steamcmdLogsTitle') : t('logsTitle')

  useEffect(() => {
    if (!open || serverId === undefined) return

    let cancelled = false
    // eslint-disable-next-line react-hooks/set-state-in-effect -- reset stream UI state on (re)open
    setStreamState('connecting')
    setLines([])

    // 1. Fetch historical logs for this kind.
    serversApi
      .getLogs(serverId, kind)
      .then((res) => {
        if (!cancelled) setLines(res.data.logs ?? [])
      })
      .catch(() => {
        /* ignore: SSE will still deliver new lines */
      })

    // 2. Open the live SSE stream. Backend emits `c.SSEvent("log", line)`,
    // so we must listen for the named `log` event (not onmessage).
    const es = new EventSource(serversApi.logStreamUrl(serverId, kind))
    es.addEventListener('log', (e) => {
      setLines((prev) => [...prev, (e as MessageEvent).data])
    })
    es.onopen = () => setStreamState('live')
    es.onerror = () => {
      // EventSource auto-reconnects; reflect the reconnecting state but do not
      // close it manually (that happens on cleanup / dialog unmount).
      setStreamState('connecting')
    }

    return () => {
      cancelled = true
      es.close()
      setStreamState('closed')
    }
  }, [open, serverId, kind])

  // Auto-scroll to the bottom whenever new log lines arrive.
  useEffect(() => {
    const el = scrollRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [lines])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <span>
              {title}
              {server ? ` — ${server.name}` : ''}
            </span>
            {streamState === 'live' && (
              <span className="flex items-center gap-1 text-xs font-normal text-emerald-600 dark:text-emerald-400">
                <span className="h-2 w-2 rounded-full bg-emerald-500" />
                {t('logsLive')}
              </span>
            )}
          </DialogTitle>
        </DialogHeader>

        <div
          ref={scrollRef}
          className="max-h-[60vh] overflow-y-auto rounded-md bg-zinc-950 p-3 font-mono text-xs text-zinc-100"
        >
          {lines.length === 0 ? (
            <div className="text-zinc-500">{t('logsEmpty')}</div>
          ) : (
            lines.map((line, i) => (
              <div key={i} className="whitespace-pre-wrap break-all">
                {line}
              </div>
            ))
          )}
        </div>

        <DialogFooter>
          <Button type="button" onClick={() => onOpenChange(false)}>
            {t('logsClose')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
