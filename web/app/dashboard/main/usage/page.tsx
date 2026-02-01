'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { motion } from 'framer-motion';
import { ArrowLeft, Activity, Wallet } from 'lucide-react';
import { useAuth } from '@/components/providers/AuthProvider';
import { StatsCards } from '@/components/dashboard/StatsCards';
import { UsageTable } from '@/components/dashboard/UsageTable';
import { DepositsTable } from '@/components/dashboard/DepositsTable';
import { LoadingOverlay } from '@/components/ui/LoadingSpinner';
import { useUsageLogs, useUsageStats } from '@/lib/hooks/useUsage';
import { useDeposits } from '@/lib/hooks/useDeposits';

type Tab = 'usage' | 'deposits';

export default function UsagePage() {
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const router = useRouter();
  const [activeTab, setActiveTab] = useState<Tab>('usage');

  const {
    data: logs,
    loading: logsLoading,
    hasMore: hasMoreLogs,
    fetchLogs,
    loadMore: loadMoreLogs,
  } = useUsageLogs();

  const {
    data: stats,
    loading: statsLoading,
    fetchStats,
  } = useUsageStats();

  const {
    data: deposits,
    loading: depositsLoading,
    hasMore: hasMoreDeposits,
    fetchDeposits,
    loadMore: loadMoreDeposits,
  } = useDeposits();

  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      router.replace('/dashboard/login');
    }
  }, [isAuthenticated, authLoading, router]);

  useEffect(() => {
    if (isAuthenticated) {
      fetchStats(30);
      fetchLogs(20, 0);
    }
  }, [isAuthenticated, fetchStats, fetchLogs]);

  useEffect(() => {
    if (isAuthenticated && activeTab === 'deposits' && deposits.length === 0) {
      fetchDeposits(20, 0);
    }
  }, [isAuthenticated, activeTab, deposits.length, fetchDeposits]);

  if (authLoading) {
    return <LoadingOverlay message="Checking authentication..." />;
  }

  if (!isAuthenticated) {
    return null;
  }

  return (
    <div className="min-h-screen bg-[#0a0a0a]">
      {/* Header */}
      <header className="border-b border-[#222]">
        <div className="max-w-6xl mx-auto px-4 py-4 flex items-center gap-4">
          <a
            href="/dashboard/main"
            className="p-2 text-gray-400 hover:text-white transition-colors"
          >
            <ArrowLeft className="w-5 h-5" />
          </a>
          <h1 className="text-xl font-bold text-white">Usage & Billing</h1>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-6xl mx-auto px-4 py-8">
        {/* Stats Cards */}
        <StatsCards stats={stats} loading={statsLoading} />

        {/* Tab Buttons */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
          className="flex gap-2 mb-6"
        >
          <button
            onClick={() => setActiveTab('usage')}
            className={`flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium transition-colors ${
              activeTab === 'usage'
                ? 'bg-[#00D4AA]/10 text-[#00D4AA] border border-[#00D4AA]/30'
                : 'bg-[#111] text-gray-400 border border-[#222] hover:text-white hover:border-[#333]'
            }`}
          >
            <Activity className="w-4 h-4" />
            Usage History
          </button>
          <button
            onClick={() => setActiveTab('deposits')}
            className={`flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium transition-colors ${
              activeTab === 'deposits'
                ? 'bg-[#00D4AA]/10 text-[#00D4AA] border border-[#00D4AA]/30'
                : 'bg-[#111] text-gray-400 border border-[#222] hover:text-white hover:border-[#333]'
            }`}
          >
            <Wallet className="w-4 h-4" />
            Deposits
          </button>
        </motion.div>

        {/* Tab Content */}
        <motion.div
          key={activeTab}
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.2 }}
        >
          {activeTab === 'usage' ? (
            <UsageTable
              logs={logs}
              loading={logsLoading}
              hasMore={hasMoreLogs}
              onLoadMore={loadMoreLogs}
            />
          ) : (
            <DepositsTable
              deposits={deposits}
              loading={depositsLoading}
              hasMore={hasMoreDeposits}
              onLoadMore={loadMoreDeposits}
            />
          )}
        </motion.div>
      </main>
    </div>
  );
}
