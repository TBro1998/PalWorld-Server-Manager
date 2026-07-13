'use client'

import { Cpu, MemoryStick, Signal, Clock, Server as ServerIcon, Users } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { useTranslations } from '@/contexts/LanguageContext'
import { SectionShell, Placeholder, PanelCard } from './shared'

// Server info, runtime metrics and online player list. Layout reserved.
export function OverviewSection() {
  const t = useTranslations('serverManage')
  const metrics = [
    { icon: Cpu, label: t('overview.cpu') },
    { icon: MemoryStick, label: t('overview.memory') },
    { icon: Signal, label: t('overview.online') },
    { icon: Clock, label: t('overview.uptime') },
  ]
  return (
    <SectionShell title={t('overview.title')} desc={t('overview.desc')}>
      {/* Metric tiles */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {metrics.map((m) => {
          const Icon = m.icon
          return (
            <Card key={m.label} className="rounded-2xl border-2 shadow-pal">
              <CardContent className="flex items-center gap-3 p-4">
                <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <Icon className="h-5 w-5" />
                </span>
                <div>
                  <div className="text-2xl font-extrabold leading-none text-foreground">
                    —
                  </div>
                  <div className="mt-1 text-xs font-medium text-muted-foreground">
                    {m.label}
                  </div>
                </div>
              </CardContent>
            </Card>
          )
        })}
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <PanelCard icon={<ServerIcon className="h-4 w-4" />} title={t('overview.info')}>
          <Placeholder className="min-h-[140px]">{t('comingSoonDesc')}</Placeholder>
        </PanelCard>
        <PanelCard icon={<Users className="h-4 w-4" />} title={t('overview.onlinePlayers')}>
          <Placeholder className="min-h-[140px]">{t('comingSoonDesc')}</Placeholder>
        </PanelCard>
      </div>
    </SectionShell>
  )
}
