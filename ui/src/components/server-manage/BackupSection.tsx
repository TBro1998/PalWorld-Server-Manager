'use client'

import { RefreshCw, Clock, Archive } from 'lucide-react'
import { useTranslations } from '@/contexts/LanguageContext'
import { SectionShell, Placeholder, PanelCard } from './shared'

// Scheduled save sync, auto backup and backup management. Layout reserved.
export function BackupSection() {
  const t = useTranslations('serverManage')
  return (
    <SectionShell title={t('backup.title')} desc={t('backup.desc')}>
      <div className="grid gap-4 lg:grid-cols-2">
        <PanelCard icon={<RefreshCw className="h-4 w-4" />} title={t('backup.sync')}>
          <Placeholder className="min-h-[120px]">{t('comingSoonDesc')}</Placeholder>
        </PanelCard>
        <PanelCard icon={<Clock className="h-4 w-4" />} title={t('backup.auto')}>
          <Placeholder className="min-h-[120px]">{t('comingSoonDesc')}</Placeholder>
        </PanelCard>
      </div>
      <PanelCard icon={<Archive className="h-4 w-4" />} title={t('backup.manage')}>
        <Placeholder className="min-h-[200px]">{t('comingSoonDesc')}</Placeholder>
      </PanelCard>
    </SectionShell>
  )
}
