import { LanguageProvider } from '@/contexts/LanguageContext';
import { Providers } from '@/components/Providers';
import { Navbar } from '@/components/Navbar';
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
            <div className="min-h-screen bg-gradient-to-br from-gray-50 via-white to-gray-50 dark:from-gray-900 dark:via-gray-800 dark:to-gray-900">
              <Navbar />
              <main>{children}</main>
            </div>
          </Providers>
        </LanguageProvider>
      </body>
    </html>
  );
}
