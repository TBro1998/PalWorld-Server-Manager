'use client'

import { useMemo, useState } from 'react'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Select } from '@/components/ui/select'
import { useTranslations } from '@/contexts/LanguageContext'
import type { ConfigParamDef } from '@/types/server'
import { SectionShell, Placeholder } from '../shared'
import { useSettingsDraft } from '../SettingsDraftContext'
import { useParamText, BASICS_INI_KEYS } from './paramText'

// The four INI param groups, shown as internal sub-tabs within the single
// "Game" nav item (balanced split: keeps the left nav short).
const CATEGORIES = ['performances', 'serverManagement', 'features', 'gameBalances'] as const
type Category = (typeof CATEGORIES)[number]

export function GameSettings() {
  const t = useTranslations('serverConfig')
  const tm = useTranslations('serverManage')
  const { paramLabel, paramDesc } = useParamText()
  const { params, settings, setSetting, loading, installed } = useSettingsDraft()

  const [cat, setCat] = useState<Category>('performances')

  const paramsByCategory = useMemo(() => {
    const map: Record<string, ConfigParamDef[]> = {}
    for (const p of params) (map[p.category] ??= []).push(p)
    return map
  }, [params])

  const renderControl = (p: ConfigParamDef) => {
    const value = settings[p.key] ?? p.default
    switch (p.type) {
      case 'bool':
        return (
          <Switch
            checked={value === 'True'}
            onCheckedChange={(c) => setSetting(p.key, c ? 'True' : 'False')}
          />
        )
      case 'enum':
        return (
          <Select
            value={value}
            onChange={(e) => setSetting(p.key, e.target.value)}
            className="max-w-[220px]"
          >
            {(p.options ?? []).map((opt) => (
              <option key={opt} value={opt}>
                {opt}
              </option>
            ))}
          </Select>
        )
      case 'int':
      case 'float':
        return (
          <Input
            type="number"
            step={p.type === 'float' ? 'any' : '1'}
            value={value}
            onChange={(e) => setSetting(p.key, e.target.value)}
            className="max-w-[220px]"
          />
        )
      default:
        return (
          <Input
            value={value}
            onChange={(e) => setSetting(p.key, e.target.value)}
            className="max-w-[220px]"
          />
        )
    }
  }

  const list = (paramsByCategory[cat] ?? []).filter(
    (p) => p.key !== 'ServerName' && !BASICS_INI_KEYS.has(p.key),
  )

  return (
    <SectionShell title={tm('sections.game')} desc={tm('gameSection.desc')} comingSoon={false}>
      {!installed && <p className="text-sm text-warning">{t('notInstalledHint')}</p>}

      {/* Internal category sub-tabs */}
      <div className="flex flex-wrap gap-1.5">
        {CATEGORIES.map((c) => {
          const active = c === cat
          return (
            <button
              key={c}
              type="button"
              onClick={() => setCat(c)}
              className={
                'rounded-full px-3.5 py-1.5 text-sm font-semibold transition-colors ' +
                (active
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-secondary text-muted-foreground hover:text-foreground')
              }
            >
              {t(`tabs.${c}`)}
            </button>
          )
        })}
      </div>

      {loading ? (
        <Placeholder className="min-h-[160px]">{t('loading')}</Placeholder>
      ) : list.length === 0 ? (
        <Placeholder className="min-h-[160px]">{t('loading')}</Placeholder>
      ) : (
        <div className="space-y-3 rounded-2xl border-2 p-5 shadow-pal">
          {list.map((p) => (
            <div
              key={p.key}
              className="flex items-start justify-between gap-4 border-b border-dashed pb-2 last:border-0"
            >
              <div className="min-w-0">
                <div className="text-sm font-medium">{paramLabel(p.key)}</div>
                <div className="font-mono text-xs text-muted-foreground">{p.key}</div>
                {paramDesc(p.key) && (
                  <div className="mt-0.5 text-xs text-muted-foreground">{paramDesc(p.key)}</div>
                )}
              </div>
              <div className="shrink-0">{renderControl(p)}</div>
            </div>
          ))}
        </div>
      )}
    </SectionShell>
  )
}
