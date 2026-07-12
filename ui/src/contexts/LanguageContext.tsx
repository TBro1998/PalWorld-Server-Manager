'use client';

import React, { createContext, useContext, useState, useEffect } from 'react';

type Locale = 'en' | 'zh' | 'ja';

// Loaded translation JSON is an arbitrarily nested object; we traverse it by
// dotted key at lookup time, so `unknown` values are narrowed there.
type Messages = Record<string, unknown>;

interface LanguageContextType {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  messages: Messages;
}

const LanguageContext = createContext<LanguageContextType | undefined>(undefined);

export function LanguageProvider({ children }: { children: React.ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>('zh');
  const [messages, setMessages] = useState<Messages>({});

  // Load locale from localStorage on mount. This must run in an effect (not a
  // lazy initializer): static export prerenders in Node where localStorage /
  // navigator are undefined, so the read is deferred to the client.
  useEffect(() => {
    const savedLocale = localStorage.getItem('locale') as Locale;
    if (savedLocale && ['en', 'zh', 'ja'].includes(savedLocale)) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- deferred client-only init
      setLocaleState(savedLocale);
    } else {
      // Detect browser language
      const browserLang = navigator.language.split('-')[0];
      if (['en', 'zh', 'ja'].includes(browserLang)) {
        setLocaleState(browserLang as Locale);
      }
    }
  }, []);

  // Load messages when locale changes
  useEffect(() => {
    import(`../../messages/${locale}.json`).then((module) => {
      setMessages(module.default);
    });
  }, [locale]);

  const setLocale = (newLocale: Locale) => {
    setLocaleState(newLocale);
    localStorage.setItem('locale', newLocale);
  };

  return (
    <LanguageContext.Provider value={{ locale, setLocale, messages }}>
      {children}
    </LanguageContext.Provider>
  );
}

export function useLanguage() {
  const context = useContext(LanguageContext);
  if (!context) {
    throw new Error('useLanguage must be used within LanguageProvider');
  }
  return context;
}

export function useTranslations(namespace?: string) {
  const { messages } = useLanguage();

  return (key: string) => {
    const fullKey = namespace ? `${namespace}.${key}` : key;
    const keys = fullKey.split('.');
    let value: unknown = messages;

    for (const k of keys) {
      value = (value as Record<string, unknown> | undefined)?.[k];
    }

    return typeof value === 'string' ? value : fullKey;
  };
}
