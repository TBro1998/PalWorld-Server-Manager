'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Server, Eye, EyeOff, ShieldCheck } from 'lucide-react';
import { authApi } from '@/lib/api';
import { useTranslations } from '@/contexts/LanguageContext';

export default function SetupPage() {
  const t = useTranslations('auth');
  const router = useRouter();
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const mismatch = confirm.length > 0 && password !== confirm;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (password !== confirm) {
      setError(t('passwordMismatch'));
      return;
    }
    if (password.length < 6) {
      setError(t('passwordTooShort'));
      return;
    }
    setError('');
    setLoading(true);
    try {
      const res = await authApi.setup(password);
      localStorage.setItem('token', res.data.token);
      router.replace('/servers');
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        t('setupFailed');
      setError(msg);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-sky px-4">
      <div className="w-full max-w-sm rounded-2xl border border-border bg-card p-8 shadow-pal">
        {/* Brand */}
        <div className="mb-8 flex flex-col items-center gap-3">
          <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-info text-primary-foreground shadow-pal">
            <Server className="h-7 w-7" />
          </div>
          <div className="text-center">
            <h1 className="text-xl font-extrabold tracking-tight text-foreground">
              PalWorld Server Manager
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">{t('setupSubtitle')}</p>
          </div>
        </div>

        {/* Info banner */}
        <div className="mb-6 flex items-start gap-2.5 rounded-xl border border-info/30 bg-info/10 px-3.5 py-3 text-sm text-info">
          <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0" />
          <span>{t('setupHint')}</span>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          {/* Password field */}
          <div className="flex flex-col gap-1.5">
            <label htmlFor="password" className="text-sm font-semibold text-foreground">
              {t('newPassword')}
            </label>
            <div className="relative">
              <input
                id="password"
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={t('passwordMinLength')}
                autoComplete="new-password"
                required
                className="w-full rounded-xl border border-border bg-background px-3.5 py-2.5 pr-10 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
              />
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                aria-label={showPassword ? t('hidePassword') : t('showPassword')}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              >
                {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
          </div>

          {/* Confirm field */}
          <div className="flex flex-col gap-1.5">
            <label htmlFor="confirm" className="text-sm font-semibold text-foreground">
              {t('confirmPassword')}
            </label>
            <input
              id="confirm"
              type={showPassword ? 'text' : 'password'}
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              placeholder={t('confirmPasswordPlaceholder')}
              autoComplete="new-password"
              required
              className={`w-full rounded-xl border bg-background px-3.5 py-2.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary ${
                mismatch ? 'border-destructive' : 'border-border'
              }`}
            />
            {mismatch && (
              <p className="text-xs text-destructive">{t('passwordMismatch')}</p>
            )}
          </div>

          {/* Error */}
          {error && (
            <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error}
            </p>
          )}

          {/* Submit */}
          <button
            type="submit"
            disabled={loading || !password || !confirm || mismatch}
            className="mt-1 rounded-xl bg-primary px-4 py-2.5 text-sm font-semibold text-primary-foreground shadow-pal transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loading ? t('settingUp') : t('setup')}
          </button>
        </form>
      </div>
    </div>
  );
}
