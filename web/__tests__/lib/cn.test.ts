import { describe, it, expect } from 'vitest'
import { cn } from '@/lib/cn'

describe('cn', () => {
  it('merges class names', () => {
    expect(cn('foo', 'bar')).toBe('foo bar')
  })

  it('handles conditional classes', () => {
    expect(cn('base', true && 'included', false && 'excluded')).toBe('base included')
  })

  it('handles undefined and null', () => {
    expect(cn('base', undefined, null, 'end')).toBe('base end')
  })

  it('handles arrays', () => {
    expect(cn(['foo', 'bar'])).toBe('foo bar')
  })

  it('handles objects', () => {
    expect(cn({ foo: true, bar: false, baz: true })).toBe('foo baz')
  })

  it('merges conflicting Tailwind classes', () => {
    // tailwind-merge should keep the last conflicting class
    expect(cn('px-2', 'px-4')).toBe('px-4')
    expect(cn('text-red-500', 'text-blue-500')).toBe('text-blue-500')
  })

  it('preserves non-conflicting classes', () => {
    expect(cn('px-2 py-4', 'mt-2')).toBe('px-2 py-4 mt-2')
  })

  it('handles empty inputs', () => {
    expect(cn()).toBe('')
    expect(cn('')).toBe('')
  })

  it('handles complex combinations', () => {
    const result = cn(
      'base-class',
      true && 'conditional',
      { 'object-true': true, 'object-false': false },
      ['array-item']
    )
    expect(result).toBe('base-class conditional object-true array-item')
  })
})
