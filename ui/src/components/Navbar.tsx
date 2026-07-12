'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { LanguageSwitcher } from './LanguageSwitcher';
import { useTranslations } from '@/contexts/LanguageContext';
import { Server, Home } from 'lucide-react';
import { cn } from '@/lib/utils';

export function Navbar() {
  const t = useTranslations('nav');
  const pathname = usePathname();

  const links = [
    { href: '/', label: t('home'), icon: Home },
    { href: '/servers', label: t('servers'), icon: Server },
  ];

  return (
    <nav className="border-b bg-white/50 dark:bg-gray-900/50 backdrop-blur-sm sticky top-0 z-50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex justify-between items-center h-16">
          {/* Logo and Brand */}
          <Link href="/" className="flex items-center gap-2">
            <div className="w-8 h-8 bg-gradient-to-br from-blue-500 to-purple-600 rounded-lg flex items-center justify-center">
              <Server className="w-5 h-5 text-white" />
            </div>
            <span className="text-xl font-bold bg-gradient-to-r from-blue-600 to-purple-600 bg-clip-text text-transparent">
              PalWorld Manager
            </span>
          </Link>

          {/* Navigation Links */}
          <div className="flex items-center gap-1 sm:gap-2">
            {links.map(({ href, label, icon: Icon }) => {
              const isActive =
                href === '/' ? pathname === '/' : pathname.startsWith(href);
              return (
                <Link
                  key={href}
                  href={href}
                  className={cn(
                    'flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                    isActive
                      ? 'bg-gray-100 text-gray-900 dark:bg-gray-800 dark:text-white'
                      : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-800 dark:hover:text-white'
                  )}
                >
                  <Icon className="w-4 h-4" />
                  {label}
                </Link>
              );
            })}
          </div>

          {/* Language Switcher */}
          <div>
            <LanguageSwitcher />
          </div>
        </div>
      </div>
    </nav>
  );
}
