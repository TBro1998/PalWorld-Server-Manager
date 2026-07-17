'use client'

import { AlertTriangle, Save } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useTranslations } from '@/contexts/LanguageContext'
import { useSettingsDraft } from './SettingsDraftContext'

// Sticky save bar for the inline settings pages. Renders only when the shared
// draft is dirty; it is shared across all config sub-pages so switching pages
// keeps unsaved edits and the bar visible. Save commits everything atomically.
export function SettingsSaveBar() {
  const t = useTranslations('serverManage')
  const { isDirty, dirtyCount, saving, installed, error, save, discard } = useSettingsDraft()

  if (!isDirty) return null

  return (
    <div className="sticky bottom-0 z-20 mt-6 -mx-1">
      <div className="flex flex-col gap-2 rounded-2xl border-2 border-primary/40 bg-card/95 p-3 shadow-pal-lg backdrop-blur-sm sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2 text-sm">
          <AlertTriangle className="h-4 w-4 shrink-0 text-warning" />
          <span className="font-semibold text-foreground">
            {t('save.unsaved')} · {dirtyCount}
          </span>
          {!installed && (
            <span className="text-xs text-muted-foreground">{t('save.metaOnly')}</span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {error && <span className="mr-1 text-sm text-destructive">{error}</span>}
          <Button type="button" variant="outline" size="sm" onClick={discard} disabled={saving}>
            {t('save.discard')}
          </Button>
          <Button type="button" size="sm" onClick={save} disabled={saving}>
            <Save className="mr-1 h-4 w-4" />
            {saving ? t('save.saving') : t('save.save')}
          </Button>
        </div>
      </div>
    </div>
  )
}
