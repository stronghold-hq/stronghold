import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import {
  Skeleton,
  SkeletonText,
  SkeletonCard,
  SkeletonTableRow,
  SkeletonStatCard,
} from '@/components/ui/Skeleton'

describe('Skeleton', () => {
  it('renders with default styles', () => {
    const { container } = render(<Skeleton />)
    const skeleton = container.firstChild as HTMLElement
    expect(skeleton).toHaveClass('animate-pulse', 'rounded-md', 'bg-[#1a1a1c]')
  })

  it('accepts custom className', () => {
    const { container } = render(<Skeleton className="w-full h-10" />)
    const skeleton = container.firstChild as HTMLElement
    expect(skeleton).toHaveClass('w-full', 'h-10')
  })
})

describe('SkeletonText', () => {
  it('renders with text-specific styles', () => {
    const { container } = render(<SkeletonText />)
    const skeleton = container.firstChild as HTMLElement
    expect(skeleton).toHaveClass('h-4', 'w-full')
  })

  it('accepts custom className', () => {
    const { container } = render(<SkeletonText className="w-32" />)
    const skeleton = container.firstChild as HTMLElement
    expect(skeleton).toHaveClass('w-32')
  })
})

describe('SkeletonCard', () => {
  it('renders card container with proper styles', () => {
    const { container } = render(<SkeletonCard />)
    const card = container.firstChild as HTMLElement
    expect(card).toHaveClass('bg-black/50', 'border', 'rounded-xl', 'p-6')
  })

  it('contains skeleton elements inside', () => {
    const { container } = render(<SkeletonCard />)
    const skeletons = container.querySelectorAll('.animate-pulse')
    expect(skeletons.length).toBeGreaterThan(0)
  })
})

describe('SkeletonTableRow', () => {
  it('renders default 5 columns', () => {
    const { container } = render(
      <table>
        <tbody>
          <SkeletonTableRow />
        </tbody>
      </table>
    )
    const cells = container.querySelectorAll('td')
    expect(cells).toHaveLength(5)
  })

  it('renders custom number of columns', () => {
    const { container } = render(
      <table>
        <tbody>
          <SkeletonTableRow columns={3} />
        </tbody>
      </table>
    )
    const cells = container.querySelectorAll('td')
    expect(cells).toHaveLength(3)
  })

  it('each cell contains a skeleton element', () => {
    const { container } = render(
      <table>
        <tbody>
          <SkeletonTableRow columns={4} />
        </tbody>
      </table>
    )
    const skeletons = container.querySelectorAll('.animate-pulse')
    expect(skeletons).toHaveLength(4)
  })
})

describe('SkeletonStatCard', () => {
  it('renders stat card container', () => {
    const { container } = render(<SkeletonStatCard />)
    const card = container.firstChild as HTMLElement
    expect(card).toHaveClass('bg-[#111]', 'border', 'rounded-xl', 'p-5')
  })

  it('contains multiple skeleton elements', () => {
    const { container } = render(<SkeletonStatCard />)
    const skeletons = container.querySelectorAll('.animate-pulse')
    expect(skeletons.length).toBeGreaterThanOrEqual(3)
  })
})
