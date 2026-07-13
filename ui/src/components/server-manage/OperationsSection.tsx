'use client'

import { UserX, Ban, Megaphone, Power } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useTranslations } from '@/contexts/LanguageContext'
import { SectionShell, Placeholder, PanelCard } from './shared'

// Kick / ban / broadcast / graceful shutdown. Layout reserved.
export function OperationsSection() {
  const t = useTranslations('serverManage')
  const ops = [
    { icon: UserX, key: 'kick' },
    { icon: Ban, key: 'ban' },
    { icon: Megaphone, key: 'broadcast' },
    { icon: Power, key: 'shutdown' },
  ] as const
  return (
    <SectionShell title={t('operations.title')} desc={t('operations.desc')}>
      <div className="grid gap-4 sm:grid-cols-2">
        {ops.map(({ icon: Icon, key }) => (
          <PanelCard key={key} icon={<Icon className="h-4 w-4" />} title={t(`operations.${key}`)}>
            <div className="space-y-3">
              <Placeholder className="min-h-[72px]">{t('comingSoonDesc')}</Placeholder>
              <Button size="sm" variant="outline" disabled className="w-full">
                {t(`operations.${key}`)}
              </Button>
            </div>
          </PanelCard>
        ))}
      </div>
    </SectionShell>
  )
}
