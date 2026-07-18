'use client';
import { useEffect } from 'react';
import { useRouter } from 'next/navigation';

/**
 * Root redirect — `/` immediately forwards to `/servers`.
 * A dedicated homepage adds no value and increases interaction steps.
 */
export default function Home() {
  const router = useRouter();
  useEffect(() => {
    router.replace('/servers');
  }, [router]);
  return null;
}
