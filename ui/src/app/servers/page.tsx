'use client'

import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient, keepPreviousData } from '@tanstack/react-query'
import { serversApi } from '@/lib/api'
import { ServerCard } from '@/components/ServerCard'
import { AddServerDialog } from '@/components/AddServerDialog'
import { ServerSettingsDialog } from '@/components/ServerSettingsDialog'
import { ServerLogsDialog } from '@/components/ServerLogsDialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Plus, AlertTriangle, RotateCw } from 'lucide-react'
import { useTranslations } from '@/contexts/LanguageContext'
import type { CreateServerData, Server } from '@/types/server'

// A single shimmering placeholder card shown during the first load.
function SkeletonCard() {
  return (
    <Card className="rounded-2xl border-2">
      <CardContent className="space-y-4 p-6">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 animate-pulse rounded-xl bg-muted" />
          <div className="h-5 w-32 animate-pulse rounded bg-muted" />
        </div>
        <div className="space-y-2">
          <div className="h-4 w-full animate-pulse rounded bg-muted" />
          <div className="h-4 w-2/3 animate-pulse rounded bg-muted" />
          <div className="h-4 w-1/2 animate-pulse rounded bg-muted" />
        </div>
        <div className="flex gap-2">
          <div className="h-9 w-20 animate-pulse rounded-md bg-muted" />
          <div className="h-9 w-20 animate-pulse rounded-md bg-muted" />
        </div>
      </CardContent>
    </Card>
  )
}

export default function ServersPage() {
  const t = useTranslations('servers')
  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false)
  const [settingsServer, setSettingsServer] = useState<Server | null>(null)
  const [logsServer, setLogsServer] = useState<Server | null>(null)
  // SteamCMD install/update log viewer. Opens automatically on install/update
  // and streams live SteamCMD output; separate from the server runtime logs.
  const [installLogsServer, setInstallLogsServer] = useState<Server | null>(null)
  const queryClient = useQueryClient()

  // Fetch servers with auto-refetch every 5 seconds to update statuses.
  // keepPreviousData keeps the previous list on screen across refetches so the
  // grid does not flicker back to a loading/empty state while polling.
  const { data: servers, isLoading, isError, refetch } = useQuery({
    queryKey: ['servers'],
    queryFn: async () => {
      const response = await serversApi.list()
      return response.data
    },
    refetchInterval: 5000,
    placeholderData: keepPreviousData,
  })

  // Create server mutation
  const createServerMutation = useMutation({
    mutationFn: async (data: CreateServerData) => {
      const response = await serversApi.create(data)
      return response.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['servers'] })
      setIsAddDialogOpen(false)
    },
  })

  // Install server mutation
  const installServerMutation = useMutation({
    mutationFn: (id: number) => serversApi.install(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['servers'] })
    },
  })

  // Start server mutation
  const startServerMutation = useMutation({
    mutationFn: (id: number) => serversApi.start(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['servers'] })
    },
  })

  // Stop server mutation
  const stopServerMutation = useMutation({
    mutationFn: (id: number) => serversApi.stop(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['servers'] })
    },
  })

  // Restart server mutation
  const restartServerMutation = useMutation({
    mutationFn: (id: number) => serversApi.restart(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['servers'] })
    },
  })

  // Delete server mutation
  const deleteServerMutation = useMutation({
    mutationFn: (id: number) => serversApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['servers'] })
    },
  })

  const handleDelete = (id: number) => {
    if (window.confirm(t('confirmDelete'))) {
      deleteServerMutation.mutate(id)
    }
  }

  const nextServerId = servers ? servers.length + 1 : 1
  const count = servers?.length ?? 0

  return (
    <div className="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-10">
      {/* Toolbar */}
      <div className="mb-6 flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <h1 className="text-3xl font-extrabold tracking-tight text-foreground">
            {t('title')}
          </h1>
          {count > 0 && (
            <span className="rounded-full bg-primary/15 px-3 py-1 text-sm font-bold text-primary">
              {count}
            </span>
          )}
        </div>
        <Button onClick={() => setIsAddDialogOpen(true)} className="shadow-pal">
          <Plus size={20} className="mr-1" />
          {t('add')}
        </Button>
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          {[0, 1, 2].map((i) => (
            <SkeletonCard key={i} />
          ))}
        </div>
      ) : isError ? (
        <Card className="rounded-2xl border-2 border-destructive/40">
          <CardContent className="flex flex-col items-center gap-4 py-14 text-center">
            <div className="flex h-14 w-14 items-center justify-center rounded-3xl bg-destructive/10 text-destructive">
              <AlertTriangle className="h-7 w-7" />
            </div>
            <p className="text-muted-foreground">{t('loading')}</p>
            <Button variant="secondary" onClick={() => refetch()}>
              <RotateCw size={16} className="mr-1" />
              {t('restart')}
            </Button>
          </CardContent>
        </Card>
      ) : servers && servers.length > 0 ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          {servers.map((server) => (
            <ServerCard
              key={server.id}
              server={server}
              onInstall={(id) => {
                installServerMutation.mutate(id)
                // Auto-open the SteamCMD log viewer so the user sees install/
                // update output live. This is separate from the server logs.
                const s = servers?.find((x) => x.id === id)
                if (s) setInstallLogsServer(s)
              }}
              onStart={(id) => startServerMutation.mutate(id)}
              onStop={(id) => stopServerMutation.mutate(id)}
              onRestart={(id) => restartServerMutation.mutate(id)}
              onDelete={handleDelete}
              onSettings={(s) => setSettingsServer(s)}
              onLogs={(s) => setLogsServer(s)}
              onInstallLogs={(s) => setInstallLogsServer(s)}
            />
          ))}
        </div>
      ) : (
        <Card className="rounded-2xl border-2 border-dashed shadow-none">
          <CardContent className="flex flex-col items-center gap-4 py-16 text-center">
            <div className="flex h-16 w-16 items-center justify-center rounded-3xl bg-primary/10 text-4xl">
              🐾
            </div>
            <p className="max-w-sm text-muted-foreground">{t('empty')}</p>
            <Button onClick={() => setIsAddDialogOpen(true)} className="shadow-pal">
              <Plus size={20} className="mr-1" />
              {t('add')}
            </Button>
          </CardContent>
        </Card>
      )}

      <AddServerDialog
        open={isAddDialogOpen}
        onOpenChange={setIsAddDialogOpen}
        onSubmit={(data) => createServerMutation.mutate(data)}
        isLoading={createServerMutation.isPending}
        nextServerId={nextServerId}
      />

      <ServerSettingsDialog
        open={settingsServer !== null}
        onOpenChange={(open) => !open && setSettingsServer(null)}
        server={settingsServer}
      />

      <ServerLogsDialog
        open={logsServer !== null}
        onOpenChange={(open) => !open && setLogsServer(null)}
        server={logsServer}
        kind="server"
      />

      <ServerLogsDialog
        open={installLogsServer !== null}
        onOpenChange={(open) => !open && setInstallLogsServer(null)}
        server={installLogsServer}
        kind="steamcmd"
      />
    </div>
  )
}
