'use client';

import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import {
  Server,
  Play,
  Square,
  AlertTriangle,
  Plus,
  ArrowRight,
  Package,
  Activity,
  Shield,
  Zap,
  Globe,
} from 'lucide-react';
import { serversApi } from '@/lib/api';
import type { Server as ServerType } from '@/types/server';
import { useTranslations } from '@/contexts/LanguageContext';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';

const statusBadge: Record<
  ServerType['status'],
  { variant: 'success' | 'secondary' | 'info' | 'destructive'; key: string }
> = {
  running: { variant: 'success', key: 'statusRunning' },
  stopped: { variant: 'secondary', key: 'statusStopped' },
  installing: { variant: 'info', key: 'statusInstalling' },
  error: { variant: 'destructive', key: 'statusError' },
};

export default function DashboardPage() {
  const t = useTranslations('dashboard');
  const ts = useTranslations('servers');
  const th = useTranslations('home');
  const tapp = useTranslations('app');

  const { data: servers } = useQuery({
    queryKey: ['servers'],
    queryFn: async () => (await serversApi.list()).data,
    refetchInterval: 5000,
  });

  const list = servers ?? [];
  const total = list.length;
  const running = list.filter((s) => s.status === 'running').length;
  const stopped = list.filter((s) => s.status === 'stopped').length;
  const attention = list.filter(
    (s) => s.status === 'error' || !s.installed
  ).length;

  const stats = [
    { label: t('statTotal'), value: total, icon: Server, tone: 'primary' },
    { label: t('statRunning'), value: running, icon: Play, tone: 'success' },
    { label: t('statStopped'), value: stopped, icon: Square, tone: 'muted' },
    {
      label: t('statAttention'),
      value: attention,
      icon: AlertTriangle,
      tone: 'warning',
    },
  ] as const;

  const toneClass: Record<string, string> = {
    primary: 'bg-primary/15 text-primary',
    success: 'bg-success/15 text-success',
    muted: 'bg-muted text-muted-foreground',
    warning: 'bg-warning/20 text-warning',
  };

  const features = [
    { icon: Server, key: 'serverMgmt' },
    { icon: Package, key: 'modMgmt' },
    { icon: Activity, key: 'monitor' },
    { icon: Shield, key: 'security' },
    { icon: Zap, key: 'deploy' },
    { icon: Globe, key: 'i18n' },
  ];

  return (
    <div className="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-10">
      {/* Welcome header */}
      <section className="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="text-sm font-semibold text-primary">{t('welcome')} 👋</p>
          <h1 className="mt-1 text-3xl font-extrabold tracking-tight text-foreground sm:text-4xl">
            {tapp('title')}
          </h1>
          <p className="mt-2 text-muted-foreground">{t('subtitle')}</p>
        </div>
        <div className="flex gap-3">
          <Link href="/servers" prefetch={false}>
            <Button size="lg" className="shadow-pal">
              <Server className="mr-1 h-5 w-5" />
              {t('manageServers')}
            </Button>
          </Link>
        </div>
      </section>

      {/* Stat cards */}
      <section className="mb-10 grid grid-cols-2 gap-4 lg:grid-cols-4">
        {stats.map((s) => {
          const Icon = s.icon;
          return (
            <Card
              key={s.label}
              className="rounded-2xl border-2 shadow-pal transition-transform hover:-translate-y-1"
            >
              <CardContent className="flex items-center gap-4 p-5">
                <div
                  className={`flex h-12 w-12 items-center justify-center rounded-2xl ${toneClass[s.tone]}`}
                >
                  <Icon className="h-6 w-6" />
                </div>
                <div>
                  <div className="text-3xl font-extrabold leading-none text-foreground">
                    {s.value}
                  </div>
                  <div className="mt-1 text-sm font-medium text-muted-foreground">
                    {s.label}
                  </div>
                </div>
              </CardContent>
            </Card>
          );
        })}
      </section>

      {/* Servers overview */}
      <section className="mb-10">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-xl font-bold text-foreground">
            {t('serversOverview')}
          </h2>
          {total > 0 && (
            <Link
              href="/servers"
              prefetch={false}
              className="inline-flex items-center gap-1 text-sm font-semibold text-primary hover:underline"
            >
              {t('viewAll')}
              <ArrowRight className="h-4 w-4" />
            </Link>
          )}
        </div>

        {total === 0 ? (
          <Card className="rounded-2xl border-2 border-dashed shadow-none">
            <CardContent className="flex flex-col items-center gap-4 py-14 text-center">
              <div className="flex h-16 w-16 items-center justify-center rounded-3xl bg-primary/10 text-4xl">
                🐾
              </div>
              <p className="max-w-sm text-muted-foreground">{t('empty')}</p>
              <Link href="/servers" prefetch={false}>
                <Button size="lg" className="shadow-pal">
                  <Plus className="mr-1 h-5 w-5" />
                  {t('addServer')}
                </Button>
              </Link>
            </CardContent>
          </Card>
        ) : (
          <div className="grid gap-3">
            {list.slice(0, 5).map((s) => {
              const badge = statusBadge[s.status];
              return (
                <Link key={s.id} href="/servers" prefetch={false}>
                  <Card className="rounded-2xl border-2 transition-all hover:-translate-y-0.5 hover:shadow-pal">
                    <CardContent className="flex items-center justify-between gap-3 p-4">
                      <div className="flex min-w-0 items-center gap-3">
                        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                          <Server className="h-5 w-5" />
                        </div>
                        <div className="min-w-0">
                          <div className="truncate font-semibold text-foreground">
                            {s.name}
                          </div>
                          <div className="truncate text-xs text-muted-foreground">
                            {s.install_path}
                          </div>
                        </div>
                      </div>
                      <Badge variant={badge.variant}>{ts(badge.key)}</Badge>
                    </CardContent>
                  </Card>
                </Link>
              );
            })}
          </div>
        )}
      </section>

      {/* Features strip */}
      <section>
        <h2 className="mb-4 text-xl font-bold text-foreground">
          {t('featuresTitle')}
        </h2>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {features.map((f) => {
            const Icon = f.icon;
            return (
              <Card
                key={f.key}
                className="rounded-2xl border-2 transition-all hover:-translate-y-1 hover:shadow-pal"
              >
                <CardContent className="flex items-start gap-3 p-5">
                  <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-info text-primary-foreground">
                    <Icon className="h-5 w-5" />
                  </div>
                  <div>
                    <div className="font-bold text-foreground">
                      {th(`features.${f.key}.title`)}
                    </div>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {th(`features.${f.key}.desc`)}
                    </p>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      </section>
    </div>
  );
}
