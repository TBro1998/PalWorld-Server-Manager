'use client'

import { AlertTriangle } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useTranslations } from '@/contexts/LanguageContext'
import type { RestStatus, RestReason } from '@/types/server'

// Unified guidance shown by Overview / Players / Operations whenever the REST
// API is not usable. It maps status.reason to a localized explanation and, for
// configuration problems, points the user at the Settings section.
export function RestUnavailableNotice({ status }: { status?: RestStatus }) {
  const t = useTranslations('serverManage')

  // While the first status probe is in flight there is nothing actionable to
  // show yet; render a neutral loading line instead of a misleading reason.
  if (!status) {
    return (
      <Card className="rounded-2xl border-2 border-dashed shadow-none">
        <CardContent className="py-8 text-center text-sm text-muted-foreground">
          {t('rest.checking')}
        </CardContent>
      </Card>
    )
  }

  const reason: RestReason = status.reason || 'unreachable'
  const reasonKey = REASON_KEYS[reason] ?? 'unknown'
  // Enabling the API is a config change (INI edit + restart); surface the
  // settings hint only when that is what's actually missing.
  const showSettingsHint =
    reason === 'restapi_disabled' || reason === 'admin_password_empty'

  return (
    <Card className="rounded-2xl border-2 border-warning/40 bg-warning/5 shadow-pal">
      <CardContent className="flex flex-col gap-3 p-5">
        <div className="flex items-center gap-2">
          <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-warning/15 text-warning">
            <AlertTriangle className="h-4 w-4" />
          </span>
          <h3 className="font-bold text-foreground">{t('rest.unavailableTitle')}</h3>
          <Badge variant="warning" className="ml-auto">
            {t('rest.badge')}
          </Badge>
        </div>
        <p className="text-sm text-muted-foreground">{t(`rest.reason.${reasonKey}`)}</p>
        {showSettingsHint && (
          <p className="text-sm text-muted-foreground">{t('rest.settingsHint')}</p>
        )}
      </CardContent>
    </Card>
  )
}

// Reason codes come straight from the backend RestStatus.reason. Keeping the
// mapping here (rather than inline string concat) means a new reason only needs
// a key added, not scattered edits.
const REASON_KEYS: Record<RestReason, string> = {
  '': 'unknown',
  not_found: 'not_found',
  not_running: 'not_running',
  restapi_disabled: 'restapi_disabled',
  admin_password_empty: 'admin_password_empty',
  unreachable: 'unreachable',
}
