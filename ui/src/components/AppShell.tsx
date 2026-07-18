'use client';

import { useEffect, useState } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { Menu, Server } from 'lucide-react';
import { Sidebar } from './Sidebar';
import { cn } from '@/lib/utils';
import { authApi } from '@/lib/api';

// Pages that bypass the auth guard and hide the sidebar.
const AUTH_PATHS = ['/login', '/setup'];

// Persistent sidebar + scrollable content shell. Desktop: a fixed-width rail
// beside the content. Mobile: the rail collapses into a slide-over drawer
// opened from the top bar.
//
// Auth guard: on mount it calls GET /api/auth/status.
//   - configured=false  → redirect to /setup  (first-time password setup)
//   - configured=true   → require a JWT in localStorage; redirect to /login otherwise
// The guard is skipped on /login and /setup to avoid redirect loops.
export function AppShell({ children }: { children: React.ReactNode }) {
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [authChecked, setAuthChecked] = useState(false);
  const pathname = usePathname();
  const router = useRouter();

  const isAuthPage = AUTH_PATHS.some((p) => pathname === p || pathname.startsWith(p + '/'));

  useEffect(() => {
    // No guard needed on the auth pages themselves.
    if (isAuthPage) {
      setAuthChecked(true);
      return;
    }

    let cancelled = false;
    authApi.status().then((res) => {
      if (cancelled) return;
      const { configured } = res.data;
      if (!configured) {
        router.replace('/setup');
        return;
      }
      const token = localStorage.getItem('token');
      if (!token) {
        router.replace('/login');
        return;
      }
      setAuthChecked(true);
    }).catch(() => {
      // Network error or backend not ready — let the request interceptor handle
      // 401s; show the app so the user can at least see a loading state.
      if (!cancelled) setAuthChecked(true);
    });

    return () => { cancelled = true; };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pathname]);

  // While checking auth, render nothing (avoids layout flash).
  if (!authChecked) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-sky">
        <div className="flex flex-col items-center gap-3 text-muted-foreground">
          <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-info text-primary-foreground shadow-pal">
            <Server className="h-6 w-6" />
          </div>
          <span className="text-sm">Loading…</span>
        </div>
      </div>
    );
  }

  // Auth pages: full-screen, no sidebar.
  if (isAuthPage) {
    return <>{children}</>;
  }

  return (
    <div className="min-h-screen lg:grid lg:grid-cols-[17rem_1fr]">
      {/* Desktop sidebar */}
      <aside className="sticky top-0 hidden h-screen border-r border-border lg:block">
        <Sidebar />
      </aside>

      {/* Mobile drawer */}
      <div
        className={cn(
          'fixed inset-0 z-50 lg:hidden',
          drawerOpen ? 'pointer-events-auto' : 'pointer-events-none'
        )}
      >
        <div
          className={cn(
            'absolute inset-0 bg-foreground/40 backdrop-blur-sm transition-opacity',
            drawerOpen ? 'opacity-100' : 'opacity-0'
          )}
          onClick={() => setDrawerOpen(false)}
        />
        <div
          className={cn(
            'absolute left-0 top-0 h-full w-72 max-w-[85%] border-r border-border shadow-pal transition-transform duration-300',
            drawerOpen ? 'translate-x-0' : '-translate-x-full'
          )}
        >
          <Sidebar onNavigate={() => setDrawerOpen(false)} />
        </div>
      </div>

      {/* Content column */}
      <div className="flex min-h-screen flex-col bg-sky">
        {/* Mobile top bar */}
        <header className="sticky top-0 z-30 flex items-center gap-3 border-b border-border bg-card/70 px-4 py-3 backdrop-blur-sm lg:hidden">
          <button
            type="button"
            onClick={() => setDrawerOpen(true)}
            aria-label="Open menu"
            className="inline-flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-card text-foreground hover:bg-secondary"
          >
            <Menu className="h-5 w-5" />
          </button>
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-gradient-to-br from-primary to-info text-primary-foreground">
              <Server className="h-4 w-4" />
            </div>
            <span className="font-extrabold tracking-tight text-foreground">
              PalWorld
            </span>
          </div>
        </header>

        <main className="flex-1">{children}</main>
      </div>
    </div>
  );
}
