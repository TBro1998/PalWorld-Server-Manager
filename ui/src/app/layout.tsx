import { Nunito } from 'next/font/google';
import { LanguageProvider } from '@/contexts/LanguageContext';
import { Providers } from '@/components/Providers';
import { AppShell } from '@/components/AppShell';
import './globals.css';

// Rounded, friendly sans for the Palworld cartoon look. Latin glyphs only;
// CJK falls back to system fonts via the --font-sans chain in globals.css.
const nunito = Nunito({
  subsets: ['latin'],
  weight: ['400', '600', '700', '800'],
  variable: '--font-nunito',
  display: 'swap',
});

export const metadata = {
  title: 'Palworld Server Manager',
  description: 'Manage your Palworld dedicated servers',
};

// Apply the saved theme before first paint to avoid a light/dark flash.
const themeInit = `(function(){try{var t=localStorage.getItem('theme');var m=window.matchMedia('(prefers-color-scheme: dark)').matches;if(t==='dark'||(!t&&m)){document.documentElement.classList.add('dark');}}catch(e){}})();`;

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className={nunito.variable}>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeInit }} />
      </head>
      <body>
        <LanguageProvider>
          <Providers>
            <AppShell>{children}</AppShell>
          </Providers>
        </LanguageProvider>
      </body>
    </html>
  );
}
