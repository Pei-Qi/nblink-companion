import { describe, expect, it } from 'vitest'
import { createMockSnapshot } from '../src/mock'
import { resolveTheme, serviceMatches } from '../src/lib/presentation'

describe('service presentation', () => {
  const services = createMockSnapshot('default').services

  it('filters by search, favorite, running and error state', () => {
    expect(services.filter((item) => serviceMatches(item, '5432', 'all'))).toHaveLength(1)
    expect(services.filter((item) => serviceMatches(item, '', 'favorite')).every((item) => item.favorite)).toBe(true)
    expect(services.filter((item) => serviceMatches(item, '', 'running')).every((item) => item.running)).toBe(true)
    expect(services.filter((item) => serviceMatches(item, '', 'error')).every((item) => item.state === 'error' || item.state === 'waiting')).toBe(true)
  })

  it('resolves system and explicit themes', () => {
    expect(resolveTheme('system', true)).toBe('dark')
    expect(resolveTheme('system', false)).toBe('light')
    expect(resolveTheme('light', true)).toBe('light')
    expect(resolveTheme('dark', false)).toBe('dark')
  })
})
