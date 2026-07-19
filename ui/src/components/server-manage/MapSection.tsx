'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { UserPlus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { serversApi } from '@/lib/api'
import { getApiErrorMessage } from '@/lib/apiError'
import { useTranslations } from '@/contexts/LanguageContext'
import { SectionShell, Placeholder, PanelCard, useServerId } from './shared'

export function MapSection() {
  const t = useTranslations('serverManage')
  const serverId = useServerId()

  return (
    <SectionShell title={t('map.title')} desc={t('map.desc')} comingSoon={false}>
      <PanelCard icon={<UserPlus className="h-4 w-4" />} title={t('map.whitelist')}>
        <WhitelistPanel serverId={serverId} />
      </PanelCard>
    </SectionShell>
  )
}

// --- Whitelist panel ----------------------------------------------------------

function WhitelistPanel({ serverId }: { serverId: number }) {
  const t = useTranslations('serverManage')
  const queryClient = useQueryClient()
  const [newUid, setNewUid] = useState('')
  const [feedback, setFeedback] = useState<{ kind: 'success' | 'error'; text: string } | null>(null)

  const wlQuery = useQuery({
    queryKey: ['whitelist', serverId],
    queryFn: async () => (await serversApi.getWhitelist(serverId)).data.entries,
    enabled: Number.isFinite(serverId),
    refetchOnWindowFocus: false,
  })
  const entries = wlQuery.data ?? []

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['whitelist', serverId] })

  const addMut = useMutation({
    mutationFn: (uid: string) => serversApi.addWhitelist(serverId, uid),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('map.wl.addOk') })
      setNewUid('')
      invalidate()
    },
    onError: (err) =>
      setFeedback({ kind: 'error', text: getApiErrorMessage(err, t('map.wl.addFail')) }),
  })

  const removeMut = useMutation({
    mutationFn: (uid: string) => serversApi.removeWhitelist(serverId, uid),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('map.wl.removeOk') })
      invalidate()
    },
    onError: (err) =>
      setFeedback({ kind: 'error', text: getApiErrorMessage(err, t('map.wl.removeFail')) }),
  })

  const handleAdd = () => {
    const uid = newUid.trim()
    if (!uid) return
    setFeedback(null)
    addMut.mutate(uid)
  }

  return (
    <div className="space-y-3">
      <p className="text-xs text-muted-foreground">{t('map.wl.hint')}</p>

      {/* Add entry */}
      <div className="flex gap-2">
        <Input
          value={newUid}
          onChange={(e) => setNewUid(e.target.value)}
          placeholder={t('map.wl.placeholder')}
          onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
          className="font-mono text-xs"
        />
        <Button
          type="button"
          size="sm"
          onClick={handleAdd}
          disabled={!newUid.trim() || addMut.isPending}
          className="shrink-0"
        >
          <UserPlus className="h-4 w-4" />
        </Button>
      </div>

      {feedback && (
        <p
          className={
            'text-xs ' +
            (feedback.kind === 'success' ? 'text-success' : 'text-destructive')
          }
        >
          {feedback.text}
        </p>
      )}

      {/* Entry list */}
      {wlQuery.isLoading ? (
        <p className="text-xs text-muted-foreground">{t('map.wl.loading')}</p>
      ) : entries.length === 0 ? (
        <Placeholder className="min-h-[100px] text-xs">{t('map.wl.empty')}</Placeholder>
      ) : (
        <ul className="max-h-[480px] space-y-1 overflow-y-auto">
          {entries.map((uid) => (
            <li
              key={uid}
              className="flex items-center gap-2 rounded-lg border px-3 py-1.5"
            >
              <span className="flex-1 truncate font-mono text-xs text-foreground">{uid}</span>
              <button
                type="button"
                className="shrink-0 text-muted-foreground hover:text-destructive"
                onClick={() => {
                  setFeedback(null)
                  removeMut.mutate(uid)
                }}
                disabled={removeMut.isPending}
                aria-label={t('map.wl.remove')}
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
