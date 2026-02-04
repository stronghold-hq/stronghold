'use client'

import { motion } from 'framer-motion'
import Image from 'next/image'

interface LogoProps {
  className?: string
  size?: number
}

export default function Logo({ className = '', size = 40 }: LogoProps) {
  return (
    <motion.div
      className={className}
      initial={{ opacity: 0, scale: 0.9 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.5 }}
    >
      <Image
        src="/logo.png"
        alt="Stronghold"
        width={size}
        height={size}
        priority
      />
    </motion.div>
  )
}
