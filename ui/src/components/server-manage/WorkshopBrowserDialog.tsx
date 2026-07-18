'use client'

import React, { useState, useCallback, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { ExternalLink, Plus, Check, AlertTriangle, Search } from 'lucide-react'
import { modsApi, globalModsApi, steamApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'
import type { Server, Mod, ServerMod, WorkshopItem, WorkshopDep } from '@/types/server'

// WorkshopBrowserDialog renders a modal workshop search experience.
//
// Two usage modes:
//   1. Global library (from /mods page): pass only `onAdded`. Adds to global
//      library via globalModsApi; never links to a server.
//   2. Server-scoped (no longer used from server mods page — kept for reuse
//      in future if needed). Pass `server` to enable server-specific logic.
//
// Props:
//   open / onOpenChange  — controlled visibility
//   onAdded              — called after a mod is added to the global library
//   server               — optional; when present, mods are also linked to server
//   existingServerMods   — current server mods list (for already-added detection)
//   existingGlobalMods   — current global library list (for already-added detection
//                          when used from the /mods page without a server context)
interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  onAdded?: (mod: Mod) => void
  server?: Server
  existingServerMods?: ServerMod[]
  existingGlobalMods?: Mod[]
}

export function WorkshopBrowserDialog({ open, onOpenChange, onAdded, server, existingServerMods, existingGlobalMods }: Props) {
  const t = useTranslations('serverConfig')
  const queryClient = useQueryClient()

  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [cursor, setCursor] = useState('*')
  const [allItems, setAllItems] = useState<WorkshopItem[]>([])
  const [total, setTotal] = useState(0)
  const [nextCursor, setNextCursor] = useState('')
  const [addingId, setAddingId] = useState<string | null>(null)
  const [addedIds, setAddedIds] = useState<Set<string>>(new Set())
  const [error, setError] = useState<string | null>(null)

  // Missing-deps confirmation dialog state
  const [depsOpen, setDepsOpen] = useState(false)
  const [pendingDeps, setPendingDeps] = useState<WorkshopDep[]>([])
  const [addingDeps, setAddingDeps] = useState(false)

  // Set of workshop IDs already in the global library or linked to this server.
  const existingIds = new Set([
    ...(existingServerMods ?? []).map((m) => m.workshop_id),
    ...(existingGlobalMods ?? []).map((m) => m.workshop_id),
  ])

  // Debounce the search query.
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedQuery(query)
      // Reset pagination when query changes.
      setCursor('*')
      setAllItems([])
      setTotal(0)
      setNextCursor('')
    }, 400)
    return () => clearTimeout(timer)
  }, [query])

  // Reset state when the dialog opens/closes.
  useEffect(() => {
    if (!open) return
    // Remove cached workshop-search results so the query always re-fetches on
    // (re)open. Without this, staleTime keeps the cache "fresh" and queryFn
    // never runs, leaving allItems empty after the first close.
    queryClient.removeQueries({ queryKey: ['workshop-search'] })
    // eslint-disable-next-line react-hooks/set-state-in-effect -- reset all search state on (re)open
    setQuery('')
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDebouncedQuery('')
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setCursor('*')
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setAllItems([])
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setTotal(0)
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setNextCursor('')
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setAddedIds(new Set())
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setError(null)
  }, [open, queryClient])

  const { isFetching } = useQuery({
    queryKey: ['workshop-search', debouncedQuery, cursor],
    queryFn: async () => {
      const res = await steamApi.workshopSearch({ q: debouncedQuery, cursor, num: 20 })
      const data = res.data
      if (cursor === '*') {
        // Fresh search — replace results.
        setAllItems(data.items ?? [])
      } else {
        // Pagination — append results.
        setAllItems((prev) => [...prev, ...(data.items ?? [])])
      }
      setTotal(data.total ?? 0)
      setNextCursor(data.next_cursor ?? '')
      return data
    },
    enabled: open,
    staleTime: 30_000,
    retry: false,
  })

  const loadMore = useCallback(() => {
    if (nextCursor && nextCursor !== '*') {
      setCursor(nextCursor)
    }
  }, [nextCursor])

  const invalidateMods = useCallback(
    () => {
      queryClient.invalidateQueries({ queryKey: ['globalMods'] })
      if (server) {
        queryClient.invalidateQueries({ queryKey: ['serverMods', server.id] })
      }
    },
    [queryClient, server],
  )

  // Add a single mod to the global library, optionally link to server, then
  // check Steam-layer dependencies.
  const handleAdd = useCallback(
    async (item: WorkshopItem) => {
      if (addingId) return
      setAddingId(item.workshop_id)
      setError(null)
      try {
        // Always add to global library first.
        const res = await globalModsApi.add({ workshopId: item.workshop_id, name: item.title })
        const newMod = res.data
        setAddedIds((prev) => new Set([...prev, item.workshop_id]))
        onAdded?.(newMod)

        // If used from a server context, also link the mod to the server.
        if (server) {
          await modsApi.link(server.id, { modId: newMod.id })
        }
        invalidateMods()

        // Resolve Steam-layer dependencies.
        const depRes = await steamApi.workshopDependencies(item.workshop_id)
        const allDeps = depRes.data.dependencies ?? []
        const currentIds = new Set([
          ...(existingServerMods ?? []).map((m) => m.workshop_id),
          ...addedIds,
          item.workshop_id,
        ])
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
    [addingId, addedIds, existingServerMods, server, invalidateMods, onAdded, t],
  )

  // Add all missing dependencies to the global library (and optionally the server).
  const handleAddAllDeps = useCallback(async () => {
    setAddingDeps(true)
    try {
      for (const dep of pendingDeps) {
        if (existingIds.has(dep.workshop_id) || addedIds.has(dep.workshop_id)) continue
        const res = await globalModsApi.add({ workshopId: dep.workshop_id, name: dep.title })
        if (server) {
          await modsApi.link(server.id, { modId: res.data.id })
        }
        setAddedIds((prev) => new Set([...prev, dep.workshop_id]))
        onAdded?.(res.data)
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
  }, [addedIds, existingIds, pendingDeps, server, invalidateMods, onAdded, t])

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

      {/* Missing deps confirmation */}
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
      {/* Thumbnail */}
      {item.preview_url ? (
        <img
          src={item.preview_url}
          alt={item.title}
          className="h-16 w-16 shrink-0 rounded-lg object-cover"
          loading="lazy"
        />
      ) : (
        <div className="h-16 w-16 shrink-0 rounded-lg bg-muted" />
      )}

      {/* Info */}
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

      {/* Actions */}
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
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={adding}
          >
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
