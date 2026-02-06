import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { CopyButton } from '@/components/ui/CopyButton'

// Mock the utils module
vi.mock('@/lib/utils', () => ({
  copyToClipboard: vi.fn(),
}))

import { copyToClipboard } from '@/lib/utils'
const mockCopyToClipboard = vi.mocked(copyToClipboard)

describe('CopyButton', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders Copy icon initially', () => {
    render(<CopyButton text="hello" />)
    const button = screen.getByTitle('Copy')
    expect(button).toBeInTheDocument()
  })

  it('calls copyToClipboard on click with correct text', async () => {
    const user = userEvent.setup()
    mockCopyToClipboard.mockResolvedValue(true)

    render(<CopyButton text="test-text-to-copy" />)

    const button = screen.getByTitle('Copy')
    await user.click(button)

    expect(mockCopyToClipboard).toHaveBeenCalledWith('test-text-to-copy')
  })

  it('shows Check icon after successful copy', async () => {
    const user = userEvent.setup()
    mockCopyToClipboard.mockResolvedValue(true)

    render(<CopyButton text="hello" />)

    const button = screen.getByTitle('Copy')
    await user.click(button)

    // After successful copy, the title should change to "Copied"
    await waitFor(() => {
      expect(screen.getByTitle('Copied')).toBeInTheDocument()
    })
  })

  it('reverts to Copy icon after 2 seconds', async () => {
    vi.useFakeTimers()
    mockCopyToClipboard.mockResolvedValue(true)

    render(<CopyButton text="hello" />)

    const button = screen.getByTitle('Copy')

    // Click the button
    await act(async () => {
      button.click()
      // Let the async copyToClipboard resolve
      await Promise.resolve()
    })

    // Should show Check icon now
    expect(screen.getByTitle('Copied')).toBeInTheDocument()

    // Advance timers by 2 seconds
    act(() => {
      vi.advanceTimersByTime(2000)
    })

    // Should revert back - no more green check icon
    expect(screen.getByTitle('Copy')).toBeInTheDocument()
    expect(screen.queryByTitle('Copied')).not.toBeInTheDocument()
  })

  it('handles copy failure gracefully', async () => {
    const user = userEvent.setup()
    mockCopyToClipboard.mockResolvedValue(false)

    render(<CopyButton text="hello" />)

    const button = screen.getByTitle('Copy')
    await user.click(button)

    // Should still show Copy icon (no green check)
    await waitFor(() => {
      expect(screen.queryByTitle('Copied')).not.toBeInTheDocument()
    })

    // Button should still be there with Copy title
    expect(screen.getByTitle('Copy')).toBeInTheDocument()
  })
})
