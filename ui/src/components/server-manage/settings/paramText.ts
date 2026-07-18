'use client'

import { useTranslations } from '@/contexts/LanguageContext'

// Shared helpers for resolving a config param's human label / description from
// the serverConfig.params.* i18n namespace, falling back to the raw key when a
// translation is missing (the translator returns the key path on a miss).
export function useParamText() {
  const t = useTranslations('serverConfig')
  const paramLabel = (key: string) => {
    const l = t(`params.${key}.label`)
    return l.includes('params.') ? key : l
  }
  const paramDesc = (key: string) => {
    const d = t(`params.${key}.desc`)
    return d.includes('params.') ? '' : d
  }
  return { paramLabel, paramDesc }
}

// OptionSettings keys promoted into the Basics page. They are edited there
// instead of the generic Game categories, so they are filtered out of those
// lists to avoid duplicate editors.
export const BASICS_INI_KEYS = new Set<string>([
  'ServerPassword',
  'AdminPassword',
  'ServerDescription',
  'RESTAPIEnabled',  // always True; hidden from UI but kept here so it never appears in generic form
  'RESTAPIPort',
])
