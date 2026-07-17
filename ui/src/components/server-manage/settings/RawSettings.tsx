'use client'

import { Textarea } from '@/components/ui/textarea'
import { useTranslations } from '@/contexts/LanguageContext'
import { SectionShell } from '../shared'
import { useSettingsDraft } from '../SettingsDraftContext'

// Raw OptionSettings editor. Editing the text switches the draft into raw mode,
// so the save writes the raw line verbatim instead of the structured settings.
export function RawSettings() {
  const t = useTranslations('serverConfig')
  const tm = useTranslations('serverManage')
  const { rawText, rawMode, installed, setRaw } = useSettingsDraft()

  return (
    <SectionShell title={tm('sections.raw')} desc={tm('rawSection.desc')} comingSoon={false}>
      {!installed && <p className="text-sm text-warning">{t('notInstalledHint')}</p>}
      <div className="space-y-2">
        <p className="text-xs text-muted-foreground">{t('rawHint')}</p>
        {rawMode && <p className="text-xs text-warning">{t('rawModeActive')}</p>}
        <Textarea
          value={rawText}
          onChange={(e) => setRaw(e.target.value)}
          className="min-h-[360px] font-mono text-xs"
          spellCheck={false}
        />
      </div>
    </SectionShell>
  )
}
