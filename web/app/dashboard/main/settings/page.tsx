'use client';

import { useState, useEffect, useCallback } from 'react';
import { motion } from 'framer-motion';
import {
  ArrowLeft,
  Wallet,
  Check,
  Copy,
  LogOut,
  Shield,
  RefreshCw,
  Building2,
  Mail,
} from 'lucide-react';
import { useAuth } from '@/components/providers/AuthProvider';
import { truncateAddress, copyToClipboard, formatUSDC } from '@/lib/utils';
import { fetchBalances, type BalancesResponse } from '@/lib/api';

export default function SettingsPage() {
  const { account, logout } = useAuth();
  const [copied, setCopied] = useState(false);
  const [evmCopied, setEvmCopied] = useState(false);
  const [solanaCopied, setSolanaCopied] = useState(false);
  const [balances, setBalances] = useState<BalancesResponse | null>(null);
  const [balancesLoading, setBalancesLoading] = useState(false);

  const isB2B = account?.account_type === 'b2b';

  const evmAddress = account?.evm_wallet_address;
  const solanaAddress = account?.solana_wallet_address;
  const hasAnyWallet = !!evmAddress || !!solanaAddress;

  const loadBalances = useCallback(async () => {
    if (!hasAnyWallet || isB2B) return;
    setBalancesLoading(true);
    try {
      const data = await fetchBalances();
      setBalances(data);
    } catch (err) {
      console.error('Failed to fetch balances:', err);
    } finally {
      setBalancesLoading(false);
    }
  }, [hasAnyWallet, isB2B]);

  useEffect(() => {
    loadBalances();
  }, [loadBalances]);

  const handleCopyAccountNumber = async () => {
    if (account?.account_number) {
      const success = await copyToClipboard(account.account_number);
      if (success) {
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      }
    }
  };

  const handleCopyEvmAddress = async () => {
    if (evmAddress) {
      const success = await copyToClipboard(evmAddress);
      if (success) {
        setEvmCopied(true);
        setTimeout(() => setEvmCopied(false), 2000);
      }
    }
  };

  const handleCopySolanaAddress = async () => {
    if (solanaAddress) {
      const success = await copyToClipboard(solanaAddress);
      if (success) {
        setSolanaCopied(true);
        setTimeout(() => setSolanaCopied(false), 2000);
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
            {isB2B ? (
              <>
                <div>
                  <label className="block text-sm text-gray-400 mb-1 flex items-center gap-1.5">
                    <Building2 className="w-3.5 h-3.5" />
                    Company Name
                  </label>
                  <div className="font-medium text-white bg-[#0a0a0a] rounded-lg px-3 py-2">
                    {account?.company_name}
                  </div>
                </div>
                <div>
                  <label className="block text-sm text-gray-400 mb-1 flex items-center gap-1.5">
                    <Mail className="w-3.5 h-3.5" />
                    Email
                  </label>
                  <div className="font-medium text-white bg-[#0a0a0a] rounded-lg px-3 py-2">
                    {account?.email}
                  </div>
                </div>
                {account?.stripe_customer_id && (
                  <div>
                    <label className="block text-sm text-gray-400 mb-1">
                      Stripe Customer ID
                    </label>
                    <code className="block font-mono text-gray-300 bg-[#0a0a0a] rounded-lg px-3 py-2 text-sm">
                      {account.stripe_customer_id}
                    </code>
                  </div>
                )}
                <div>
                  <label className="block text-sm text-gray-400 mb-1">
                    Account Type
                  </label>
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-[#00D4AA] bg-[#00D4AA]/10 px-2 py-1 rounded font-medium">
                      Business
                    </span>
                  </div>
                </div>
              </>
            ) : (
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
            )}
          </div>
        </motion.div>

        {/* Wallets Section - B2C only */}
        {!isB2B && (
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.1 }}
            className="bg-[#111] border border-[#222] rounded-2xl p-6"
          >
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold text-white flex items-center gap-2">
                <Wallet className="w-5 h-5 text-[#00D4AA]" />
                Wallets
              </h2>
              {hasAnyWallet && (
                <button
                  onClick={loadBalances}
                  disabled={balancesLoading}
                  className="p-1.5 text-gray-400 hover:text-white transition-colors disabled:opacity-50"
                  title="Refresh balances"
                >
                  <RefreshCw className={`w-4 h-4 ${balancesLoading ? 'animate-spin' : ''}`} />
                </button>
              )}
            </div>

            <div className="space-y-4">
              {/* EVM Wallet (Base) */}
              {evmAddress ? (
                <div>
                  <label className="block text-sm text-gray-400 mb-1">
                    EVM Address
                  </label>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 font-mono text-white bg-[#0a0a0a] rounded-lg px-3 py-2 text-sm">
                      {truncateAddress(evmAddress, 20, 10)}
                    </code>
                    <button
                      onClick={handleCopyEvmAddress}
                      className="p-2 text-gray-400 hover:text-white transition-colors"
                      title="Copy EVM wallet address"
                    >
                      {evmCopied ? (
                        <Check className="w-4 h-4 text-[#00D4AA]" />
                      ) : (
                        <Copy className="w-4 h-4" />
                      )}
                    </button>
                    <span className="text-xs text-[#00D4AA] bg-[#00D4AA]/10 px-2 py-1 rounded">
                      Base
                    </span>
                  </div>
                  {balances?.evm && !balances.evm.error && (
                    <p className="text-gray-400 text-xs mt-1">
                      Balance: <span className="text-white font-medium">{formatUSDC(balances.evm.balance_usdc)} USDC</span>
                    </p>
                  )}
                  {balances?.evm?.error && (
                    <p className="text-red-400/70 text-xs mt-1">
                      Could not fetch balance
                    </p>
                  )}
                </div>
              ) : (
                <div>
                  <label className="block text-sm text-gray-400 mb-1">
                    EVM Wallet (Base)
                  </label>
                  <p className="text-gray-500 text-sm">
                    Not configured. Set up via CLI: <code className="text-gray-400">stronghold init</code>
                  </p>
                </div>
              )}

              {/* Solana Wallet */}
              {solanaAddress ? (
                <div>
                  <label className="block text-sm text-gray-400 mb-1">
                    Solana Address
                  </label>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 font-mono text-white bg-[#0a0a0a] rounded-lg px-3 py-2 text-sm">
                      {truncateAddress(solanaAddress, 20, 10)}
                    </code>
                    <button
                      onClick={handleCopySolanaAddress}
                      className="p-2 text-gray-400 hover:text-white transition-colors"
                      title="Copy Solana wallet address"
                    >
                      {solanaCopied ? (
                        <Check className="w-4 h-4 text-[#00D4AA]" />
                      ) : (
                        <Copy className="w-4 h-4" />
                      )}
                    </button>
                    <span className="text-xs text-purple-400 bg-purple-400/10 px-2 py-1 rounded">
                      Solana
                    </span>
                  </div>
                  {balances?.solana && !balances.solana.error && (
                    <p className="text-gray-400 text-xs mt-1">
                      Balance: <span className="text-white font-medium">{formatUSDC(balances.solana.balance_usdc)} USDC</span>
                    </p>
                  )}
                  {balances?.solana?.error && (
                    <p className="text-red-400/70 text-xs mt-1">
                      Could not fetch balance
                    </p>
                  )}
                </div>
              ) : (
                <div>
                  <label className="block text-sm text-gray-400 mb-1">
                    Solana Wallet
                  </label>
                  <p className="text-gray-500 text-sm">
                    Not configured. Set up via CLI: <code className="text-gray-400">stronghold init</code>
                  </p>
                </div>
              )}
            </div>

            {hasAnyWallet && (
              <p className="text-gray-500 text-xs mt-4">
                <span className="text-[#00D4AA] font-medium">To fund your account:</span> Send USDC to the wallet address on the corresponding network. Balance updates automatically.
              </p>
            )}
          </motion.div>
        )}

        {/* Security Section */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: isB2B ? 0.1 : 0.2 }}
          className="bg-[#111] border border-[#222] rounded-2xl p-6"
        >
          <h2 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
            <Shield className="w-5 h-5 text-[#00D4AA]" />
            Security
          </h2>

          <div className="space-y-4">
            {!isB2B && (
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
            )}

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
