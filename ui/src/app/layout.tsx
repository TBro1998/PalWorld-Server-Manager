import { LanguageProvider } from '@/contexts/LanguageContext';
import { Providers } from '@/components/Providers';
import Link from 'next/link';
import './globals.css';

export const metadata = {
  title: 'Palworld Server Manager',
  description: 'Manage your Palworld dedicated servers',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>
        <LanguageProvider>
          <Providers>
            <div className="min-h-screen bg-background">
              <nav className="border-b">
                <div className="container mx-auto px-4 py-4">
                  <div className="flex items-center gap-6">
                    <Link href="/" className="text-lg font-bold">
                      Palworld Server Manager
                    </Link>
                    <Link href="/servers" className="text-sm hover:underline">
                      Servers
                    </Link>
                  </div>
                </div>
              </nav>
              <main>{children}</main>
            </div>
          </Providers>
        </LanguageProvider>
      </body>
    </html>
  );
}
