import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import CreateAccountPage from '@/app/dashboard/create/page'
import { AuthProvider } from '@/components/providers/AuthProvider'
import { mockAccount } from '../test-utils'

// Mock next/navigation
const mockPush = vi.fn()
const mockReplace = vi.fn()
vi.mock('next/navigation', () => ({
  useRouter: () => ({
    push: mockPush,
    replace: mockReplace,
    prefetch: vi.fn(),
  }),
  usePathname: () => '/dashboard/create',
}))

// Mock downloadTextFile and copyToClipboard
vi.mock('@/lib/utils', () => ({
  downloadTextFile: vi.fn(),
  copyToClipboard: vi.fn(),
}))
import { downloadTextFile } from '@/lib/utils'

describe('CreateAccountPage', () => {
  const originalFetch = global.fetch

  beforeEach(() => {
    vi.clearAllMocks()
    mockPush.mockClear()
    mockReplace.mockClear()
  })

  afterEach(() => {
    global.fetch = originalFetch
  })

  it('renders create form initially', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ error: 'Unauthorized' }),
    })

    render(
      <AuthProvider>
        <CreateAccountPage />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /create account/i })).toBeInTheDocument()
    })

    expect(screen.getByText(/no email required/i)).toBeInTheDocument()
  })

  it('shows loading state during creation', async () => {
    const user = userEvent.setup()

    // Make the account creation POST hang so loading state persists
    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes('/auth/account')) {
        return new Promise(() => {}) // Never resolves
      }
      // Initial /auth/me check returns 401
      return Promise.resolve({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ error: 'Unauthorized' }),
      })
    })

    render(
      <AuthProvider>
        <CreateAccountPage />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /create account/i })).toBeInTheDocument()
    })

    const submitButton = screen.getByRole('button', { name: /create account/i })
    await user.click(submitButton)

    // AuthProvider sets isLoading=true when createAccount is called,
    // which triggers the page loading guard
    await waitFor(() => {
      expect(screen.getByText('Loading...')).toBeInTheDocument()
    })
  })

  it('displays account number and wallet on success', async () => {
    const user = userEvent.setup()

    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes('/auth/account')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              account_number: '9999-8888-7777-6666',
              recovery_file: 'test-recovery',
              wallet_address: '0xabc123',
            }),
        })
      }
      // All /auth/me calls return 401 so isAuthenticated stays false
      // and the success screen is visible
      if (url.includes('/auth/me')) {
        return Promise.resolve({
          ok: false,
          status: 401,
          json: () => Promise.resolve({ error: 'Unauthorized' }),
        })
      }
      return Promise.resolve({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ error: 'Unauthorized' }),
      })
    })

    render(
      <AuthProvider>
        <CreateAccountPage />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /create account/i })).toBeInTheDocument()
    })

    const submitButton = screen.getByRole('button', { name: /create account/i })
    await user.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Account Created!')).toBeInTheDocument()
    })

    expect(screen.getByText('9999-8888-7777-6666')).toBeInTheDocument()
    expect(screen.getByText('0xabc123')).toBeInTheDocument()
  })

  it('recovery file download triggers downloadTextFile', async () => {
    const user = userEvent.setup()

    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes('/auth/account')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              account_number: '9999-8888-7777-6666',
              recovery_file: 'test-recovery-content',
              wallet_address: '0xabc123',
            }),
        })
      }
      // Keep isAuthenticated false so success screen stays visible
      return Promise.resolve({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ error: 'Unauthorized' }),
      })
    })

    render(
      <AuthProvider>
        <CreateAccountPage />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /create account/i })).toBeInTheDocument()
    })

    const submitButton = screen.getByRole('button', { name: /create account/i })
    await user.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Account Created!')).toBeInTheDocument()
    })

    const downloadButton = screen.getByText('Download Recovery File')
    await user.click(downloadButton)

    expect(downloadTextFile).toHaveBeenCalledWith(
      'stronghold-recovery-9999888877776666.txt',
      'test-recovery-content'
    )
  })

  it('continue button navigates to /dashboard/main', async () => {
    const user = userEvent.setup()

    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes('/auth/account')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              account_number: '9999-8888-7777-6666',
              recovery_file: 'test-recovery',
              wallet_address: '0xabc123',
            }),
        })
      }
      // Keep isAuthenticated false so success screen stays visible
      return Promise.resolve({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ error: 'Unauthorized' }),
      })
    })

    render(
      <AuthProvider>
        <CreateAccountPage />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /create account/i })).toBeInTheDocument()
    })

    const submitButton = screen.getByRole('button', { name: /create account/i })
    await user.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Account Created!')).toBeInTheDocument()
    })

    const continueButton = screen.getByText('Continue to Dashboard')
    await user.click(continueButton)

    expect(mockPush).toHaveBeenCalledWith('/dashboard/main')
  })

  it('error state shows error message', async () => {
    const user = userEvent.setup()

    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes('/auth/account')) {
        return Promise.resolve({
          ok: false,
          status: 500,
          json: () => Promise.resolve({ error: 'Server error' }),
        })
      }
      return Promise.resolve({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ error: 'Unauthorized' }),
      })
    })

    render(
      <AuthProvider>
        <CreateAccountPage />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /create account/i })).toBeInTheDocument()
    })

    const submitButton = screen.getByRole('button', { name: /create account/i })
    await user.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Server error')).toBeInTheDocument()
    })
  })

  it('redirects authenticated users to dashboard', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(mockAccount),
    })

    render(
      <AuthProvider>
        <CreateAccountPage />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith('/dashboard/main')
    })
  })

  it('has link to login page', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ error: 'Unauthorized' }),
    })

    render(
      <AuthProvider>
        <CreateAccountPage />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByText(/login/i)).toBeInTheDocument()
    })

    const loginLink = screen.getByRole('link', { name: /login/i })
    expect(loginLink).toHaveAttribute('href', '/dashboard/login')
  })
})
