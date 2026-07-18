'use client'

import React, { useState, useCallback, useEffect, useMemo } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { ExternalLink, Plus, Check, AlertTriangle, Search } from 'lucide-react'
import { modsApi, steamApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'
import type { Server, Mod, WorkshopItem, WorkshopDep } from '@/types/server'

// ---------------------------------------------------------------------------
// Browser-direct Steam API helpers
//
// The backend proxy (/api/steam/workshop/*) makes outbound requests from the
// server, which fails when the server cannot reach api.steampowered.com (e.g.
// behind a firewall). These helpers call Steam directly from the browser,
// which typically can reach Steam even when the server cannot.
// ---------------------------------------------------------------------------

const STEAM_API = 'https://api.steampowered.com'
const APP_ID = '1623730'
const MAX_RECURSE_DEPTH = 5

interface SteamSearchResult {
  items: WorkshopItem[]
  nextCursor: string
  total: number
}

interface SteamDetailItem {
  workshopId: string
  title: string
  previewUrl: string
  children: string[]
}

async function steamQueryFiles(
  key: string,
  query: string,
  cursor: string,
  numPerPage: number,
): Promise<SteamSearchResult> {
  const params = new URLSearchParams({
    key,
    appid: APP_ID,
    query_type: '12', // k_PublishedFileQueryType_RankedByTextSearch
    search_text: query,
    cursor: cursor || '*',
    numperpage: String(Math.min(100, Math.max(1, numPerPage))),
    return_short_description: 'true',
    return_previews: 'true',
    return_tags: 'true',
    return_metadata: 'true',
  })

  const res = await fetch(`${STEAM_API}/IPublishedFileService/QueryFiles/v1/?${params}`)
  if (!res.ok) throw new Error(`Steam API ${res.status}`)
  const json = await res.json()
  const r = json?.response ?? {}

  const items: WorkshopItem[] = (r.publishedfiledetails ?? []).map((d: Record<string, unknown>) => ({
    workshop_id: String(d.publishedfileid ?? ''),
    title: String(d.title ?? ''),
    description: String(d.short_description ?? ''),
    preview_url: String(d.preview_url ?? ''),
    author: String(d.creator ?? ''),
    subscriptions: Number(d.subscriptions ?? 0),
    views: Number(d.views ?? 0),
    time_updated: Number(d.time_updated ?? 0),
    tags: ((d.tags as { tag: string }[]) ?? []).map((t) => t.tag).filter(Boolean),
  }))

  let nextCursor = String(r.next_cursor ?? '')
  // Normalise: Steam may echo back '*' or the same cursor on the last page.
  if (nextCursor === cursor && items.length < numPerPage) nextCursor = ''

  return { items, nextCursor, total: Number(r.total ?? 0) }
}

async function steamGetDetails(key: string, ids: string[]): Promise<SteamDetailItem[]> {
  if (!ids.length) return []
  const params = new URLSearchParams({
    key,
    return_children: 'true',
    return_short_description: 'true',
    return_previews: 'true',
  })
  ids.forEach((id, i) => params.append(`publishedfileids[${i}]`, id))

  const res = await fetch(`${STEAM_API}/IPublishedFileService/GetDetails/v1/?${params}`)
  if (!res.ok) throw new Error(`Steam API ${res.status}`)
  const json = await res.json()

  return ((json?.response?.publishedfiledetails ?? []) as Record<string, unknown>[]).map((d) => ({
    workshopId: String(d.publishedfileid ?? ''),
    title: String(d.title ?? ''),
    previewUrl: String(d.preview_url ?? ''),
    children: ((d.children as { publishedfileid: string }[]) ?? [])
      .map((c) => String(c.publishedfileid ?? '').trim())
      .filter(Boolean),
  }))
}

async function steamResolveDeps(key: string, rootId: string): Promise<WorkshopDep[]> {
  const visited = new Set([rootId])
  const result: WorkshopDep[] = []
  let queue = [rootId]

  for (let depth = 0; depth < MAX_RECURSE_DEPTH && queue.length > 0; depth++) {
    const details = await steamGetDetails(key, queue)
    const nextQueue: string[] = []

    for (const d of details) {
      for (const childId of d.children) {
        if (!childId || visited.has(childId)) continue
        visited.add(childId)
        nextQueue.push(childId)
      }
      if (d.workshopId !== rootId) {
        result.push({ workshop_id: d.workshopId, title: d.title, preview_url: d.previewUrl })
      }
    }
    queue = nextQueue
  }
  return result
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  server: Server
  mods: Mod[]
}

export function WorkshopBrowserDialog({ open, onOpenChange, server, mods }: Props) {
  const t = useTranslations('serverConfig')
  const queryClient = useQueryClient()

  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [cursor, setCursor] = useState('*')
  const [allItems, setAllItems] = useState<WorkshopItem[]>([])
  const [total, setTotal] = useState(0)
  const [nextCursor, setNextCursor] = useState('')
  const [isFetching, setIsFetching] = useState(false)
  const [addingId, setAddingId] = useState<string | null>(null)
  const [addedIds, setAddedIds] = useState<Set<string>>(new Set())
  const [error, setError] = useState<string | null>(null)

  // Missing-deps confirmation dialog state
  const [depsOpen, setDepsOpen] = useState(false)
  const [pendingDeps, setPendingDeps] = useState<WorkshopDep[]>([])
  const [addingDeps, setAddingDeps] = useState(false)

  // Fetch the Web API key from the backend (browser uses it directly).
  const { data: keyData } = useQuery({
    queryKey: ['steamWebApiKey'],
    queryFn: async () => (await steamApi.getWebApiKey()).data,
    enabled: open,
    staleTime: 60_000,
  })
  const apiKey = keyData?.configured ? keyData.key : ''

  // Set of workshop IDs already in the mod list.
  const existingIds = useMemo(() => new Set(mods.map((m) => m.workshop_id)), [mods])

  // Debounce the search query.
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedQuery(query)
      setCursor('*')
      setAllItems([])
      setTotal(0)
      setNextCursor('')
    }, 400)
    return () => clearTimeout(timer)
  }, [query])

  // Reset state when the dialog opens.
  useEffect(() => {
    if (!open) return
    /* eslint-disable react-hooks/set-state-in-effect -- intentional: reset all dialog state on each open */
    setQuery('')
    setDebouncedQuery('')
    setCursor('*')
    setAllItems([])
    setTotal(0)
    setNextCursor('')
    setAddedIds(new Set())
    setError(null)
    /* eslint-enable react-hooks/set-state-in-effect */
  }, [open])

  // Run the search whenever query or cursor changes (browser-direct).
  useEffect(() => {
    if (!open || !apiKey) return
    let cancelled = false

    /* eslint-disable react-hooks/set-state-in-effect -- direct setState is intentional here: start loading/clear error before the async fetch */
    setIsFetching(true)
    setError(null)
    /* eslint-enable react-hooks/set-state-in-effect */
    steamQueryFiles(apiKey, debouncedQuery, cursor, 20)
      .then((res) => {
        if (cancelled) return
        if (cursor === '*') {
          setAllItems(res.items)
        } else {
          setAllItems((prev) => [...prev, ...res.items])
        }
        setTotal(res.total)
        setNextCursor(res.nextCursor)
      })
      .catch((err) => {
        if (cancelled) return
        setError(t('workshop.errors.steamError') + ' (' + String(err) + ')')
      })
      .finally(() => {
        if (!cancelled) setIsFetching(false)
      })

    return () => { cancelled = true }
  // debouncedQuery and cursor are the only pagination triggers; apiKey changes
  // only when the user reconfigures the key (rare), which resets state anyway.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, apiKey, debouncedQuery, cursor])

  const loadMore = useCallback(() => {
    if (nextCursor && nextCursor !== '*') setCursor(nextCursor)
  }, [nextCursor])

  const invalidateMods = useCallback(
    () => queryClient.invalidateQueries({ queryKey: ['mods', server.id] }),
    [queryClient, server.id],
  )

  // Add a single mod (DB only), then check its Steam-layer dependencies.
  const handleAdd = useCallback(
    async (item: WorkshopItem) => {
      if (addingId || !apiKey) return
      setAddingId(item.workshop_id)
      setError(null)
      try {
        await modsApi.add(server.id, { workshopId: item.workshop_id, name: item.title })
        setAddedIds((prev) => new Set([...prev, item.workshop_id]))
        invalidateMods()

        // Resolve Steam-layer dependencies directly from the browser.
        const allDeps = await steamResolveDeps(apiKey, item.workshop_id)
        const currentIds = new Set([...existingIds, ...addedIds, item.workshop_id])
        const missing = allDeps.filter((d) => !currentIds.has(d.workshop_id))
        if (missing.length > 0) {
          setPendingDeps(missing)
          setDepsOpen(true)
        }
      } catch (err) {
        const e = err as { response?: { data?: { error?: string } } }
        setError(t('workshop.errors.addFailed').replace('{{msg}}', e.response?.data?.error ?? String(err)))
      } finally {
        setAddingId(null)
      }
    },
    [addingId, apiKey, addedIds, existingIds, server.id, invalidateMods, t],
  )

  // Add all missing dependencies in sequence.
  const handleAddAllDeps = useCallback(async () => {
    setAddingDeps(true)
    try {
      for (const dep of pendingDeps) {
        if (existingIds.has(dep.workshop_id) || addedIds.has(dep.workshop_id)) continue
        await modsApi.add(server.id, { workshopId: dep.workshop_id, name: dep.title })
        setAddedIds((prev) => new Set([...prev, dep.workshop_id]))
      }
      invalidateMods()
    } catch (err) {
      const e = err as { response?: { data?: { error?: string } } }
      setError(t('workshop.errors.addFailed').replace('{{msg}}', e.response?.data?.error ?? String(err)))
    } finally {
      setAddingDeps(false)
      setDepsOpen(false)
      setPendingDeps([])
    }
  }, [addedIds, existingIds, pendingDeps, server.id, invalidateMods, t])

  const canLoadMore = !!nextCursor && nextCursor !== '*' && allItems.length < total

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="flex max-h-[85vh] max-w-2xl flex-col gap-0 p-0">
          <DialogHeader className="border-b px-6 py-4">
            <DialogTitle>{t('workshop.title')}</DialogTitle>
          </DialogHeader>

          {/* Search bar */}
          <div className="border-b px-6 py-3">
            <div className="relative">
              <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t('workshop.searchPlaceholder')}
                className="pl-9"
                autoFocus
              />
            </div>
            {total > 0 && !isFetching && (
              <p className="mt-1.5 text-xs text-muted-foreground">
                {t('workshop.total').replace('{{total}}', total.toString())}
              </p>
            )}
          </div>

          {/* Result list */}
          <div className="flex-1 overflow-y-auto px-6 py-3">
            {isFetching && allItems.length === 0 ? (
              <p className="py-8 text-center text-sm text-muted-foreground">{t('workshop.loading')}</p>
            ) : allItems.length === 0 && !isFetching ? (
              <p className="py-8 text-center text-sm text-muted-foreground">{t('workshop.empty')}</p>
            ) : (
              <div className="space-y-2">
                {allItems.map((item) => (
                  <WorkshopItemRow
                    key={item.workshop_id}
                    item={item}
                    isAdded={existingIds.has(item.workshop_id) || addedIds.has(item.workshop_id)}
                    isAdding={addingId === item.workshop_id}
                    onAdd={handleAdd}
                    t={t}
                  />
                ))}
                {isFetching && allItems.length > 0 && (
                  <p className="py-2 text-center text-xs text-muted-foreground">{t('workshop.loading')}</p>
                )}
                {canLoadMore && !isFetching && (
                  <div className="pt-2 text-center">
                    <Button type="button" variant="outline" size="sm" onClick={loadMore}>
                      {t('workshop.loadMore')}
                    </Button>
                  </div>
                )}
              </div>
            )}
          </div>

          {error && (
            <div className="border-t px-6 py-2">
              <p className="text-sm text-destructive">{error}</p>
            </div>
          )}
        </DialogContent>
      </Dialog>

      <MissingDepsDialog
        open={depsOpen}
        onOpenChange={setDepsOpen}
        deps={pendingDeps}
        adding={addingDeps}
        onAddAll={handleAddAllDeps}
        t={t}
      />
    </>
  )
}

// WorkshopItemRow renders a single search result card.
interface RowProps {
  item: WorkshopItem
  isAdded: boolean
  isAdding: boolean
  onAdd: (item: WorkshopItem) => void
  t: (key: string) => string
}

function WorkshopItemRow({ item, isAdded, isAdding, onAdd, t }: RowProps) {
  const workshopUrl = `https://steamcommunity.com/sharedfiles/filedetails/?id=${item.workshop_id}`
  const subCount = item.subscriptions > 0
    ? t('workshop.subscribers').replace('{{count}}', formatCount(item.subscriptions))
    : null

  return (
    <div className="flex items-start gap-3 rounded-xl border border-border/60 bg-background/60 p-3">
      {item.preview_url ? (
        // eslint-disable-next-line @next/next/no-img-element -- Steam CDN URLs are external, not local assets; next/image does not support arbitrary external URLs without config
        <img
          src={item.preview_url}
          alt={item.title}
          className="h-16 w-16 shrink-0 rounded-lg object-cover"
          loading="lazy"
        />
      ) : (
        <div className="h-16 w-16 shrink-0 rounded-lg bg-muted" />
      )}

      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium leading-snug">{item.title || item.workshop_id}</div>
        {item.description && (
          <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">{item.description}</p>
        )}
        <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-[10px] text-muted-foreground">
          {subCount && <span>{subCount}</span>}
          {(item.tags ?? []).slice(0, 3).map((tag) => (
            <span
              key={tag}
              className="rounded-full border border-border/60 bg-muted px-1.5 py-0.5 font-medium"
            >
              {tag}
            </span>
          ))}
        </div>
      </div>

      <div className="flex shrink-0 flex-col items-end gap-1.5">
        <a
          href={workshopUrl}
          target="_blank"
          rel="noreferrer"
          className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
        >
          <ExternalLink size={12} />
          {t('workshop.openPage')}
        </a>
        {isAdded ? (
          <span className="flex items-center gap-1 text-xs font-medium text-success">
            <Check size={13} />
            {t('workshop.added')}
          </span>
        ) : (
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="h-7 px-2.5 text-xs"
            disabled={isAdding}
            onClick={() => onAdd(item)}
          >
            <Plus size={13} className="mr-1" />
            {isAdding ? '…' : t('workshop.add')}
          </Button>
        )}
      </div>
    </div>
  )
}

// MissingDepsDialog prompts the user to add all missing Steam-layer dependencies.
interface DepDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  deps: WorkshopDep[]
  adding: boolean
  onAddAll: () => void
  t: (key: string) => string
}

function MissingDepsDialog({ open, onOpenChange, deps, adding, onAddAll, t }: DepDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <AlertTriangle size={16} className="text-warning" />
            {t('workshop.deps.title')}
          </DialogTitle>
        </DialogHeader>

        <p className="text-sm text-muted-foreground">{t('workshop.deps.hint')}</p>

        <div className="max-h-48 space-y-1.5 overflow-y-auto">
          {deps.map((dep) => (
            <div key={dep.workshop_id} className="flex items-center gap-2 rounded-lg border border-border/60 bg-muted/40 px-3 py-2">
              {dep.preview_url && (
                // eslint-disable-next-line @next/next/no-img-element -- Steam CDN URL, cannot use next/image without configuring remotePatterns
                <img src={dep.preview_url} alt={dep.title} className="h-8 w-8 shrink-0 rounded object-cover" />
              )}
              <div className="min-w-0">
                <div className="truncate text-sm font-medium">{dep.title || dep.workshop_id}</div>
                <div className="font-mono text-xs text-muted-foreground">{dep.workshop_id}</div>
              </div>
            </div>
          ))}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={adding}>
            {t('workshop.deps.skip')}
          </Button>
          <Button type="button" onClick={onAddAll} disabled={adding}>
            {adding ? t('workshop.deps.adding') : t('workshop.deps.addAll')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function formatCount(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return n.toString()
}
