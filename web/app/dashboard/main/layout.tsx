'use client';

import { ErrorBoundary } from '@/components/ui/ErrorBoundary';

export default function MainDashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <ErrorBoundary>{children}</ErrorBoundary>;
}
