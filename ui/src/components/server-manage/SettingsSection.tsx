'use client'

import { Settings } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useTranslations } from '@/contexts/LanguageContext'
import { PanelCard } from './shared'

// Server settings entry. Reuses the still-functional ServerSettingsDialog owned
// by the manage page; `onOpen` triggers it.
export function SettingsSection({ onOpen }: { onOpen: () => void }) {
  const t = useTranslations('serverManage')
  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-xl font-bold text-foreground">{t('settings.title')}</h2>
        <p className="mt-1 text-sm text-muted-foreground">{t('settings.desc')}</p>
      </div>
      <PanelCard icon={<Settings className="h-4 w-4" />} title={t('settings.title')}>
        <p className="text-sm text-muted-foreground">{t('settings.hint')}</p>
        <Button onClick={onOpen} className="shadow-pal">
          <Settings className="mr-1 h-4 w-4" />
          {t('settings.open')}
        </Button>
      </PanelCard>
    </div>
  )
}
