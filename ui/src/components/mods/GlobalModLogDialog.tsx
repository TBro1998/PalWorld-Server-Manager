'use client'

import { useEffect, useRef, useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { globalModsApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'

interface GlobalModLogDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Called when the SSE stream emits a `done` event. */
  onDone?: (success: boolean) => void
}

/**
 * Streams the global mod download log (/api/mods/logs/stream) via SSE.
 * Re-connects whenever the dialog is opened so fresh download runs are captured.
 */
export function GlobalModLogDialog({ open, onOpenChange, onDone }: GlobalModLogDialogProps) {
  const t = useTranslations('modLibrary')
  const [lines, setLines] = useState<string[]>([])
  const [live, setLive] = useState(false)
  const scrollRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (!open) return
    setLines([])
    setLive(false)

    const es = new EventSource(globalModsApi.logStreamUrl())

    es.addEventListener('log', (e) => {
      setLines((prev) => [...prev, (e as MessageEvent).data])
    })

    es.addEventListener('done', (e) => {
      const success = (e as MessageEvent).data === 'ok'
      setLive(false)
      onDone?.(success)
    })

    es.onopen = () => setLive(true)
    es.onerror = () => setLive(false)

    return () => {
      es.close()
      setLive(false)
    }
  }, [open]) // eslint-disable-line react-hooks/exhaustive-deps

  // Auto-scroll to bottom as new lines arrive.
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [lines])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {t('logs')}
            {live && (
              <span className="flex items-center gap-1 text-xs font-normal text-success">
                <span className="h-2 w-2 animate-pulse rounded-full bg-success" />
                Live
              </span>
            )}
          </DialogTitle>
        </DialogHeader>

        <div
          ref={scrollRef}
          className="max-h-96 min-h-[12rem] overflow-auto rounded-md bg-black/90 p-3 font-mono text-xs text-green-200"
        >
          {lines.length === 0 ? (
            <div className="text-zinc-500">Waiting for output...</div>
          ) : (
            lines.map((line, i) => (
              <div key={i} className="whitespace-pre-wrap break-all">
                {line}
              </div>
            ))
          )}
        </div>

        <div className="flex justify-end">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Close
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
