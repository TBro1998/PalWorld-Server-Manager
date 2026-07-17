'use client'

import { useTranslations } from '@/contexts/LanguageContext'
import { SectionShell, LaunchToggle, LaunchNumber } from '../shared'
import { useSettingsDraft } from '../SettingsDraftContext'

// Launch-args page: process launch flags (-players, thread flags, query port,
// public lobby, ...). The game port lives on the Basics page.
export function LaunchSettings() {
  const t = useTranslations('serverConfig')
  const tm = useTranslations('serverManage')
  const { launchArgs, setLaunch } = useSettingsDraft()

  return (
    <SectionShell title={tm('sections.launch')} desc={tm('launchSection.desc')} comingSoon={false}>
      <div className="max-w-2xl space-y-3 rounded-2xl border-2 p-5 shadow-pal">
        <LaunchNumber
          label={t('launch.players')}
          value={launchArgs.players}
          onChange={(v) => setLaunch({ players: v })}
        />
        <LaunchToggle
          label={t('launch.usePerfThreads')}
          checked={!!launchArgs.usePerfThreads}
          onChange={(c) => setLaunch({ usePerfThreads: c })}
        />
        <LaunchToggle
          label={t('launch.noAsyncLoadingThread')}
          checked={!!launchArgs.noAsyncLoadingThread}
          onChange={(c) => setLaunch({ noAsyncLoadingThread: c })}
        />
        <LaunchToggle
          label={t('launch.useMultithreadForDS')}
          checked={!!launchArgs.useMultithreadForDS}
          onChange={(c) => setLaunch({ useMultithreadForDS: c })}
        />
        <LaunchNumber
          label={t('launch.numberOfWorkerThreadsServer')}
          value={launchArgs.numberOfWorkerThreadsServer}
          onChange={(v) => setLaunch({ numberOfWorkerThreadsServer: v })}
        />
        <LaunchNumber
          label={t('launch.queryPort')}
          value={launchArgs.queryPort}
          onChange={(v) => setLaunch({ queryPort: v })}
          placeholder="27015"
        />
        <LaunchToggle
          label={t('launch.publicLobby')}
          checked={!!launchArgs.publicLobby}
          onChange={(c) => setLaunch({ publicLobby: c })}
        />
      </div>
    </SectionShell>
  )
}
