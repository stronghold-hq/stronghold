import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { LoadingSpinner, LoadingOverlay } from '@/components/ui/LoadingSpinner'

describe('LoadingSpinner', () => {
  it('renders with default medium size', () => {
    const { container } = render(<LoadingSpinner />)
    const spinner = container.firstChild as HTMLElement
    expect(spinner).toHaveClass('w-6', 'h-6')
  })

  it('renders with small size', () => {
    const { container } = render(<LoadingSpinner size="sm" />)
    const spinner = container.firstChild as HTMLElement
    expect(spinner).toHaveClass('w-4', 'h-4')
  })

  it('renders with large size', () => {
    const { container } = render(<LoadingSpinner size="lg" />)
    const spinner = container.firstChild as HTMLElement
    expect(spinner).toHaveClass('w-8', 'h-8')
  })

  it('has animation class', () => {
    const { container } = render(<LoadingSpinner />)
    const spinner = container.firstChild as HTMLElement
    expect(spinner).toHaveClass('animate-spin')
  })

  it('has proper border colors for spinner effect', () => {
    const { container } = render(<LoadingSpinner />)
    const spinner = container.firstChild as HTMLElement
    expect(spinner).toHaveClass('border-[#00D4AA]', 'border-t-transparent')
  })

  it('has accessibility attributes', () => {
    render(<LoadingSpinner />)
    expect(screen.getByRole('status')).toBeInTheDocument()
    expect(screen.getByText('Loading...')).toHaveClass('sr-only')
  })

  it('accepts custom className', () => {
    const { container } = render(<LoadingSpinner className="custom-class" />)
    const spinner = container.firstChild as HTMLElement
    expect(spinner).toHaveClass('custom-class')
  })
})

describe('LoadingOverlay', () => {
  it('renders fullscreen overlay', () => {
    const { container } = render(<LoadingOverlay />)
    const overlay = container.firstChild as HTMLElement
    expect(overlay).toHaveClass('min-h-screen', 'bg-[#0a0a0a]')
  })

  it('contains a spinner', () => {
    render(<LoadingOverlay />)
    expect(screen.getByRole('status')).toBeInTheDocument()
  })

  it('displays optional message', () => {
    render(<LoadingOverlay message="Please wait..." />)
    expect(screen.getByText('Please wait...')).toBeInTheDocument()
  })

  it('does not display message when not provided', () => {
    const { container } = render(<LoadingOverlay />)
    const paragraphs = container.querySelectorAll('p')
    expect(paragraphs).toHaveLength(0)
  })
})
