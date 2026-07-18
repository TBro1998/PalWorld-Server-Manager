'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { LayoutDashboard, Server, Package, Settings, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useTranslations } from '@/contexts/LanguageContext';
import { LanguageSwitcher } from './LanguageSwitcher';
import { ThemeToggle } from './ThemeToggle';

// Palworld-styled persistent navigation rail. On desktop it sits statically in
// the AppShell grid; on narrow screens AppShell renders it inside a slide-over
// drawer and passes `onNavigate` to close the drawer after a link is tapped.
export function Sidebar({ onNavigate }: { onNavigate?: () => void }) {
  const t = useTranslations('nav');
  const pathname = usePathname();

  const links = [
    { href: '/', label: t('home'), icon: LayoutDashboard, exact: true },
    { href: '/servers', label: t('servers'), icon: Server, exact: false },
    { href: '/mods', label: t('mods'), icon: Package, exact: false },
    { href: '/settings', label: t('settings'), icon: Settings, exact: false },
  ];

  return (
    <div className="flex h-full flex-col gap-6 bg-card/80 p-4 backdrop-blur-sm">
      {/* Brand */}
      <div className="flex items-center justify-between">
        <Link href="/" prefetch={false} onClick={onNavigate} className="flex items-center gap-2.5">
          <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-info text-primary-foreground shadow-pal">
            <Server className="h-5 w-5" />
          </div>
          <div className="leading-tight">
            <div className="text-base font-extrabold tracking-tight text-foreground">
              PalWorld
            </div>
            <div className="text-xs font-medium text-muted-foreground">
              Server Manager
            </div>
          </div>
        </Link>
        {onNavigate && (
          <button
            type="button"
            onClick={onNavigate}
            aria-label="Close menu"
            className="inline-flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground hover:bg-secondary lg:hidden"
          >
            <X className="h-5 w-5" />
          </button>
        )}
      </div>

      {/* Nav */}
      <nav className="flex flex-1 flex-col gap-1.5">
        {links.map(({ href, label, icon: Icon, exact }) => {
          const isActive = exact ? pathname === href : pathname.startsWith(href);
          return (
            <Link
              key={href}
              href={href}
              prefetch={false}
              onClick={onNavigate}
              className={cn(
                'flex items-center gap-3 rounded-xl px-3.5 py-2.5 text-sm font-semibold transition-all',
                isActive
                  ? 'bg-primary text-primary-foreground shadow-pal'
                  : 'text-muted-foreground hover:bg-secondary hover:text-foreground'
              )}
            >
              <Icon className="h-5 w-5 shrink-0" />
              {label}
            </Link>
          );
        })}
      </nav>

      {/* Footer controls */}
      <div className="flex flex-col gap-2 border-t border-border pt-4">
        <LanguageSwitcher />
        <ThemeToggle className="w-full" />
      </div>
    </div>
  );
}
