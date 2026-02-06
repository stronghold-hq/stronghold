'use client';

import { useState } from 'react';
import { Copy, Check } from 'lucide-react';
import { copyToClipboard } from '@/lib/utils';

interface CopyButtonProps {
  text: string;
  className?: string;
  iconClassName?: string;
}

export function CopyButton({ text, className = '', iconClassName = 'w-4 h-4' }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    const success = await copyToClipboard(text);
    if (success) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <button
      onClick={handleCopy}
      className={`p-1.5 text-gray-400 hover:text-white transition-colors ${className}`}
      title={copied ? "Copied" : "Copy"}
    >
      {copied ? (
        <Check className={`${iconClassName} text-[#00D4AA]`} />
      ) : (
        <Copy className={iconClassName} />
      )}
    </button>
  );
}
