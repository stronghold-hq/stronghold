'use client';

import { useState } from 'react';
import { motion } from 'framer-motion';
import {
  ArrowLeft,
  Wallet,
  Check,
  Copy,
  LogOut,
  Shield,
} from 'lucide-react';
import { useAuth } from '@/components/providers/AuthProvider';
import { truncateAddress, copyToClipboard } from '@/lib/utils';

export default function SettingsPage() {
  const { account, logout } = useAuth();
  const [copied, setCopied] = useState(false);
  const [walletCopied, setWalletCopied] = useState(false);

  const handleCopyAccountNumber = async () => {
    if (account?.account_number) {
      const success = await copyToClipboard(account.account_number);
      if (success) {
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      }
    }
  };

  const handleCopyWalletAddress = async () => {
    if (account?.wallet_address) {
      const success = await copyToClipboard(account.wallet_address);
      if (success) {
        setWalletCopied(true);
        setTimeout(() => setWalletCopied(false), 2000);
      }
    }
  };

  const handleLogout = async () => {
    await logout();
    window.location.href = '/dashboard/login';
  };

  return (
    <div className="min-h-screen bg-[#0a0a0a]">
      {/* Header */}
      <header className="border-b border-[#222]">
        <div className="max-w-2xl mx-auto px-4 py-4 flex items-center gap-4">
          <a
            href="/dashboard/main"
            className="p-2 text-gray-400 hover:text-white transition-colors"
          >
            <ArrowLeft className="w-5 h-5" />
          </a>
          <h1 className="text-xl font-bold text-white">Settings</h1>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-2xl mx-auto px-4 py-8 space-y-6">
        {/* Account Section */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="bg-[#111] border border-[#222] rounded-2xl p-6"
        >
          <h2 className="text-lg font-semibold text-white mb-4">
            Account Information
          </h2>

          <div className="space-y-4">
            <div>
              <label className="block text-sm text-gray-400 mb-1">
                Account Number
              </label>
              <div className="flex items-center gap-2">
                <code className="flex-1 font-mono text-white bg-[#0a0a0a] rounded-lg px-3 py-2">
                  {account?.account_number}
                </code>
                <button
                  onClick={handleCopyAccountNumber}
                  className="p-2 text-gray-400 hover:text-white transition-colors"
                  title="Copy account number"
                >
                  {copied ? (
                    <Check className="w-4 h-4 text-[#00D4AA]" />
                  ) : (
                    <Copy className="w-4 h-4" />
                  )}
                </button>
              </div>
              <p className="text-gray-500 text-xs mt-2">
                This is your login credential. Keep it secure.
              </p>
            </div>
          </div>
        </motion.div>

        {/* Wallet Section */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="bg-[#111] border border-[#222] rounded-2xl p-6"
        >
          <h2 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
            <Wallet className="w-5 h-5 text-[#00D4AA]" />
            Wallet
          </h2>

          {account?.wallet_address ? (
            <div>
              <label className="block text-sm text-gray-400 mb-1">
                Linked Address
              </label>
              <div className="flex items-center gap-2">
                <code className="flex-1 font-mono text-white bg-[#0a0a0a] rounded-lg px-3 py-2 text-sm">
                  {truncateAddress(account.wallet_address, 20, 10)}
                </code>
                <button
                  onClick={handleCopyWalletAddress}
                  className="p-2 text-gray-400 hover:text-white transition-colors"
                  title="Copy wallet address"
                >
                  {walletCopied ? (
                    <Check className="w-4 h-4 text-[#00D4AA]" />
                  ) : (
                    <Copy className="w-4 h-4" />
                  )}
                </button>
                <span className="text-xs text-[#00D4AA] bg-[#00D4AA]/10 px-2 py-1 rounded">
                  Base
                </span>
              </div>
              <p className="text-gray-500 text-xs mt-2">
                Send USDC on Base network to this address for direct deposits.
              </p>
            </div>
          ) : (
            <div>
              <p className="text-gray-400">
                No wallet linked to this account.
              </p>
            </div>
          )}
        </motion.div>

        {/* Security Section */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
          className="bg-[#111] border border-[#222] rounded-2xl p-6"
        >
          <h2 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
            <Shield className="w-5 h-5 text-[#00D4AA]" />
            Security
          </h2>

          <div className="space-y-4">
            <div className="bg-yellow-500/10 border border-yellow-500/20 rounded-lg p-4">
              <h3 className="text-yellow-400 font-medium mb-2">
                Account Recovery
              </h3>
              <p className="text-yellow-300/80 text-sm mb-3">
                If you lose your account number, you cannot recover your account
                without your recovery file.
              </p>
              <a
                href="/dashboard/create"
                className="text-yellow-400 hover:underline text-sm"
              >
                Create new account →
              </a>
            </div>

            <button
              onClick={handleLogout}
              className="w-full py-2.5 px-4 bg-red-500/10 hover:bg-red-500/20 text-red-400 font-semibold rounded-lg transition-colors flex items-center justify-center gap-2"
            >
              <LogOut className="w-4 h-4" />
              Logout
            </button>
          </div>
        </motion.div>
      </main>
    </div>
  );
}
