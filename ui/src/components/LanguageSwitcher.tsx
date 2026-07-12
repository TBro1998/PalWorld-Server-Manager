'use client';

import { useLanguage } from '@/contexts/LanguageContext';

const languages = {
  en: 'English',
  zh: '中文',
  ja: '日本語',
};

export function LanguageSwitcher() {
  const { locale, setLocale } = useLanguage();

  return (
    <select
      value={locale}
      onChange={(e) => setLocale(e.target.value as 'en' | 'zh' | 'ja')}
      className="px-3 py-2 border rounded-md bg-white dark:bg-gray-800"
    >
      {Object.entries(languages).map(([code, name]) => (
        <option key={code} value={code}>
          {name}
        </option>
      ))}
    </select>
  );
}
