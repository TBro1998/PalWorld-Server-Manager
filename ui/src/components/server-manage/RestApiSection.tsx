'use client'

import { ExternalLink } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { useTranslations } from '@/contexts/LanguageContext'
import { SectionShell } from './shared'

// Representative endpoint groups from the official Palworld REST API, laid out
// as reserved rows. Wiring is intentionally deferred.
const ENDPOINTS = [
  { method: 'GET', path: '/v1/api/info' },
  { method: 'GET', path: '/v1/api/metrics' },
  { method: 'GET', path: '/v1/api/players' },
  { method: 'GET', path: '/v1/api/settings' },
  { method: 'POST', path: '/v1/api/announce' },
  { method: 'POST', path: '/v1/api/kick' },
  { method: 'POST', path: '/v1/api/ban' },
  { method: 'POST', path: '/v1/api/save' },
  { method: 'POST', path: '/v1/api/shutdown' },
]

export function RestApiSection() {
  const t = useTranslations('serverManage')
  return (
    <SectionShell title={t('restapi.title')} desc={t('restapi.desc')}>
      <a
        href="https://docs.palworldgame.com/category/rest-api"
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex items-center gap-1.5 text-sm font-semibold text-primary hover:underline"
      >
        <ExternalLink className="h-4 w-4" />
        {t('restapi.docs')}
      </a>
      <Card className="rounded-2xl border-2 shadow-pal">
        <CardContent className="divide-y divide-border/60 p-0">
          {ENDPOINTS.map((e) => (
            <div key={e.path} className="flex items-center gap-3 px-4 py-2.5 opacity-70">
              <span
                className={
                  'w-14 shrink-0 rounded-md px-2 py-0.5 text-center text-xs font-bold ' +
                  (e.method === 'GET' ? 'bg-info/15 text-info' : 'bg-success/15 text-success')
                }
              >
                {e.method}
              </span>
              <code className="truncate font-mono text-sm text-foreground">{e.path}</code>
            </div>
          ))}
        </CardContent>
      </Card>
    </SectionShell>
  )
}
