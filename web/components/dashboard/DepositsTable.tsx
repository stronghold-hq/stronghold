'use client';

import { motion } from 'framer-motion';
import { CreditCard, Wallet, AlertTriangle, Clock, CheckCircle, XCircle } from 'lucide-react';
import { SkeletonTableRow } from '@/components/ui/Skeleton';
import { formatDate, formatUSDC } from '@/lib/utils';
import type { Deposit } from '@/lib/hooks/useDeposits';

interface DepositsTableProps {
  deposits: Deposit[];
  loading: boolean;
  hasMore: boolean;
  onLoadMore: () => void;
}

const statusConfig = {
  pending: {
    icon: Clock,
    label: 'Pending',
    className: 'text-yellow-400 bg-yellow-500/10',
  },
  completed: {
    icon: CheckCircle,
    label: 'Completed',
    className: 'text-green-400 bg-green-500/10',
  },
  failed: {
    icon: XCircle,
    label: 'Failed',
    className: 'text-red-400 bg-red-500/10',
  },
};

export function DepositsTable({ deposits, loading, hasMore, onLoadMore }: DepositsTableProps) {
  if (loading && deposits.length === 0) {
    return (
      <div className="bg-[#111] border border-[#222] rounded-xl overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-[#222]">
              <th className="text-left text-gray-500 text-sm font-medium py-3 px-4">Date</th>
              <th className="text-left text-gray-500 text-sm font-medium py-3 px-4">Amount</th>
              <th className="text-left text-gray-500 text-sm font-medium py-3 px-4">Fee</th>
              <th className="text-left text-gray-500 text-sm font-medium py-3 px-4">Net</th>
              <th className="text-left text-gray-500 text-sm font-medium py-3 px-4">Provider</th>
              <th className="text-left text-gray-500 text-sm font-medium py-3 px-4">Status</th>
            </tr>
          </thead>
          <tbody>
            <SkeletonTableRow columns={6} />
            <SkeletonTableRow columns={6} />
            <SkeletonTableRow columns={6} />
          </tbody>
        </table>
      </div>
    );
  }

  if (!loading && deposits.length === 0) {
    return (
      <div className="bg-[#111] border border-[#222] rounded-xl p-12 text-center">
        <div className="w-16 h-16 rounded-full bg-[#1a1a1c] flex items-center justify-center mx-auto mb-4">
          <Wallet className="w-8 h-8 text-gray-600" />
        </div>
        <h3 className="text-white font-semibold mb-2">No deposits yet</h3>
        <p className="text-gray-500 text-sm mb-4">
          Add funds to your account to start using Stronghold.
        </p>
        <a
          href="/dashboard/main/deposit"
          className="inline-block py-2 px-4 bg-[#00D4AA] hover:bg-[#00b894] text-black font-semibold rounded-lg transition-colors text-sm"
        >
          Add Funds
        </a>
      </div>
    );
  }

  return (
    <div className="bg-[#111] border border-[#222] rounded-xl overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-[#222]">
              <th className="text-left text-gray-500 text-sm font-medium py-3 px-4">Date</th>
              <th className="text-left text-gray-500 text-sm font-medium py-3 px-4">Amount</th>
              <th className="text-left text-gray-500 text-sm font-medium py-3 px-4">Fee</th>
              <th className="text-left text-gray-500 text-sm font-medium py-3 px-4">Net</th>
              <th className="text-left text-gray-500 text-sm font-medium py-3 px-4">Provider</th>
              <th className="text-left text-gray-500 text-sm font-medium py-3 px-4">Status</th>
            </tr>
          </thead>
          <tbody>
            {deposits.map((deposit, index) => {
              const status = statusConfig[deposit.status];
              const StatusIcon = status.icon;

              return (
                <motion.tr
                  key={deposit.id}
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: index * 0.02 }}
                  className="border-b border-[#222] last:border-b-0 hover:bg-[#1a1a1c] transition-colors"
                >
                  <td className="py-3 px-4 text-gray-400 text-sm">
                    {formatDate(deposit.created_at)}
                  </td>
                  <td className="py-3 px-4 text-white text-sm font-mono">
                    ${formatUSDC(deposit.amount_usdc)}
                  </td>
                  <td className="py-3 px-4 text-gray-500 text-sm font-mono">
                    {deposit.fee_usdc > 0 ? `-$${formatUSDC(deposit.fee_usdc)}` : '—'}
                  </td>
                  <td className="py-3 px-4 text-green-400 text-sm font-mono">
                    +${formatUSDC(deposit.net_usdc)}
                  </td>
                  <td className="py-3 px-4">
                    <span className="inline-flex items-center gap-1.5 text-gray-300 text-sm">
                      {deposit.provider === 'stripe' ? (
                        <>
                          <CreditCard className="w-3.5 h-3.5" />
                          Card
                        </>
                      ) : (
                        <>
                          <Wallet className="w-3.5 h-3.5" />
                          Crypto
                        </>
                      )}
                    </span>
                  </td>
                  <td className="py-3 px-4">
                    <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 text-xs rounded-full ${status.className}`}>
                      <StatusIcon className="w-3 h-3" />
                      {status.label}
                    </span>
                  </td>
                </motion.tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {hasMore && (
        <div className="p-4 border-t border-[#222]">
          <button
            onClick={onLoadMore}
            disabled={loading}
            className="w-full py-2.5 text-sm text-gray-400 hover:text-white hover:bg-[#1a1a1c] rounded-lg transition-colors disabled:opacity-50"
          >
            {loading ? 'Loading...' : 'Load More'}
          </button>
        </div>
      )}
    </div>
  );
}
