'use client'

import { Map as MapIcon, ListChecks } from 'lucide-react'
import { useTranslations } from '@/contexts/LanguageContext'
import { SectionShell, Placeholder, PanelCard } from './shared'

// Visual map + whitelist management. Layout reserved.
export function MapSection() {
  const t = useTranslations('serverManage')
  return (
    <SectionShell title={t('map.title')} desc={t('map.desc')}>
      <div className="grid gap-4 lg:grid-cols-[1fr_18rem]">
        <PanelCard icon={<MapIcon className="h-4 w-4" />} title={t('map.mapView')}>
          <Placeholder className="min-h-[320px]">{t('comingSoonDesc')}</Placeholder>
        </PanelCard>
        <PanelCard icon={<ListChecks className="h-4 w-4" />} title={t('map.whitelist')}>
          <Placeholder className="min-h-[320px]">{t('comingSoonDesc')}</Placeholder>
        </PanelCard>
      </div>
    </SectionShell>
  )
}
