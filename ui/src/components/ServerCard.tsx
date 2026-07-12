import React from 'react'
import { Server } from '@/types/server'
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from './ui/card'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Play, Square, RotateCw, Trash2, Download } from 'lucide-react'

interface ServerCardProps {
  server: Server
  onInstall: (id: number) => void
  onStart: (id: number) => void
  onStop: (id: number) => void
  onRestart: (id: number) => void
  onDelete: (id: number) => void
}

const statusConfig = {
  stopped: { variant: 'secondary' as const, label: 'Stopped' },
  running: { variant: 'default' as const, label: 'Running' },
  installing: { variant: 'outline' as const, label: 'Installing' },
  error: { variant: 'destructive' as const, label: 'Error' },
}

export function ServerCard({
  server,
  onInstall,
  onStart,
  onStop,
  onRestart,
  onDelete,
}: ServerCardProps) {
  const status = statusConfig[server.status]

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-lg">{server.name}</CardTitle>
          <Badge variant={status.variant}>{status.label}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-2 text-sm text-gray-600 dark:text-gray-400">
          <div>
            <span className="font-medium">Path:</span> {server.install_path}
          </div>
          <div>
            <span className="font-medium">Port:</span> {server.port}
          </div>
        </div>
      </CardContent>
      <CardFooter>
        <div className="flex gap-2 flex-wrap">
          {server.status === 'stopped' && (
            <>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => onInstall(server.id)}
              >
                <Download size={16} className="mr-1" />
                Install
              </Button>
              <Button
                size="sm"
                variant="default"
                onClick={() => onStart(server.id)}
              >
                <Play size={16} className="mr-1" />
                Start
              </Button>
            </>
          )}
          {server.status === 'running' && (
            <>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => onStop(server.id)}
              >
                <Square size={16} className="mr-1" />
                Stop
              </Button>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => onRestart(server.id)}
              >
                <RotateCw size={16} className="mr-1" />
                Restart
              </Button>
            </>
          )}
          {server.status === 'installing' && (
            <div className="text-sm text-gray-500 dark:text-gray-400 flex items-center">
              <div className="animate-spin mr-2">⏳</div>
              Installing...
            </div>
          )}
          <Button
            size="sm"
            variant="destructive"
            onClick={() => onDelete(server.id)}
            disabled={server.status === 'running' || server.status === 'installing'}
          >
            <Trash2 size={16} className="mr-1" />
            Delete
          </Button>
        </div>
      </CardFooter>
    </Card>
  )
}
