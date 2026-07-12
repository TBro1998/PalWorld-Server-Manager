'use client';

import { useTranslations } from '@/contexts/LanguageContext';
import { Button } from '@/components/ui/button';
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Server, Package, Activity, Shield, Zap, Globe } from 'lucide-react';
import Link from 'next/link';

export default function HomePage() {
  const t = useTranslations('app');
  const th = useTranslations('home');

  const features = [
    { icon: Server, key: 'serverMgmt', color: 'from-blue-500 to-cyan-500' },
    { icon: Package, key: 'modMgmt', color: 'from-purple-500 to-pink-500' },
    { icon: Activity, key: 'monitor', color: 'from-green-500 to-emerald-500' },
    { icon: Shield, key: 'security', color: 'from-orange-500 to-red-500' },
    { icon: Zap, key: 'deploy', color: 'from-yellow-500 to-orange-500' },
    { icon: Globe, key: 'i18n', color: 'from-indigo-500 to-purple-500' },
  ];

  return (
    <div>
      {/* Hero Section */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-br from-blue-50 via-purple-50 to-pink-50 dark:from-blue-950/20 dark:via-purple-950/20 dark:to-pink-950/20" />
        <div className="absolute inset-0 bg-grid-pattern opacity-10" />

        <div className="relative max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-24 sm:py-32">
          <div className="text-center">
            <h1 className="text-5xl sm:text-6xl lg:text-7xl font-bold tracking-tight mb-6">
              <span className="bg-gradient-to-r from-blue-600 via-purple-600 to-pink-600 bg-clip-text text-transparent">
                {t('title')}
              </span>
            </h1>
            <p className="text-xl sm:text-2xl text-gray-600 dark:text-gray-300 mb-8 max-w-3xl mx-auto">
              {t('description')}
            </p>
            <p className="text-lg text-gray-500 dark:text-gray-400 mb-12 max-w-2xl mx-auto">
              {th('subtitle')}
            </p>

            <div className="flex flex-col sm:flex-row gap-4 justify-center">
              <Link href="/servers">
                <Button size="lg" className="text-lg px-8 py-6 bg-gradient-to-r from-blue-600 to-purple-600 hover:from-blue-700 hover:to-purple-700">
                  <Server className="w-5 h-5 mr-2" />
                  {th('getStarted')}
                </Button>
              </Link>
              <Button size="lg" variant="outline" className="text-lg px-8 py-6">
                <Package className="w-5 h-5 mr-2" />
                {th('browseMods')}
              </Button>
            </div>
          </div>
        </div>
      </section>

      {/* Features Section */}
      <section className="relative py-24">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center mb-16">
            <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white mb-4">
              {th('featuresTitle')}
            </h2>
            <p className="text-lg text-gray-600 dark:text-gray-400 max-w-2xl mx-auto">
              {th('featuresSubtitle')}
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {features.map((feature) => {
              const Icon = feature.icon;
              return (
                <Card
                  key={feature.key}
                  className="border-2 hover:border-purple-200 dark:hover:border-purple-800 transition-all duration-300 hover:shadow-xl hover:-translate-y-1"
                >
                  <CardHeader>
                    <div className={`w-12 h-12 rounded-lg bg-gradient-to-br ${feature.color} flex items-center justify-center mb-4`}>
                      <Icon className="w-6 h-6 text-white" />
                    </div>
                    <CardTitle className="text-xl">{th(`features.${feature.key}.title`)}</CardTitle>
                    <CardDescription className="text-base">
                      {th(`features.${feature.key}.desc`)}
                    </CardDescription>
                  </CardHeader>
                </Card>
              );
            })}
          </div>
        </div>
      </section>

      {/* CTA Section */}
      <section className="relative py-24 bg-gradient-to-br from-blue-600 via-purple-600 to-pink-600">
        <div className="absolute inset-0 bg-grid-pattern opacity-10" />
        <div className="relative max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
          <h2 className="text-3xl sm:text-4xl font-bold text-white mb-6">
            {th('ctaTitle')}
          </h2>
          <p className="text-xl text-white/90 mb-8">
            {th('ctaSubtitle')}
          </p>
          <Link href="/servers">
            <Button size="lg" variant="secondary" className="text-lg px-8 py-6">
              <Server className="w-5 h-5 mr-2" />
              {th('ctaButton')}
            </Button>
          </Link>
        </div>
      </section>
    </div>
  );
}
