import { LanguageProvider } from '@/contexts/LanguageContext';
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
          {children}
        </LanguageProvider>
      </body>
    </html>
  );
}
