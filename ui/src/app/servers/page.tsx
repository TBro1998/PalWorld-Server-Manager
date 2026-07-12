'use client'

import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { serversApi } from '@/lib/api'
import { ServerCard } from '@/components/ServerCard'
import { AddServerDialog } from '@/components/AddServerDialog'
import { Button } from '@/components/ui/button'
import { Plus } from 'lucide-react'
import type { CreateServerData } from '@/types/server'

export default function ServersPage() {
  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false)
  const queryClient = useQueryClient()

  // Fetch servers with auto-refetch every 5 seconds to update statuses
  const { data: servers, isLoading } = useQuery({
    queryKey: ['servers'],
    queryFn: async () => {
      const response = await serversApi.list()
      return response.data
    },
    refetchInterval: 5000,
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
    if (window.confirm('Are you sure you want to delete this server?')) {
      deleteServerMutation.mutate(id)
    }
  }

  const nextServerId = servers ? servers.length + 1 : 1

  return (
    <div className="container mx-auto px-4 py-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-3xl font-bold text-gray-900 dark:text-gray-100">
          Servers
        </h1>
        <Button onClick={() => setIsAddDialogOpen(true)}>
          <Plus size={20} className="mr-2" />
          Add Server
        </Button>
      </div>

      {isLoading ? (
        <div className="text-center py-12">
          <p className="text-gray-500 dark:text-gray-400">Loading servers...</p>
        </div>
      ) : servers && servers.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {servers.map((server) => (
            <ServerCard
              key={server.id}
              server={server}
              onInstall={(id) => installServerMutation.mutate(id)}
              onStart={(id) => startServerMutation.mutate(id)}
              onStop={(id) => stopServerMutation.mutate(id)}
              onRestart={(id) => restartServerMutation.mutate(id)}
              onDelete={handleDelete}
            />
          ))}
        </div>
      ) : (
        <div className="text-center py-12 bg-gray-50 dark:bg-gray-800 rounded-lg">
          <p className="text-gray-500 dark:text-gray-400 mb-4">
            No servers yet. Create your first server to get started.
          </p>
          <Button onClick={() => setIsAddDialogOpen(true)}>
            <Plus size={20} className="mr-2" />
            Add Server
          </Button>
        </div>
      )}

      <AddServerDialog
        open={isAddDialogOpen}
        onOpenChange={setIsAddDialogOpen}
        onSubmit={(data) => createServerMutation.mutate(data)}
        isLoading={createServerMutation.isPending}
        nextServerId={nextServerId}
      />
    </div>
  )
}
