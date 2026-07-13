'use client'

import { useTranslations } from '@/contexts/LanguageContext'
import { SectionShell, Placeholder } from './shared'

// Player / guild / Pal / inventory data viewer. Layout reserved.
export function PlayersSection() {
  const t = useTranslations('serverManage')
  const tabs = ['players', 'guilds', 'pals', 'inventory'] as const
  return (
    <SectionShell title={t('players.title')} desc={t('players.desc')}>
      {/* Reserved inner tab bar for the four data domains. */}
      <div className="flex flex-wrap gap-1.5">
        {tabs.map((tb, i) => (
          <span
            key={tb}
            className={
              'rounded-full px-3.5 py-1.5 text-sm font-semibold ' +
              (i === 0
                ? 'bg-primary text-primary-foreground'
                : 'bg-secondary text-muted-foreground')
            }
          >
            {t(`players.tabs.${tb}`)}
          </span>
        ))}
      </div>
      <Placeholder className="min-h-[280px]">{t('comingSoonDesc')}</Placeholder>
    </SectionShell>
  )
}
