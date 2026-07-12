import { getTranslations } from 'next-intl/server';
import { setRequestLocale } from 'next-intl/server';

type Props = {
  params: Promise<{ locale: string }>;
};

export default async function HomePage({ params }: Props) {
  const { locale } = await params;

  // Enable static rendering
  setRequestLocale(locale);

  const t = await getTranslations('app');

  return (
    <main className="min-h-screen p-8">
      <h1 className="text-4xl font-bold mb-4">{t('title')}</h1>
      <p className="text-lg text-gray-600">{t('description')}</p>
    </main>
  );
}
