'use client';

import { useTranslations } from '@/contexts/LanguageContext';
import { LanguageSwitcher } from '@/components/LanguageSwitcher';

export default function HomePage() {
  const t = useTranslations('app');

  return (
    <main className="min-h-screen p-8">
      <div className="flex justify-end mb-8">
        <LanguageSwitcher />
      </div>
      <h1 className="text-4xl font-bold mb-4">{t('title')}</h1>
      <p className="text-lg text-gray-600">{t('description')}</p>
    </main>
  );
}
